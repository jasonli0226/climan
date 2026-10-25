package view

import (
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
)

const header = "Installed CLI tools on this machine (from climan). Prefer one of these over writing a script; run `<tool> --help` for usage.\n"

func testReg() *registry.Registry {
	r := registry.New()
	r.LastSync = time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	return r
}

func discovered(r *registry.Registry, key string, status registry.Status, e registry.Entry) {
	e.Status, e.Source, e.LastSeen = status, registry.Discovered, r.LastSync
	if e.Members == nil {
		e.Members = []string{key}
	}
	r.Tools[key] = &e
}

func TestContextFormat(t *testing.T) {
	r := testReg()
	discovered(r, "aws", registry.Enabled, registry.Entry{Summary: "AWS command line interface", Tags: []string{"cloud"}})
	discovered(r, "python", registry.Enabled, registry.Entry{
		Members: []string{"python", "python3", "python3.12"}, Desc: "Python interpreter"})
	discovered(r, "zoxide", registry.Enabled, registry.Entry{Summary: "zoxide 0.9.9", Version: "zoxide 0.9.9"})
	discovered(r, "rg", registry.Pending, registry.Entry{Summary: "search"})
	discovered(r, "x", registry.Ignored, registry.Entry{Summary: "x"})

	want := header +
		"aws: AWS command line interface [cloud]\n" +
		"python (also python3, python3.12): Python interpreter\n" +
		"zoxide\n"
	if got := Context(platform.Env{GOOS: "linux"}, r); got != want {
		t.Fatalf("Context =\n%s\nwant\n%s", got, want)
	}
}

func TestContextSkipsAbsent(t *testing.T) {
	r := testReg()
	discovered(r, "old", registry.Enabled, registry.Entry{Summary: "gone"})
	r.Tools["old"].LastSeen = r.LastSync.Add(-time.Hour)
	r.Tools["file://~/gone"] = &registry.Entry{Status: registry.Enabled, Source: registry.Registered, Desc: "backup"}
	r.Tools["alias://gs"] = &registry.Entry{Status: registry.Enabled, Source: registry.Registered, Desc: "git status"}

	want := header + "alias://gs: git status\n"
	if got := Context(platform.Env{GOOS: "linux", Home: t.TempDir()}, r); got != want {
		t.Fatalf("Context =\n%s\nwant\n%s", got, want)
	}
}

func TestContextEmpty(t *testing.T) {
	want := header + "No tools enabled yet. Tell the user to run `climan sync` (interactive; do not run it yourself).\n"
	if got := Context(platform.Env{}, registry.New()); got != want {
		t.Fatalf("Context = %q, want %q", got, want)
	}
}

func TestContextFamilyKeyNotRepeated(t *testing.T) {
	r := testReg()
	discovered(r, "pip", registry.Enabled, registry.Entry{Members: []string{"pip3", "pip3.12"}, Desc: "installer"})
	want := header + "pip (also pip3, pip3.12): installer\n"
	if got := Context(platform.Env{}, r); got != want {
		t.Fatalf("Context = %q, want %q", got, want)
	}
}
