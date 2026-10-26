package view

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
)

func TestDescriptionPrefersDesc(t *testing.T) {
	text, weak := Description(&registry.Entry{Desc: "jump to dirs", Summary: "x"})
	if text != "jump to dirs" || weak {
		t.Fatalf("Description = %q, %v; want jump to dirs, false", text, weak)
	}
}

func TestDescriptionWeakSummary(t *testing.T) {
	tests := []struct {
		e        registry.Entry
		wantText string
		wantWeak bool
	}{
		{registry.Entry{}, "", true},
		{registry.Entry{Summary: "zoxide 0.9.9", Version: "zoxide 0.9.9"}, "zoxide 0.9.9", true},
		{registry.Entry{Summary: "Usage: tool [x]"}, "Usage: tool [x]", true},
		{registry.Entry{Summary: "usage tool"}, "usage tool", true},
		{registry.Entry{Summary: "npm <command>"}, "npm <command>", true},
		{registry.Entry{Summary: "aws: [ERROR]: arguments are required"}, "aws: [ERROR]: arguments are required", true},
		{registry.Entry{Summary: "usage-stats tool"}, "usage-stats tool", false},
		{registry.Entry{Summary: "Search files"}, "Search files", false},
	}
	for _, tt := range tests {
		text, weak := Description(&tt.e)
		if text != tt.wantText || weak != tt.wantWeak {
			t.Errorf("Description(%+v) = %q, %v; want %q, %v", tt.e, text, weak, tt.wantText, tt.wantWeak)
		}
	}
}

func TestDescriptionOneLine(t *testing.T) {
	text, _ := Description(&registry.Entry{Desc: "line one\n  line two\t end\n"})
	if text != "line one line two end" {
		t.Fatalf("Description = %q, want one line", text)
	}
}

func TestMissing(t *testing.T) {
	home := t.TempDir()
	env := platform.Env{GOOS: "linux", Home: home}
	present := filepath.Join(home, "backup")
	if err := os.WriteFile(present, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := func(path string) *registry.Entry {
		return &registry.Entry{Source: registry.Registered, Path: path}
	}
	tests := []struct {
		name string
		key  string
		e    *registry.Entry
		want bool
	}{
		{"existing path", "file://~/backup", reg(present), false},
		{"deleted path", "file://~/backup", reg(filepath.Join(home, "gone")), true},
		{"key only, missing", "file://~/gone", reg(""), true},
		{"key only, present", "file://~/backup", reg(""), false},
		{"alias", "alias://gs", reg(""), false},
		{"mcp", "mcp://github", reg(""), false},
		{"discovered", "rg", &registry.Entry{Source: registry.Discovered, Path: filepath.Join(home, "gone")}, false},
	}
	for _, tt := range tests {
		if got := Missing(env, tt.key, tt.e); got != tt.want {
			t.Errorf("%s: Missing = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestPresent(t *testing.T) {
	env := platform.Env{GOOS: "linux", Home: t.TempDir()}
	r := registry.New()
	r.LastSync = time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	r.Tools["old"] = &registry.Entry{Source: registry.Discovered, LastSeen: r.LastSync.Add(-time.Hour)}
	r.Tools["fresh"] = &registry.Entry{Source: registry.Discovered, LastSeen: r.LastSync}
	r.Tools["file://~/gone"] = &registry.Entry{Source: registry.Registered}
	for key, want := range map[string]bool{"old": false, "fresh": true, "file://~/gone": false} {
		if got := Present(env, r, key); got != want {
			t.Errorf("Present(%s) = %v, want %v", key, got, want)
		}
	}
}
