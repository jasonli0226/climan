package reconcile

import (
	"reflect"
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/registry"
)

var (
	t0 = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	t1 = t0.Add(24 * time.Hour)
)

func tool(key string, tag discover.Tag, members ...string) discover.Tool {
	if len(members) == 0 {
		members = []string{key}
	}
	t := discover.Tool{Key: key, Tag: tag}
	for _, m := range members {
		t.Members = append(t.Members, discover.Found{Name: m, Path: "/h/bin/" + m, Dir: "/h/bin", Tag: tag})
	}
	return t
}

func TestNewInstalledToolIsPending(t *testing.T) {
	reg := registry.New()
	tl := tool("rg", discover.TagInstalled)
	tl.Members[0].Shim = "scoop"
	tl.Members[0].Shadowed = []string{"/usr/bin/rg"}

	res := Reconcile(reg, []discover.Tool{tl}, t0)

	want := &registry.Entry{
		Status: registry.Pending, Source: registry.Discovered, Tag: "installed",
		Path: "/h/bin/rg", Members: []string{"rg"}, Shim: "scoop",
		Shadowed: []string{"/usr/bin/rg"}, LastSeen: t0,
	}
	if !reflect.DeepEqual(reg.Tools["rg"], want) {
		t.Fatalf("entry = %+v\nwant    %+v", reg.Tools["rg"], want)
	}
	if !reflect.DeepEqual(res.New, []string{"rg"}) {
		t.Fatalf("New = %v, want [rg]", res.New)
	}
	if !reg.LastSync.Equal(t0) {
		t.Fatalf("LastSync = %v, want %v", reg.LastSync, t0)
	}
}

func TestCoreToolNotWritten(t *testing.T) {
	reg := registry.New()
	res := Reconcile(reg, []discover.Tool{tool("ls", discover.TagCore)}, t0)
	if len(reg.Tools) != 0 || len(res.New) != 0 {
		t.Fatalf("core tool written: tools=%v new=%v", reg.Tools, res.New)
	}
}

func TestCoreToolAlreadyEnabledIsUpdated(t *testing.T) {
	reg := registry.New()
	reg.Tools["git"] = &registry.Entry{Status: registry.Enabled, Source: registry.Discovered, LastSeen: t0}
	Reconcile(reg, []discover.Tool{tool("git", discover.TagCore)}, t1)
	if e := reg.Tools["git"]; !e.LastSeen.Equal(t1) || e.Status != registry.Enabled {
		t.Fatalf("git = %+v, want enabled with LastSeen %v", e, t1)
	}
}

func TestNoiseIsIgnored(t *testing.T) {
	reg := registry.New()
	tl := tool("x.sh", discover.TagInstalled)
	tl.Noise = true
	res := Reconcile(reg, []discover.Tool{tl}, t0)
	if reg.Tools["x.sh"].Status != registry.Ignored {
		t.Fatalf("status = %q, want ignored", reg.Tools["x.sh"].Status)
	}
	if !reflect.DeepEqual(res.Noise, []string{"x.sh"}) || len(res.New) != 0 {
		t.Fatalf("Noise = %v New = %v, want [x.sh] []", res.Noise, res.New)
	}
}

func TestStatusKept(t *testing.T) {
	reg := registry.New()
	reg.Tools["rg"] = &registry.Entry{
		Status: registry.Enabled, Source: registry.Discovered, Path: "/old/rg",
		Notes: "mine", Version: "14.0", Summary: "search", Tags: []string{"search"},
	}
	Reconcile(reg, []discover.Tool{tool("rg", discover.TagInstalled)}, t0)
	e := reg.Tools["rg"]
	if e.Status != registry.Enabled || e.Notes != "mine" || e.Path != "/h/bin/rg" {
		t.Fatalf("entry = %+v, want enabled, notes kept, path updated", e)
	}
	if e.Version != "14.0" || e.Summary != "search" || !reflect.DeepEqual(e.Tags, []string{"search"}) {
		t.Fatalf("curated/probed fields changed: %+v", e)
	}
}

func TestStaleAndReturn(t *testing.T) {
	reg := registry.New()
	Reconcile(reg, []discover.Tool{tool("rg", discover.TagInstalled)}, t0)
	reg.Tools["rg"].Status = registry.Enabled

	res := Reconcile(reg, nil, t1)
	if !reflect.DeepEqual(res.Stale, []string{"rg"}) || !reg.Stale("rg") {
		t.Fatalf("Stale = %v, reg.Stale = %v; want rg stale", res.Stale, reg.Stale("rg"))
	}
	if !reg.Tools["rg"].LastSeen.Equal(t0) {
		t.Fatalf("LastSeen = %v, want unchanged %v", reg.Tools["rg"].LastSeen, t0)
	}

	t2 := t1.Add(time.Hour)
	res = Reconcile(reg, []discover.Tool{tool("rg", discover.TagInstalled)}, t2)
	if reg.Stale("rg") || len(res.Stale) != 0 || reg.Tools["rg"].Status != registry.Enabled || len(res.New) != 0 {
		t.Fatalf("after return: stale=%v status=%q new=%v", reg.Stale("rg"), reg.Tools["rg"].Status, res.New)
	}
}

func TestRegisteredUntouched(t *testing.T) {
	reg := registry.New()
	orig := registry.Entry{Status: registry.Enabled, Source: registry.Registered, Path: "/my/rg", Notes: "manual"}
	e := orig
	reg.Tools["rg"] = &e
	res := Reconcile(reg, []discover.Tool{tool("rg", discover.TagInstalled)}, t0)
	if !reflect.DeepEqual(*reg.Tools["rg"], orig) {
		t.Fatalf("registered entry changed: %+v", *reg.Tools["rg"])
	}
	if len(res.New) != 0 || len(res.Stale) != 0 {
		t.Fatalf("New = %v Stale = %v, want none", res.New, res.Stale)
	}
}

func TestFamilyGainsMember(t *testing.T) {
	reg := registry.New()
	reg.Tools["python"] = &registry.Entry{
		Status: registry.Enabled, Source: registry.Discovered,
		Members: []string{"python", "python3"}, Notes: "use uv",
	}
	tl := tool("python", discover.TagInstalled, "python", "python3", "python3.13")
	res := Reconcile(reg, []discover.Tool{tl}, t0)
	e := reg.Tools["python"]
	if !reflect.DeepEqual(e.Members, []string{"python", "python3", "python3.13"}) {
		t.Fatalf("Members = %v", e.Members)
	}
	if e.Notes != "use uv" || e.Status != registry.Enabled || len(res.New) != 0 {
		t.Fatalf("entry = %+v new = %v, want notes and status kept", e, res.New)
	}
}

func TestFamilyPathPrefersKeyMember(t *testing.T) {
	reg := registry.New()
	Reconcile(reg, []discover.Tool{tool("pip", discover.TagInstalled, "pip3", "pip3.12")}, t0)
	if got := reg.Tools["pip"].Path; got != "/h/bin/pip3" {
		t.Fatalf("Path = %q, want first member when no member equals the key", got)
	}
	Reconcile(reg, []discover.Tool{tool("python", discover.TagInstalled, "python", "python3")}, t0)
	if got := reg.Tools["python"].Path; got != "/h/bin/python" {
		t.Fatalf("Path = %q, want the member named like the key", got)
	}
}

func TestNeverDeletes(t *testing.T) {
	reg := registry.New()
	Reconcile(reg, []discover.Tool{tool("a", discover.TagInstalled), tool("b", discover.TagInstalled)}, t0)
	Reconcile(reg, nil, t1)
	if len(reg.Tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(reg.Tools))
	}
}
