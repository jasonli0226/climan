package status

import (
	"bytes"
	"context"
	"errors"
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
	"github.com/jasonli0226/climan/internal/syncer"
)

// fixture is a fake Windows machine usable on any host: SystemRoot/System32
// is core, bin is installed. Probes are faked.
type fixture struct {
	t         *testing.T
	root, bin string
	core      string
	store     registry.Store
	now       time.Time
	mu        sync.Mutex
	probed    []string
	out       bytes.Buffer
}

func newFixture(t *testing.T, installed ...string) *fixture {
	root := t.TempDir()
	f := &fixture{
		t: t, root: root, bin: filepath.Join(root, "bin"),
		core:  filepath.Join(root, "Windows", "System32"),
		store: registry.Store{Dir: filepath.Join(root, "config")},
		now:   time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
	}
	for _, d := range []string{f.bin, f.core} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range installed {
		f.add(f.bin, n)
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

func (f *fixture) probe(_ context.Context, _ platform.Env, path string) probe.Info {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probed = append(f.probed, filepath.Base(path))
	return probe.Info{Version: "v1", Summary: "does things"}
}

// sync runs a batch sync, so every installed tool is pending.
func (f *fixture) sync() {
	f.now = f.now.Add(time.Hour)
	err := syncer.Run(context.Background(), syncer.Options{
		Env: f.env(), Store: f.store, Chooser: syncer.BatchChooser{},
		Now: func() time.Time { return f.now }, Probe: f.probe, Out: &bytes.Buffer{},
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) set(names []string, to registry.Status, desc *string) error {
	f.out.Reset()
	f.probed = nil
	return Set(context.Background(), Options{
		Env: f.env(), Store: f.store, Now: func() time.Time { return f.now },
		Probe: f.probe, Out: &f.out,
	}, names, to, desc)
}

func (f *fixture) entry(key string) *registry.Entry {
	reg, err := f.store.Load()
	if err != nil {
		f.t.Fatal(err)
	}
	return reg.Tools[key]
}

func (f *fixture) lines() []string {
	return strings.Split(strings.TrimSpace(f.out.String()), "\n")
}

func ptr(s string) *string { return &s }

func TestSetEnablesAndProbes(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	if err := f.set([]string{"rg"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if e := f.entry("rg"); e.Status != registry.Enabled || e.Version != "v1" {
		t.Fatalf("rg = %+v, want enabled with probed version", e)
	}
	if !reflect.DeepEqual(f.probed, []string{"rg.exe"}) {
		t.Fatalf("probed %v", f.probed)
	}
	if !strings.Contains(f.out.String(), "enabled: rg\n") {
		t.Fatalf("output %q", f.out.String())
	}
}

func TestSetUnchanged(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	if err := f.set([]string{"rg"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.set([]string{"rg"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if f.out.String() != "unchanged: rg\n" || len(f.probed) != 0 {
		t.Fatalf("output %q probed %v", f.out.String(), f.probed)
	}
}

func TestSetDescOnEnabled(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	if err := f.set([]string{"rg"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.set([]string{"rg"}, registry.Enabled, ptr(" a\tb ")); err != nil {
		t.Fatal(err)
	}
	if got := f.lines(); !reflect.DeepEqual(got, []string{"unchanged: rg", "desc set: rg"}) {
		t.Fatalf("lines %q", got)
	}
	if d := f.entry("rg").Desc; d != "a b" {
		t.Fatalf("desc = %q, want cleaned \"a b\"", d)
	}
	if err := f.set([]string{"rg"}, registry.Enabled, ptr("")); err != nil {
		t.Fatal(err)
	}
	if f.entry("rg").Desc != "" || !strings.Contains(f.out.String(), "desc cleared: rg") {
		t.Fatalf("desc %q output %q", f.entry("rg").Desc, f.out.String())
	}
}

func TestSetDescNewline(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	err := f.set([]string{"rg"}, registry.Enabled, ptr("a\nb"))
	if err == nil || !strings.Contains(err.Error(), "single line") {
		t.Fatalf("err = %v, want single line error", err)
	}
	if f.entry("rg").Status != registry.Pending {
		t.Fatal("registry changed despite error")
	}
}

func TestSetMemberName(t *testing.T) {
	f := newFixture(t, "python", "python3")
	f.sync()
	if err := f.set([]string{"python3"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if f.entry("python").Status != registry.Enabled || !strings.Contains(f.out.String(), "enabled: python\n") {
		t.Fatalf("python = %+v output %q", f.entry("python"), f.out.String())
	}
}

func TestSetDuplicateNames(t *testing.T) {
	f := newFixture(t, "python", "python3")
	f.sync()
	if err := f.set([]string{"python", "python3"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(f.out.String(), "enabled: python"); n != 1 {
		t.Fatalf("output %q has %d enabled lines, want 1", f.out.String(), n)
	}
}

func TestSetUnknownAllOrNothing(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	err := f.set([]string{"rg", "nope", "zzz"}, registry.Enabled, nil)
	if !errors.Is(err, ErrUnknown) || !strings.Contains(err.Error(), "unknown tool: nope, zzz") {
		t.Fatalf("err = %v", err)
	}
	if f.entry("rg").Status != registry.Pending {
		t.Fatal("rg changed despite unknown names")
	}
}

func TestSetAdoptsCoreFromScan(t *testing.T) {
	f := newFixture(t, "rg")
	f.add(f.core, "cmd")
	f.sync()
	if f.entry("cmd") != nil {
		t.Fatal("sync wrote a core tool")
	}
	if err := f.set([]string{"cmd"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	e := f.entry("cmd")
	if e == nil || e.Status != registry.Enabled || e.Tag != "core" || len(f.probed) != 0 {
		t.Fatalf("cmd = %+v probed %v, want enabled core, no probe", e, f.probed)
	}
}

func TestSetAdoptsNewInstalled(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	f.add(f.bin, "fd")
	if err := f.set([]string{"fd"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if f.entry("fd").Status != registry.Enabled || !reflect.DeepEqual(f.probed, []string{"fd.exe"}) {
		t.Fatalf("fd = %+v probed %v", f.entry("fd"), f.probed)
	}
}

func TestSetStale(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	if err := os.Remove(filepath.Join(f.bin, "rg.exe")); err != nil {
		t.Fatal(err)
	}
	f.sync()
	if err := f.set([]string{"rg"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	want := "enabled: rg (not installed now; hidden from context until it is back)\n"
	if f.entry("rg").Status != registry.Enabled || f.out.String() != want || len(f.probed) != 0 {
		t.Fatalf("status %q output %q probed %v", f.entry("rg").Status, f.out.String(), f.probed)
	}
}

func TestSetIgnoreNeverProbes(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	if err := f.set([]string{"rg"}, registry.Ignored, nil); err != nil {
		t.Fatal(err)
	}
	if f.entry("rg").Status != registry.Ignored || f.out.String() != "ignored: rg\n" || len(f.probed) != 0 {
		t.Fatalf("status %q output %q probed %v", f.entry("rg").Status, f.out.String(), f.probed)
	}
}

func TestSetRegistered(t *testing.T) {
	f := newFixture(t)
	orig := registry.Entry{Status: registry.Pending, Source: registry.Registered, Notes: "mine"}
	if err := f.store.Update(func(r *registry.Registry) error {
		e := orig
		r.Tools["alias://gs"] = &e
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.set([]string{"alias://gs"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	want := orig
	want.Status = registry.Enabled
	if got := f.entry("alias://gs"); !reflect.DeepEqual(*got, want) {
		t.Fatalf("entry = %+v, want %+v", *got, want)
	}
}

func TestSetNoRegistryFile(t *testing.T) {
	f := newFixture(t, "rg")
	if err := f.set([]string{"rg"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if e := f.entry("rg"); e == nil || e.Status != registry.Enabled {
		t.Fatalf("rg = %+v, want enabled in a new registry", e)
	}
}

func TestSetWindowsCase(t *testing.T) {
	f := newFixture(t, "rg")
	f.sync()
	if err := f.set([]string{"RG"}, registry.Enabled, nil); err != nil {
		t.Fatal(err)
	}
	if f.entry("rg").Status != registry.Enabled {
		t.Fatal("RG did not match rg on windows")
	}
}
