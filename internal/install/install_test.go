package install

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var doc = []byte("---\nname: climan\n---\n" + Marker + "\nbody\n")

func tempAgent(t *testing.T, name string) Agent {
	t.Helper()
	return agent(name, filepath.Join(t.TempDir(), name))
}

func skillFile(a Agent) string { return filepath.Join(a.SkillsDir, "climan", FileName) }

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func installOne(t *testing.T, a Agent) Result {
	t.Helper()
	results := Install([]Agent{a}, doc)
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	return results[0]
}

func TestInstallNew(t *testing.T) {
	a := tempAgent(t, "claude")
	r := installOne(t, a)
	if r.Outcome != Installed || r.Path != skillFile(a) || r.Agent != "claude" {
		t.Fatalf("result = %+v", r)
	}
	data, err := os.ReadFile(skillFile(a))
	if err != nil || !bytes.Equal(data, doc) {
		t.Fatalf("file = %q, %v", data, err)
	}
	if fi, _ := os.Stat(skillFile(a)); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", fi.Mode().Perm())
	}
}

func TestInstallUnchanged(t *testing.T) {
	a := tempAgent(t, "pi")
	installOne(t, a)
	if r := installOne(t, a); r.Outcome != Unchanged {
		t.Fatalf("rerun = %+v, want unchanged", r)
	}
}

func TestInstallOverwritesMarked(t *testing.T) {
	a := tempAgent(t, "pi")
	writeFile(t, skillFile(a), Marker+"\nold version\n")
	if r := installOne(t, a); r.Outcome != Installed {
		t.Fatalf("result = %+v", r)
	}
	if data, _ := os.ReadFile(skillFile(a)); !bytes.Equal(data, doc) {
		t.Fatalf("not overwritten: %q", data)
	}
}

func TestInstallSkipsHandWritten(t *testing.T) {
	a := tempAgent(t, "pi")
	writeFile(t, skillFile(a), "mine\n")
	r := installOne(t, a)
	if r.Outcome != Skipped || r.Reason != "not written by climan" {
		t.Fatalf("result = %+v", r)
	}
	if data, _ := os.ReadFile(skillFile(a)); string(data) != "mine\n" {
		t.Fatalf("file changed: %q", data)
	}
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

func TestInstallSkipsSymlinkedDir(t *testing.T) {
	a := tempAgent(t, "claude")
	target := filepath.Join(t.TempDir(), "climan")
	writeFile(t, filepath.Join(target, FileName), Marker+"\nrepo copy\n")
	link := filepath.Join(a.SkillsDir, "climan")
	symlinkOrSkip(t, target, link)
	r := installOne(t, a)
	if r.Outcome != Skipped || r.Path != link || r.Reason != "symlink to "+target {
		t.Fatalf("result = %+v", r)
	}
	if data, _ := os.ReadFile(filepath.Join(target, FileName)); string(data) != Marker+"\nrepo copy\n" {
		t.Fatalf("target changed: %q", data)
	}
}

func TestInstallSkipsSymlinkedFile(t *testing.T) {
	a := tempAgent(t, "claude")
	target := filepath.Join(t.TempDir(), FileName)
	writeFile(t, target, Marker+"\nrepo copy\n")
	symlinkOrSkip(t, target, skillFile(a))
	r := installOne(t, a)
	if r.Outcome != Skipped || r.Path != skillFile(a) || r.Reason != "symlink to "+target {
		t.Fatalf("result = %+v", r)
	}
	if data, _ := os.ReadFile(target); string(data) != Marker+"\nrepo copy\n" {
		t.Fatalf("target changed: %q", data)
	}
}

func TestInstallFailureIsolated(t *testing.T) {
	a, b := tempAgent(t, "claude"), tempAgent(t, "pi")
	writeFile(t, a.SkillsDir, "not a dir")
	results := Install([]Agent{a, b}, doc)
	if len(results) != 2 || results[0].Outcome != Failed || results[0].Reason == "" || results[1].Outcome != Installed {
		t.Fatalf("results = %+v", results)
	}
}

func TestInstallNoTempLeftOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	a := tempAgent(t, "pi")
	old := Marker + "\nold\n"
	writeFile(t, skillFile(a), old)
	dir := filepath.Dir(skillFile(a))
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if r := installOne(t, a); r.Outcome != Failed {
		t.Fatalf("result = %+v, want failed", r)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != FileName {
		t.Fatalf("dir holds %v", entries)
	}
	if data, _ := os.ReadFile(skillFile(a)); string(data) != old {
		t.Fatalf("file changed: %q", data)
	}
}

func TestResultString(t *testing.T) {
	if got := (Result{Path: "/p", Outcome: Skipped, Reason: "r"}).String(); got != "skipped: /p (r)" {
		t.Errorf("got %q", got)
	}
	if got := (Result{Path: "/p", Outcome: Installed}).String(); got != "installed: /p" {
		t.Errorf("got %q", got)
	}
}
