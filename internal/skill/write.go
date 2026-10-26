package skill

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/view"
)

// FileName is the skill file inside each skill folder.
const FileName = "SKILL.md"

// Options wires Write to its environment.
type Options struct {
	Env  platform.Env
	Help func(ctx context.Context, env platform.Env, path string) string
	Out  io.Writer
}

// Write renders one SKILL.md per key into outDir/<category>/<slug>/. It
// overwrites files climan wrote for the same key and skips any other
// existing file. Keys that map to the same path fail before anything is
// written.
func Write(ctx context.Context, o Options, reg *registry.Registry, keys []string, outDir string) error {
	paths, err := skillPaths(reg, keys, outDir)
	if err != nil {
		return err
	}
	for i, key := range keys {
		e := reg.Tools[key]
		path := paths[i]
		if why := skipReason(path, key); why != "" {
			fmt.Fprintf(o.Out, "skipped: %s (%s)\n", path, why)
			continue
		}
		help := ""
		if e.Path != "" && view.Present(o.Env, reg, key) {
			help = o.Help(ctx, o.Env, e.Path)
		}
		doc, err := Render(key, e, help)
		if err != nil {
			return fmt.Errorf("render %s: %w", key, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(o.Out, "wrote: %s\n", path)
	}
	return nil
}

// skillPaths maps each key to its SKILL.md path, failing when two keys
// slug to the same one.
func skillPaths(reg *registry.Registry, keys []string, outDir string) ([]string, error) {
	paths := make([]string, len(keys))
	owners := make(map[string]string, len(keys))
	for i, key := range keys {
		path := filepath.Join(outDir, Category(reg.Tools[key]), Slug(key), FileName)
		if other, ok := owners[path]; ok {
			return nil, fmt.Errorf("%s and %s would both write %s", other, key, path)
		}
		owners[path] = key
		paths[i] = path
	}
	return paths, nil
}

// skipReason explains why an existing file at path must not be replaced
// with key's skill, or returns "" when it may be.
func skipReason(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if !strings.Contains(string(data), Marker) {
		return "not written by climan"
	}
	if owner := owner(string(data)); owner != "" && owner != key {
		return "written for " + owner
	}
	return ""
}

// owner is the key a generated SKILL.md was written for: the heading Render
// puts right after the marker.
func owner(doc string) string {
	_, rest, ok := strings.Cut(doc, Marker+"\n\n# ")
	if !ok {
		return ""
	}
	key, _, _ := strings.Cut(rest, "\n")
	return key
}
