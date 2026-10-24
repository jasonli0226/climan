package discover

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jasonli0226/climan/internal/platform"
)

func names(found []Found) []string {
	var out []string
	for _, f := range found {
		out = append(out, f.Name)
	}
	return out
}

func TestScanShadowing(t *testing.T) {
	root := t.TempDir()
	d1, d2 := filepath.Join(root, "d1"), filepath.Join(root, "d2")
	first := mkExec(t, d1, "x")
	second := mkExec(t, d2, "x")

	found := Scan(hostEnv(d1, d2))
	if len(found) != 1 {
		t.Fatalf("got %d entries, want 1: %+v", len(found), found)
	}
	if found[0].Path != first {
		t.Errorf("Path = %q, want %q", found[0].Path, first)
	}
	if !reflect.DeepEqual(found[0].Shadowed, []string{second}) {
		t.Errorf("Shadowed = %v, want [%s]", found[0].Shadowed, second)
	}
}

func TestScanSkipsJunkEntries(t *testing.T) {
	dir := t.TempDir()
	mkExec(t, dir, "good")
	env := hostEnv("", ".", filepath.Join("rel", "dir"), filepath.Join(dir, "nonexistent"), dir)

	got := names(Scan(env))
	if !reflect.DeepEqual(got, []string{"good"}) {
		t.Fatalf("names = %v, want [good]", got)
	}
}

func TestScanWindowsPathExt(t *testing.T) {
	dir := t.TempDir()
	exe := writeFile(t, dir, "rg.exe", "", 0o644)
	writeFile(t, dir, "rg.cmd", "", 0o644)
	writeFile(t, dir, "notes.txt", "", 0o644)

	found := Scan(windowsEnv(dir))
	if len(found) != 1 || found[0].Name != "rg" {
		t.Fatalf("found = %+v, want one entry rg", found)
	}
	if found[0].Path != exe {
		t.Errorf("Path = %q, want %q (earliest PATHEXT wins)", found[0].Path, exe)
	}
	if len(found[0].Shadowed) != 0 {
		t.Errorf("Shadowed = %v, want none", found[0].Shadowed)
	}
}

func TestScanWindowsCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	d1, d2 := filepath.Join(root, "d1"), filepath.Join(root, "d2")
	writeFile(t, d1, "Python.EXE", "", 0o644)
	second := writeFile(t, d2, "python.exe", "", 0o644)

	found := Scan(windowsEnv(d1, d2))
	if len(found) != 1 || found[0].Name != "python" {
		t.Fatalf("found = %+v, want one entry python", found)
	}
	if !reflect.DeepEqual(found[0].Shadowed, []string{second}) {
		t.Errorf("Shadowed = %v, want [%s]", found[0].Shadowed, second)
	}
}

func TestScanSortedByName(t *testing.T) {
	dir := t.TempDir()
	mkExec(t, dir, "zeta")
	mkExec(t, dir, "alpha")
	got := names(Scan(hostEnv(dir)))
	if !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("names = %v, want [alpha zeta]", got)
	}
}

func TestTagFor(t *testing.T) {
	linux := platform.Env{GOOS: "linux", Home: "/home/u"}
	for _, dir := range []string{"/home/u/.cargo/bin", "/usr/local/bin", "/opt/x/bin", "/snap/bin", "/srv/bin"} {
		if got := tagFor(linux, dir); got != TagInstalled {
			t.Errorf("linux tagFor(%q) = %q, want installed", dir, got)
		}
	}
	for _, dir := range linux.CoreDirs() {
		if got := tagFor(linux, dir); got != TagCore {
			t.Errorf("linux tagFor(%q) = %q, want core", dir, got)
		}
	}
	if got := tagFor(linux, "/usr/bin/"); got != TagCore {
		t.Errorf("linux tagFor(/usr/bin/) = %q, want core", got)
	}

	win := windowsEnv()
	if got := tagFor(win, `C:\WINDOWS\system32`); got != TagCore {
		t.Errorf("windows tagFor(system32) = %q, want core", got)
	}
	if got := tagFor(win, `C:\ProgramData\chocolatey\bin`); got != TagInstalled {
		t.Errorf("windows tagFor(chocolatey) = %q, want installed", got)
	}
}

func TestSkipDirWSL(t *testing.T) {
	wsl := platform.Env{GOOS: "linux", Vars: map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}}
	if !skipDir(wsl, "/mnt/c/Windows") {
		t.Error("skipDir(wsl, /mnt/c/Windows) = false, want true")
	}
	if skipDir(wsl, "/mnt/data") {
		t.Error("skipDir(wsl, /mnt/data) = true, want false")
	}
	plain := platform.Env{GOOS: "linux", Vars: map[string]string{}}
	if skipDir(plain, "/mnt/c/Windows") {
		t.Error("skipDir(non-wsl, /mnt/c/Windows) = true, want false")
	}
}
