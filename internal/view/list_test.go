package view

import (
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
)

var linuxEnv = platform.Env{GOOS: "linux"}

func listReg() *registry.Registry {
	r := testReg()
	discovered(r, "rg", registry.Enabled, registry.Entry{Summary: "search"})
	discovered(r, "zoxide", registry.Enabled, registry.Entry{Summary: "zoxide 0.9.9", Version: "zoxide 0.9.9"})
	discovered(r, "old", registry.Enabled, registry.Entry{Summary: "gone"})
	r.Tools["old"].LastSeen = r.LastSync.Add(-time.Hour)
	discovered(r, "p", registry.Pending, registry.Entry{Summary: "pending tool"})
	discovered(r, "ig", registry.Ignored, registry.Entry{Summary: "ignored tool"})
	return r
}

func TestListEnabled(t *testing.T) {
	want := "old (stale)  gone\n" +
		"rg           search\n" +
		"zoxide       ? zoxide 0.9.9\n"
	if got := List(linuxEnv, listReg(), Enabled, nil); got != want {
		t.Fatalf("List =\n%s\nwant\n%s", got, want)
	}
}

func TestListModes(t *testing.T) {
	r := listReg()
	if got := List(linuxEnv, r, Pending, nil); got != "p  pending tool\n" {
		t.Errorf("Pending = %q", got)
	}
	if got := List(linuxEnv, r, Ignored, nil); got != "ig  ignored tool\n" {
		t.Errorf("Ignored = %q", got)
	}
	r.Tools["ig"].LastSeen = r.LastSync.Add(-time.Hour)
	want := "ig (stale)   ignored tool\nold (stale)  gone\n"
	if got := List(linuxEnv, r, Stale, nil); got != want {
		t.Errorf("Stale = %q, want %q", got, want)
	}
}

func TestListMissing(t *testing.T) {
	r := testReg()
	r.Tools["file://~/gone"] = &registry.Entry{Status: registry.Enabled, Source: registry.Registered, Desc: "backup"}
	env := platform.Env{GOOS: "linux", Home: t.TempDir()}
	if got := List(env, r, Enabled, nil); got != "file://~/gone (missing)  backup\n" {
		t.Fatalf("List = %q", got)
	}
}

func TestListAll(t *testing.T) {
	r := testReg()
	discovered(r, "rg", registry.Enabled, registry.Entry{Summary: "search"})
	discovered(r, "p", registry.Pending, registry.Entry{Summary: "pending tool"})
	discovered(r, "ig", registry.Ignored, registry.Entry{Summary: "ignored tool"})
	core := []discover.Tool{
		{Key: "git", Tag: discover.TagCore},
		{Key: "rg", Tag: discover.TagCore}, // already in the registry: not repeated
	}
	want := "git  core\n" +
		"ig   ignored  ignored tool\n" +
		"p    pending  pending tool\n" +
		"rg   enabled  search\n"
	if got := List(linuxEnv, r, All, core); got != want {
		t.Fatalf("List =\n%q\nwant\n%q", got, want)
	}
}

func TestListEmpty(t *testing.T) {
	if got := List(linuxEnv, registry.New(), Enabled, nil); got != "" {
		t.Fatalf("List = %q, want empty", got)
	}
}

func TestListDescWins(t *testing.T) {
	r := testReg()
	discovered(r, "zoxide", registry.Enabled, registry.Entry{Summary: "zoxide 0.9.9", Version: "zoxide 0.9.9", Desc: "jump to dirs"})
	if got := List(linuxEnv, r, Enabled, nil); got != "zoxide  jump to dirs\n" {
		t.Fatalf("List = %q", got)
	}
}
