package platform

import (
	"reflect"
	"testing"
)

func TestCoreDirsLinux(t *testing.T) {
	got := Env{GOOS: "linux"}.CoreDirs()
	want := []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/usr/games"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CoreDirs() = %v, want %v", got, want)
	}
}

func TestCoreDirsWindows(t *testing.T) {
	env := Env{GOOS: "windows", Vars: map[string]string{"SystemRoot": `C:\Windows`}}
	got := env.CoreDirs()
	want := []string{`C:\Windows\System32`, `C:\Windows`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CoreDirs() = %v, want %v", got, want)
	}
}

func TestSplitPathWindows(t *testing.T) {
	got := Env{GOOS: "windows", Path: `C:\a;;C:\b`}.SplitPath()
	want := []string{`C:\a`, `C:\b`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPath() = %v, want %v", got, want)
	}
}

func TestSplitPathLinux(t *testing.T) {
	got := Env{GOOS: "linux", Path: "/a::/b"}.SplitPath()
	want := []string{"/a", "/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPath() = %v, want %v", got, want)
	}
}

func TestConfigDirOverride(t *testing.T) {
	env := Env{GOOS: "linux", Vars: map[string]string{"CLIMAN_CONFIG_DIR": "/tmp/x"}}
	got, err := env.ConfigDir()
	if err != nil || got != "/tmp/x" {
		t.Fatalf("ConfigDir() = %q, %v; want /tmp/x", got, err)
	}
}

func TestIsWSL(t *testing.T) {
	if (Env{Vars: map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}}).IsWSL() != true {
		t.Fatal("IsWSL() = false with WSL_DISTRO_NAME set")
	}
	if (Env{Vars: map[string]string{}}).IsWSL() {
		t.Fatal("IsWSL() = true without WSL_DISTRO_NAME")
	}
}
