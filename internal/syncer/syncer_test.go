package syncer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/probe"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/rules"
)

// fixture is a fake Windows machine (usable on any host): SystemRoot/System32
// is core, bin is installed. Tool files are empty; probes are faked.
type fixture struct {
	t      *testing.T
	root   string
	bin    string
	core   string
	store  registry.Store
	mu     sync.Mutex
	probed []string
	delay  time.Duration
	out    bytes.Buffer
}

func newFixture(t *testing.T, installed ...string) *fixture {
	root := t.TempDir()
	f := &fixture{
		t: t, root: root,
		bin:   filepath.Join(root, "bin"),
		core:  filepath.Join(root, "Windows", "System32"),
		store: registry.Store{Dir: filepath.Join(root, "config")},
	}
	for _, d := range []string{f.bin, f.core} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range installed {
		f.add(f.bin, name)
	}
	return f
}

func (f *fixture) add(dir, name string) {
	if err := os.WriteFile(filepath.Join(dir, name+".exe"), nil, 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) env() platform.Env {
	return platform.Env{
		GOOS: "windows", Path: f.core + ";" + f.bin, PathExt: []string{".exe"},
		Vars: map[string]string{"SystemRoot": filepath.Join(f.root, "Windows")},
	}
}

func (f *fixture) run(c Chooser) error {
	f.out.Reset()
	return Run(context.Background(), Options{
		Env: f.env(), Store: f.store, Chooser: c,
		Now: func() time.Time { return time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC) },
		Probe: func(_ context.Context, _ platform.Env, path string) probe.Info {
			time.Sleep(f.delay)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.probed = append(f.probed, filepath.Base(path))
			return probe.Info{Version: "v-" + filepath.Base(path), Summary: "does things"}
		},
		Out: &f.out,
	})
}

func (f *fixture) entry(key string) *registry.Entry {
	reg, err := f.store.Load()
	if err != nil {
		f.t.Fatal(err)
	}
	return reg.Tools[key]
}

// fakeChooser returns fixed decisions and records whether it was called.
type fakeChooser struct {
	decisions map[string]Decision
	called    bool
	seen      []string
	during    func()
}

func (c *fakeChooser) Choose(cands []Candidate) (map[string]Decision, error) {
	c.called = true
	for _, cand := range cands {
		c.seen = append(c.seen, cand.Key)
	}
	if c.during != nil {
		c.during()
	}
	return c.decisions, nil
}

func TestRunInteractiveEnablesChecked(t *testing.T) {
	f := newFixture(t, "a", "b")
	c := &fakeChooser{decisions: map[string]Decision{
		"a": {Status: registry.Enabled}, "b": {Status: registry.Ignored},
	}}
	if err := f.run(c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.seen, []string{"a", "b"}) {
		t.Fatalf("chooser saw %v, want [a b]", c.seen)
	}
	a, b := f.entry("a"), f.entry("b")
	if a.Status != registry.Enabled || b.Status != registry.Ignored {
		t.Fatalf("a=%q b=%q, want enabled ignored", a.Status, b.Status)
	}
	if !reflect.DeepEqual(f.probed, []string{"a.exe"}) {
		t.Fatalf("probed %v, want [a.exe]", f.probed)
	}
	if a.Version != "v-a.exe" || a.Summary != "does things" {
		t.Fatalf("probe info not stored: %+v", a)
	}
}

func TestRunBatchLeavesPending(t *testing.T) {
	f := newFixture(t, "a", "b")
	if err := f.run(BatchChooser{}); err != nil {
		t.Fatal(err)
	}
	if f.entry("a").Status != registry.Pending || f.entry("b").Status != registry.Pending {
		t.Fatal("batch sync changed statuses")
	}
	if len(f.probed) != 0 {
		t.Fatalf("probed %v, want none", f.probed)
	}
}

func TestRunAuto(t *testing.T) {
	f := newFixture(t, "gopls", "rg")
	r := rules.Rules{Rules: []rules.Rule{{Name: "go*", Action: rules.ActionEnable, Tags: []string{"go"}}}}
	if err := f.run(AutoChooser{Rules: r}); err != nil {
		t.Fatal(err)
	}
	g := f.entry("gopls")
	if g.Status != registry.Enabled || !reflect.DeepEqual(g.Tags, []string{"go"}) {
		t.Fatalf("gopls = %+v, want enabled with tags [go]", g)
	}
	if f.entry("rg").Status != registry.Pending {
		t.Fatalf("rg = %q, want pending", f.entry("rg").Status)
	}
}

