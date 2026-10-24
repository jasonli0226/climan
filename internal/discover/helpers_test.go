package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jasonli0226/climan/internal/platform"
)

// exeName returns name as an executable file name on the host OS.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// writeFile creates dir/name with the given content and mode, creating dir.
func writeFile(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

// mkExec creates an executable for the host OS and returns its path.
func mkExec(t *testing.T, dir, name string) string {
	return writeFile(t, dir, exeName(name), "#!/bin/sh\n", 0o755)
}

// hostEnv builds an Env for the host OS whose PATH is dirs.
func hostEnv(dirs ...string) platform.Env {
	env := platform.Env{GOOS: runtime.GOOS, Home: "/nonexistent-home", Vars: map[string]string{}}
	if runtime.GOOS == "windows" {
		env.PathExt = []string{".com", ".exe", ".bat", ".cmd"}
	}
	env.Path = strings.Join(dirs, string(os.PathListSeparator))
	return env
}

// windowsEnv builds a Windows Env whose PATH is dirs, usable on any host.
func windowsEnv(dirs ...string) platform.Env {
	return platform.Env{
		GOOS:    "windows",
		Path:    strings.Join(dirs, ";"),
		PathExt: []string{".exe", ".cmd"},
		Vars:    map[string]string{"SystemRoot": `C:\Windows`},
	}
}
