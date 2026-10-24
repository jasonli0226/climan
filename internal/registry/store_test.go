package registry

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestUpdateCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "climan")
	s := Store{Dir: dir}
	err := s.Update(func(r *Registry) error {
		r.Tools["rg"] = &Entry{Status: Pending, Source: Discovered}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Tools["rg"] == nil {
		t.Fatalf("rg missing after Update; dir has %v", listDir(t, dir))
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	reg, err := Store{Dir: t.TempDir()}.Load()
	if err != nil || len(reg.Tools) != 0 {
		t.Fatalf("Load = %v, %v; want empty registry", reg, err)
	}
}

func TestUpdateLockBusy(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "registry.yaml.lock")
	held := flock.New(lockPath)
	if err := held.Lock(); err != nil {
		t.Fatal(err)
	}
	defer held.Unlock()

	err := Store{Dir: dir, LockWait: 100 * time.Millisecond}.Update(func(*Registry) error { return nil })
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v, want ErrLocked", err)
	}
	if !strings.Contains(err.Error(), lockPath) {
		t.Fatalf("err %q should name %s", err, lockPath)
	}
}

func TestUpdateFnErrorLeavesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.yaml")
	orig := []byte("version: 1\ntools: {}\n")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := Store{Dir: dir}.Update(func(r *Registry) error {
		r.Tools["x"] = &Entry{Status: Pending}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, orig) {
		t.Fatalf("file changed to %q", got)
	}
	for _, n := range listDir(t, dir) {
		if strings.HasPrefix(n, "registry-") {
			t.Fatalf("temp file %s left behind", n)
		}
	}
}

func TestUpdateParseErrorLeavesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.yaml")
	orig := []byte("tools: [unclosed\n")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	err := Store{Dir: dir}.Update(func(*Registry) error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("err = %v, fn called = %v; want parse error before fn", err, called)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, orig) {
		t.Fatalf("file changed to %q", got)
	}
}
