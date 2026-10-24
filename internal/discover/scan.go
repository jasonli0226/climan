// Package discover turns the PATH into a list of tools. It only reads
// directory listings, file metadata and small script headers; it never runs
// the tools it finds.
package discover

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jasonli0226/climan/internal/platform"
)

// Tag says where a tool lives.
type Tag string

const (
	TagCore      Tag = "core"
	TagInstalled Tag = "installed"
)

// Found is one command on PATH.
type Found struct {
	Name     string   // command name; lower-cased and extension-free on windows
	Path     string   // winning path
	Dir      string   // dir of Path, as listed on PATH (cleaned)
	Tag      Tag
	Shim     string   // shim kind; "" for plain binaries
	Shadowed []string // later paths with the same name, in PATH order
}

// Scan lists every command on env's PATH, sorted by name. The first
// occurrence of a name wins, as in the shell; later ones are Shadowed.
func Scan(env platform.Env) []Found {
	byName := map[string]*Found{}
	seenDir := map[string]bool{}

	for _, raw := range env.SplitPath() {
		if !filepath.IsAbs(raw) || skipDir(env, raw) {
			continue
		}
		dir := filepath.Clean(raw)
		key, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue
		}
		if env.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seenDir[key] {
			continue
		}
		seenDir[key] = true

		for name, path := range commandsIn(env, dir) {
			if f, ok := byName[name]; ok {
				f.Shadowed = append(f.Shadowed, path)
				continue
			}
			byName[name] = &Found{Name: name, Path: path, Dir: dir, Tag: tagFor(env, dir)}
		}
	}

	out := make([]Found, 0, len(byName))
	for _, f := range byName {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// commandsIn maps command name to path for the executables in dir.
func commandsIn(env platform.Env, dir string) map[string]string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	cmds := map[string]string{}
	rank := map[string]int{}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path) // follows symlinks
		if err != nil || info.IsDir() {
			continue
		}
		if env.GOOS != "windows" {
			if info.Mode()&0o111 != 0 {
				cmds[e.Name()] = path
			}
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		r := extRank(env.PathExt, ext)
		if r < 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		if prev, ok := rank[name]; ok && prev <= r {
			continue
		}
		rank[name] = r
		cmds[name] = path
	}
	return cmds
}

func extRank(pathExt []string, ext string) int {
	if ext == "" {
		return -1
	}
	for i, e := range pathExt {
		if e == ext {
			return i
		}
	}
	return -1
}

// tagFor tags dir as core when it is one of env's system dirs. Comparison
// uses env's path conventions, not the host's.
func tagFor(env platform.Env, dir string) Tag {
	norm := func(p string) string {
		if env.GOOS == "windows" {
			p = strings.ToLower(strings.ReplaceAll(p, "/", `\`))
			return strings.TrimRight(p, `\`)
		}
		if len(p) > 1 {
			p = strings.TrimRight(p, "/")
		}
		return p
	}
	d := norm(dir)
	for _, core := range env.CoreDirs() {
		if d == norm(core) {
			return TagCore
		}
	}
	return TagInstalled
}

// skipDir reports whether a PATH dir is skipped entirely: under WSL, the
// Windows drives mounted at /mnt/<letter> belong to the Windows side.
func skipDir(env platform.Env, dir string) bool {
	if env.GOOS == "windows" || !env.IsWSL() {
		return false
	}
	rest, ok := strings.CutPrefix(dir, "/mnt/")
	if !ok {
		return false
	}
	drive, _, _ := strings.Cut(rest, "/")
	return len(drive) == 1
}
