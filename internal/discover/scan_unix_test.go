//go:build !windows

package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanDedupesSymlinkedDirs(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	mkExec(t, a, "tool")
	b := filepath.Join(root, "b")
	if err := os.Symlink(a, b); err != nil {
		t.Fatal(err)
	}

	found := Scan(hostEnv(a, b))
	if len(found) != 1 {
		t.Fatalf("got %d entries, want 1: %+v", len(found), found)
	}
	if len(found[0].Shadowed) != 0 {
		t.Errorf("Shadowed = %v, want none", found[0].Shadowed)
	}
}

func TestScanSkipsBrokenSymlinksAndDirs(t *testing.T) {
	dir := t.TempDir()
	mkExec(t, dir, "good")
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "broken")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := names(Scan(hostEnv(dir)))
	if !reflect.DeepEqual(got, []string{"good"}) {
		t.Fatalf("names = %v, want [good]", got)
	}
}

func TestScanLinuxNeedsExecBit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "plain", "", 0o644)
	mkExec(t, dir, "runnable")

	got := names(Scan(hostEnv(dir)))
	if !reflect.DeepEqual(got, []string{"runnable"}) {
		t.Fatalf("names = %v, want [runnable]", got)
	}
}