func TestRunMissingDecisionIsPending(t *testing.T) {
	f := newFixture(t, "a", "b")
	if err := f.run(&fakeChooser{decisions: map[string]Decision{}}); err != nil {
		t.Fatal(err)
	}
	if f.entry("a").Status != registry.Pending || f.entry("b").Status != registry.Pending {
		t.Fatal("missing decisions should leave tools pending")
	}
}

func TestRunNoCandidatesSkipsChooser(t *testing.T) {
	f := newFixture(t, "a")
	if err := f.run(BatchChooser{}); err != nil {
		t.Fatal(err)
	}
	c := &fakeChooser{}
	if err := f.run(c); err != nil {
		t.Fatal(err)
	}
	if c.called {
		t.Fatal("chooser called with no new tools")
	}
}

func TestRunCoreNeverProbed(t *testing.T) {
	f := newFixture(t, "a")
	f.add(f.core, "cmd")
	if err := f.store.Update(func(r *registry.Registry) error {
		r.Tools["cmd"] = &registry.Entry{Status: registry.Enabled, Source: registry.Discovered}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	c := &fakeChooser{decisions: map[string]Decision{"a": {Status: registry.Enabled}}}
	if err := f.run(c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.probed, []string{"a.exe"}) {
		t.Fatalf("probed %v, want only [a.exe]", f.probed)
	}
	if !reflect.DeepEqual(c.seen, []string{"a"}) {
		t.Fatalf("chooser saw %v, want [a] (core tools are not offered)", c.seen)
	}
}

func TestRunRereadsBeforeWrite(t *testing.T) {
	f := newFixture(t, "a", "b")
	if err := f.run(BatchChooser{}); err != nil {
		t.Fatal(err)
	}
	f.add(f.bin, "c")
	c := &fakeChooser{decisions: map[string]Decision{"c": {Status: registry.Enabled}}}
	c.during = func() {
		err := f.store.Update(func(r *registry.Registry) error {
			r.Tools["a"].Notes = "edited during prompt"
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := f.run(c); err != nil {
		t.Fatal(err)
	}
	if f.entry("a").Notes != "edited during prompt" || f.entry("c").Status != registry.Enabled {
		t.Fatalf("a=%+v c=%+v, want edit kept and c enabled", f.entry("a"), f.entry("c"))
	}
}

func TestRunSummary(t *testing.T) {
	f := newFixture(t, "a", "b")
	c := &fakeChooser{decisions: map[string]Decision{
		"a": {Status: registry.Enabled}, "b": {Status: registry.Ignored},
	}}
	if err := f.run(c); err != nil {
		t.Fatal(err)
	}
	if want := "new: 2, enabled: 1, ignored: 1, pending: 0, stale: 0"; !strings.Contains(f.out.String(), want) {
		t.Fatalf("output %q lacks %q", f.out.String(), want)
	}
}

func TestRunReportsProbing(t *testing.T) {
	f := newFixture(t, "a")
	if err := f.run(&fakeChooser{decisions: map[string]Decision{"a": {Status: registry.Enabled}}}); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	p, s := strings.Index(out, "probing 1 new tool"), strings.Index(out, "sync:")
	if p < 0 || p > s {
		t.Fatalf("output %q: want a probing line before the summary", out)
	}
}

func TestRunProbesInParallel(t *testing.T) {
	keys := []string{"a", "b", "c", "d"}
	f := newFixture(t, keys...)
	f.delay = 300 * time.Millisecond
	d := map[string]Decision{}
	for _, k := range keys {
		d[k] = Decision{Status: registry.Enabled}
	}
	start := time.Now()
	if err := f.run(&fakeChooser{decisions: d}); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 900*time.Millisecond {
		t.Fatalf("4 probes of 300ms took %v, want them run in parallel", took)
	}
	if len(f.probed) != 4 {
		t.Fatalf("probed %v, want 4", f.probed)
	}
}
