package describe

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/probe"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/status"
)

var seen = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

// fixture: a fake Windows machine (any host) with a core dir, plus a store.
type fixture struct {
	root  string
	store registry.Store
	out   bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	root := t.TempDir()
	f := &fixture{root: root, store: registry.Store{Dir: filepath.Join(root, "config")}}
	core := filepath.Join(root, "Windows", "System32")
	if err := os.MkdirAll(core, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(core, "git.exe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) env() platform.Env {
	return platform.Env{
		GOOS: "windows", Path: filepath.Join(f.root, "Windows", "System32"), PathExt: []string{".exe"},
		Vars: map[string]string{"SystemRoot": filepath.Join(f.root, "Windows")},
	}
}

func (f *fixture) put(t *testing.T, lastSync time.Time, key string, e registry.Entry) {
	t.Helper()
	err := f.store.Update(func(r *registry.Registry) error {
		r.LastSync = lastSync
		r.Tools[key] = &e
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) run(t *testing.T, name string, p probe.Func) error {
	t.Helper()
	f.out.Reset()
	return Run(context.Background(), Options{Env: f.env(), Store: f.store, Probe: p, Out: &f.out}, name)
}

func TestDescribeEntry(t *testing.T) {
	f := newFixture(t)
	f.put(t, seen, "rg", registry.Entry{
		Status: registry.Enabled, Source: registry.Discovered, Tag: "installed",
		Path: "/h/bin/rg", Members: []string{"rg"}, Shadowed: []string{"/usr/bin/rg"},
		LastSeen: seen, Version: "rg 14", Summary: "search tool", Desc: "fast grep",
		Tags: []string{"search"}, Notes: "line1\nline2",
	})
	if err := f.run(t, "rg", nil); err != nil {
		t.Fatal(err)
	}
	want := `rg
  status:    enabled
  source:    discovered
  tag:       installed
  path:      /h/bin/rg
  shadowed:  /usr/bin/rg
  version:   rg 14
  summary:   search tool
  desc:      fast grep
  tags:      search
  notes:     line1
             line2
  last seen: 2026-10-25T09:00:00Z
  state:     present
`
	if got := f.out.String(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDescribeFamilyAndStale(t *testing.T) {
	f := newFixture(t)
	f.put(t, seen.Add(time.Hour), "python", registry.Entry{
		Status: registry.Enabled, Source: registry.Discovered,
		Members: []string{"python", "python3"}, LastSeen: seen,
	})
	if err := f.run(t, "python3", nil); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "  members:   python, python3\n") || !strings.Contains(out, "  state:     stale\n") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestDescribeScanOnly(t *testing.T) {
	f := newFixture(t)
	if err := f.run(t, "git", nil); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.HasPrefix(out, "git (not in registry)\n") || !strings.Contains(out, "  tag:       core\n") ||
		!strings.Contains(out, "  path:      ") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestDescribeUnknown(t *testing.T) {
	f := newFixture(t)
	if err := f.run(t, "nope", nil); !errors.Is(err, status.ErrUnknown) {
		t.Fatalf("err = %v", err)
	}
}

func TestDescribeProbe(t *testing.T) {
	f := newFixture(t)
	f.put(t, seen, "rg", registry.Entry{Status: registry.Enabled, Source: registry.Discovered, Path: "/h/bin/rg", LastSeen: seen})
	fake := func(context.Context, platform.Env, string) probe.Info {
		return probe.Info{Version: "rg 15", Summary: "new summary"}
	}
	if err := f.run(t, "rg", fake); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "probed version: rg 15\n") || !strings.Contains(out, "probed summary: new summary\n") {
		t.Fatalf("output:\n%s", out)
	}
	reg, _ := f.store.Load()
	if e := reg.Tools["rg"]; e.Version != "rg 15" || e.Summary != "new summary" {
		t.Fatalf("entry not updated: %+v", e)
	}

	if err := f.run(t, "git", fake); err != nil {
		t.Fatal(err)
	}
	reg, _ = f.store.Load()
	if reg.Tools["git"] != nil {
		t.Fatal("describe --probe wrote a scan-only tool")
	}
}
