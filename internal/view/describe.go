// Package view renders the registry for people (list) and agents (context).
package view

import (
	"errors"
	"os"
	"strings"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/probe"
	"github.com/jasonli0226/climan/internal/registry"
)

// Description returns the text to show for e: the user's desc if set,
// otherwise the probed summary, flattened to one line. weak reports a
// summary that does not describe the tool (see probe.Weak).
func Description(e *registry.Entry) (text string, weak bool) {
	if d := oneLine(e.Desc); d != "" {
		return d, false
	}
	s := oneLine(e.Summary)
	return s, probe.Weak(s, oneLine(e.Version))
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Missing reports a registered entry whose target no longer exists. Only
// file:// keys and entry paths are checked; aliases and MCP servers cannot
// be, and discovered entries are covered by staleness instead.
func Missing(env platform.Env, key string, e *registry.Entry) bool {
	if e.Source != registry.Registered {
		return false
	}
	target := e.Path
	if target == "" {
		if strings.HasPrefix(key, "alias://") || strings.HasPrefix(key, "mcp://") {
			return false
		}
		target = strings.TrimPrefix(key, "file://")
	}
	if target == "~" || strings.HasPrefix(target, "~/") || strings.HasPrefix(target, `~\`) {
		target = env.Home + target[1:]
	}
	_, err := os.Stat(target)
	return errors.Is(err, os.ErrNotExist)
}

// Present reports whether the tool behind key can be run now.
func Present(env platform.Env, reg *registry.Registry, key string) bool {
	e, ok := reg.Tools[key]
	return ok && !reg.Stale(key) && !Missing(env, key, e)
}
