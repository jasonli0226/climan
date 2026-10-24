// Package register adds tools by hand: scripts off PATH, shell aliases and
// MCP servers.
package register

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/status"
)

var (
	// ErrBadID means the ID is not one of the supported forms.
	ErrBadID = errors.New("id must be file://<path>, alias://<name> or mcp://<name>")
	// ErrExists means the key is already in the registry.
	ErrExists = errors.New("already in the registry")
)

var (
	nameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	driveRe = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

// ParseID normalizes an ID; cwd resolves relative paths. It returns the
// registry key and, for file:// IDs, the absolute target path.
func ParseID(env platform.Env, cwd, id string) (key, target string, err error) {
	for _, scheme := range []string{"alias://", "mcp://"} {
		if name, ok := strings.CutPrefix(id, scheme); ok {
			if !nameRe.MatchString(name) {
				return "", "", fmt.Errorf("%w (bad name %q)", ErrBadID, name)
			}
			return id, "", nil
		}
	}
	p, isFile := strings.CutPrefix(id, "file://")
	if !isFile && !looksLikePath(p) {
		return "", "", fmt.Errorf("%w (got %q)", ErrBadID, id)
	}
	if p == "" {
		return "", "", fmt.Errorf("%w (empty path)", ErrBadID)
	}
	switch {
	case p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`):
		return "file://" + p, env.Home + p[1:], nil
	case isAbs(env, p):
		return "file://" + p, p, nil
	default:
		abs := join(env, cwd, p)
		return "file://" + abs, abs, nil
	}
}

func looksLikePath(p string) bool {
	return strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") ||
		strings.HasPrefix(p, ".") || driveRe.MatchString(p)
}

func isAbs(env platform.Env, p string) bool {
	if env.GOOS == "windows" {
		return driveRe.MatchString(p) || strings.HasPrefix(p, `\\`)
	}
	return strings.HasPrefix(p, "/")
}

// join resolves rel against cwd using env's path rules. When env matches the
// host, the host's rules apply; otherwise (tests) env's separator is used.
func join(env platform.Env, cwd, rel string) string {
	if env.GOOS == runtime.GOOS {
		return filepath.Join(cwd, rel)
	}
	if env.GOOS == "windows" {
		rel = strings.TrimPrefix(strings.TrimPrefix(rel, `.\`), "./")
		return strings.TrimRight(cwd, `\/`) + `\` + rel
	}
	return path.Join(cwd, rel)
}

// Add registers id as an enabled tool. file:// targets must be existing
// regular files. It never probes.
func Add(store registry.Store, env platform.Env, cwd, id, desc string, tags []string) (string, error) {
	key, target, err := ParseID(env, cwd, id)
	if err != nil {
		return "", err
	}
	if target != "" {
		info, err := os.Stat(target)
		if err != nil {
			return "", fmt.Errorf("register %s: %w", target, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("register %s: not a file", target)
		}
	}
	cleaned, err := status.CleanDesc(desc)
	if err != nil {
		return "", err
	}
	err = store.Update(func(reg *registry.Registry) error {
		if reg.Tools[key] != nil {
			return fmt.Errorf("%s is %w; use: climan enable %s --desc", key, ErrExists, key)
		}
		reg.Tools[key] = &registry.Entry{
			Status: registry.Enabled, Source: registry.Registered,
			Path: target, Desc: cleaned, Tags: tags,
		}
		return nil
	})
	return key, err
}
