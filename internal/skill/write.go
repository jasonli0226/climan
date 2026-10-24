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
// overwrites files climan wrote and skips any other existing file.
func Write(ctx context.Context, o Options, reg *registry.Registry, keys []string, outDir string) error {
	for _, key := range keys {
		e := reg.Tools[key]
		path := filepath.Join(outDir, Category(e), Slug(key), FileName)
		if handWritten(path) {
			fmt.Fprintf(o.Out, "skipped: %s (not written by climan)\n", path)
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

// handWritten reports an existing file that lacks climan's marker.
func handWritten(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && !strings.Contains(string(data), Marker)
}
