package discover

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasonli0226/climan/internal/platform"
)

// maxShimSize is the size below which a script may be a shim; real tools
// written as scripts are usually larger.
const maxShimSize = 4096

// versionManagers are checked in order against a script's text.
var versionManagers = []string{"pyenv", "asdf", "mise", "rbenv", "nodenv", "volta"}

// DetectShim returns the kind of shim at path, or "" for a plain command.
// It only reads files; it never executes them.
func DetectShim(env platform.Env, path string) string {
	if env.GOOS == "windows" {
		if kind := windowsShim(path); kind != "" {
			return kind
		}
	}
	text, ok := readScript(path)
	if !ok {
		return ""
	}
	for _, vm := range versionManagers {
		if strings.Contains(text, vm) {
			return vm
		}
	}
	if lastElem(filepath.Dir(path)) == "shims" {
		return "shims-dir"
	}
	return ""
}

// windowsShim recognizes scoop shims (a sibling .shim file naming the target)
// and npm's .cmd wrappers.
func windowsShim(path string) string {
	ext := filepath.Ext(path)
	if data, err := os.ReadFile(strings.TrimSuffix(path, ext) + ".shim"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			k, v, ok := strings.Cut(line, "=")
			if ok && strings.TrimSpace(k) == "path" {
				return "scoop:" + strings.Trim(strings.TrimSpace(v), `"`)
			}
		}
		return "scoop"
	}
	if strings.EqualFold(ext, ".cmd") && lastElem(filepath.Dir(path)) == "npm" {
		return "npm-cmd"
	}
	return ""
}

// readScript returns the text of path when it is a small "#!" script.
func readScript(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() >= maxShimSize {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, maxShimSize))
	if err != nil || !strings.HasPrefix(string(buf), "#!") {
		return "", false
	}
	return string(buf), true
}

// lastElem returns the lower-cased last element of dir, accepting either
// path separator.
func lastElem(dir string) string {
	dir = strings.TrimRight(dir, `/\`)
	if i := strings.LastIndexAny(dir, `/\`); i >= 0 {
		dir = dir[i+1:]
	}
	return strings.ToLower(dir)
}
