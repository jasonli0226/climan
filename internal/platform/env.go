// Package platform describes the operating system climan runs on, as a value
// so that Windows behavior can be tested on Linux and vice versa.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ConfigDirVar overrides the config directory when set.
const ConfigDirVar = "CLIMAN_CONFIG_DIR"

// Env is a snapshot of the parts of the environment climan depends on.
type Env struct {
	GOOS    string   // "linux" or "windows"
	Path    string   // raw PATH value
	PathExt []string // lower-case, e.g. [".com", ".exe"]; nil on linux
	Home    string
	Vars    map[string]string // the full environment
}

// Current returns the Env of the running process.
func Current() Env {
	vars := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			vars[k] = v
		}
	}
	home, _ := os.UserHomeDir()
	env := Env{GOOS: runtime.GOOS, Path: os.Getenv("PATH"), Home: home, Vars: vars}
	if env.GOOS == "windows" {
		for _, ext := range strings.Split(os.Getenv("PATHEXT"), ";") {
			if ext != "" {
				env.PathExt = append(env.PathExt, strings.ToLower(ext))
			}
		}
	}
	return env
}

// ConfigDir returns the directory holding registry.yaml and rules.yaml.
func (e Env) ConfigDir() (string, error) {
	if dir := e.Vars[ConfigDirVar]; dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "climan"), nil
}

// CoreDirs returns the system directories whose tools are tagged core.
func (e Env) CoreDirs() []string {
	if e.GOOS == "windows" {
		root := e.Vars["SystemRoot"]
		if root == "" {
			root = `C:\Windows`
		}
		return []string{root + `\System32`, root}
	}
	return []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/usr/games"}
}

// IsWSL reports whether climan runs inside Windows Subsystem for Linux.
func (e Env) IsWSL() bool {
	return e.Vars["WSL_DISTRO_NAME"] != ""
}

// SplitPath splits Path by the separator of e.GOOS (not the host OS) and
// drops empty items.
func (e Env) SplitPath() []string {
	sep := ":"
	if e.GOOS == "windows" {
		sep = ";"
	}
	var dirs []string
	for _, d := range strings.Split(e.Path, sep) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}
