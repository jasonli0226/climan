package register

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/view"
)

func TestParseID(t *testing.T) {
	env := platform.Env{GOOS: "linux", Home: "/home/u"}
	tests := []struct{ id, key, path string }{
		{"file:///opt/x", "file:///opt/x", "/opt/x"},
		{"~/bin/backup", "file://~/bin/backup", "/home/u/bin/backup"},
		{"file://~/bin/backup", "file://~/bin/backup", "/home/u/bin/backup"},
		{"./b.sh", "file:///work/b.sh", "/work/b.sh"},
		{"/opt/y", "file:///opt/y", "/opt/y"},
		{"alias://gs", "alias://gs", ""},
		{"mcp://github", "mcp://github", ""},
	}
	for _, tt := range tests {
		key, path, err := ParseID(env, "/work", tt.id)
		if err != nil || key != tt.key || path != tt.path {
			t.Errorf("ParseID(%q) = %q, %q, %v; want %q, %q", tt.id, key, path, err, tt.key, tt.path)
		}
	}
	for _, bad := range []string{"alias://", "alias://a b", "mcp://", "gs", "http://x", "file://"} {
		if _, _, err := ParseID(env, "/work", bad); !errors.Is(err, ErrBadID) {
			t.Errorf("ParseID(%q) err = %v, want ErrBadID", bad, err)
		}
	}
}

func TestParseIDWindowsDrive(t *testing.T) {
	env := platform.Env{GOOS: "windows", Home: `C:\Users\u`}
	key, path, err := ParseID(env, `C:\work`, `C:\tools\x.cmd`)
	if err != nil || key != `file://C:\tools\x.cmd` || path != `C:\tools\x.cmd` {
		t.Fatalf("ParseID = %q, %q, %v", key, path, err)
	}
	key, path, err = ParseID(env, `C:\work`, `.\x.cmd`)
	if err != nil || key != `file://C:\work\x.cmd` || path != `C:\work\x.cmd` {
		t.Fatalf("ParseID(relative) = %q, %q, %v", key, path, err)
	}
}

func hostEnv(home string) platform.Env {
	return platform.Env{GOOS: runtime.GOOS, Home: home, Vars: map[string]string{}}
}

func TestAddFile(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "backup.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := registry.Store{Dir: filepath.Join(dir, "config")}
	env := hostEnv(t.TempDir())
	key, err := Add(store, env, dir, "./backup.sh", "nightly backup", []string{"ops"})
	if err != nil {
		t.Fatal(err)
	}
	reg, _ := store.Load()
	e := reg.Tools[key]
	if e == nil || e.Status != registry.Enabled || e.Source != registry.Registered ||
		e.Path != script || e.Desc != "nightly backup" || !reflect.DeepEqual(e.Tags, []string{"ops"}) {
		t.Fatalf("entry %s = %+v", key, e)
	}
	t.Chdir(t.TempDir())
	if !view.Present(env, reg, key) {
		t.Fatal("registered script not present from another cwd")
	}
}

func TestAddMissingFile(t *testing.T) {
	dir := t.TempDir()
	store := registry.Store{Dir: filepath.Join(dir, "config")}
	_, err := Add(store, hostEnv(dir), dir, "./nope.sh", "", nil)
	if err == nil || !strings.Contains(err.Error(), "nope.sh") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(store.Path()); statErr == nil {
		t.Fatal("registry written despite error")
	}
}

func TestAddDirectory(t *testing.T) {
	dir := t.TempDir()
	store := registry.Store{Dir: filepath.Join(dir, "config")}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(store, hostEnv(dir), dir, "./sub", "", nil); err == nil || !strings.Contains(err.Error(), "not a file") {
		t.Fatalf("err = %v", err)
	}
}

func TestAddExists(t *testing.T) {
	dir := t.TempDir()
	store := registry.Store{Dir: filepath.Join(dir, "config")}
	if _, err := Add(store, hostEnv(dir), dir, "alias://gs", "git status", nil); err != nil {
		t.Fatal(err)
	}
	_, err := Add(store, hostEnv(dir), dir, "alias://gs", "", nil)
	if !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "climan enable alias://gs --desc") {
		t.Fatalf("err = %v", err)
	}
}

func TestAddAlias(t *testing.T) {
	dir := t.TempDir()
	store := registry.Store{Dir: filepath.Join(dir, "config")}
	env := hostEnv(dir)
	if _, err := Add(store, env, dir, "alias://gs", "git status", nil); err != nil {
		t.Fatal(err)
	}
	reg, _ := store.Load()
	if reg.Tools["alias://gs"].Path != "" || !strings.Contains(view.Context(env, reg), "alias://gs: git status\n") {
		t.Fatalf("entry %+v, context %q", reg.Tools["alias://gs"], view.Context(env, reg))
	}
}
