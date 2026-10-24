// Package status implements `climan enable` and `climan ignore`.
package status

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/probe"
	"github.com/jasonli0226/climan/internal/reconcile"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/view"
)

// ErrUnknown means a name matched no registry entry and no command on PATH.
var ErrUnknown = errors.New("unknown tool")

// Options wires Set to its environment.
type Options struct {
	Env   platform.Env
	Store registry.Store
	Now   func() time.Time
	Probe probe.Func
	Out   io.Writer
}

// Set gives every named tool the status to, in one registry update. A name
// is a registry key or a family member name; names not in the registry are
// adopted from a fresh scan. If any name is unknown, nothing changes. desc,
// when non-nil, is written to the single named tool (the caller ensures one
// name and to == Enabled). Newly enabled installed tools are then probed.
func Set(ctx context.Context, o Options, names []string, to registry.Status, desc *string) error {
	cleaned, err := cleanDesc(desc)
	if err != nil {
		return err
	}
	var lines []string
	toProbe := map[string]string{}
	err = o.Store.Update(func(reg *registry.Registry) error {
		keys, err := resolve(o, reg, names)
		if err != nil {
			return err
		}
		for _, key := range keys {
			e := reg.Tools[key]
			changed := e.Status != to
			e.Status = to
			lines = append(lines, outcome(o.Env, reg, key, to, changed))
			if cleaned != nil {
				e.Desc = *cleaned
				lines = append(lines, descLine(key, *cleaned))
			}
			if changed && to == registry.Enabled && e.Tag == string(discover.TagInstalled) &&
				view.Present(o.Env, reg, key) && e.Path != "" {
				toProbe[key] = e.Path
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Fprintln(o.Out, l)
	}
	return probeAndStore(ctx, o, toProbe)
}

// cleanDesc applies CleanDesc to an optional desc.
func cleanDesc(desc *string) (*string, error) {
	if desc == nil {
		return nil, nil
	}
	s, err := CleanDesc(*desc)
	return &s, err
}

// CleanDesc trims a user description, turns control characters into spaces
// and collapses whitespace; a newline is an error.
func CleanDesc(desc string) (string, error) {
	if strings.ContainsAny(desc, "\r\n") {
		return "", errors.New("--desc must be a single line")
	}
	s := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, desc)
	return strings.Join(strings.Fields(s), " "), nil
}

// resolve maps names to registry keys, adopting tools from a scan when a
// name is not in the registry. Duplicate keys are dropped, keeping order.
func resolve(o Options, reg *registry.Registry, names []string) ([]string, error) {
	var keys, unknown []string
	seen := map[string]bool{}
	var scanned []discover.Tool
	scannedOnce := false

	for _, name := range names {
		key, ok := Lookup(o.Env, reg, name)
		if !ok {
			if !scannedOnce {
				scanned = discover.Group(discover.Scan(o.Env))
				scannedOnce = true
			}
			t, found := LookupScan(o.Env, scanned, name)
			if !found {
				unknown = append(unknown, name)
				continue
			}
			key = t.Key
			if reg.Tools[key] == nil {
				reconcile.Adopt(reg, t, o.Now())
			}
		}
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnknown, strings.Join(unknown, ", "))
	}
	return keys, nil
}

func sameName(env platform.Env, a, b string) bool {
	if env.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Lookup finds name in the registry as a key first, then as a family member.
func Lookup(env platform.Env, reg *registry.Registry, name string) (string, bool) {
	for key := range reg.Tools {
		if sameName(env, key, name) {
			return key, true
		}
	}
	for key, e := range reg.Tools {
		for _, m := range e.Members {
			if sameName(env, m, name) {
				return key, true
			}
		}
	}
	return "", false
}

// LookupScan finds name among scanned tools by key, then member; the noise
// flag is ignored.
func LookupScan(env platform.Env, tools []discover.Tool, name string) (discover.Tool, bool) {
	for _, t := range tools {
		if sameName(env, t.Key, name) {
			return t, true
		}
	}
	for _, t := range tools {
		for _, m := range t.Members {
			if sameName(env, m.Name, name) {
				return t, true
			}
		}
	}
	return discover.Tool{}, false
}

func outcome(env platform.Env, reg *registry.Registry, key string, to registry.Status, changed bool) string {
	if !changed {
		return "unchanged: " + key
	}
	line := string(to) + ": " + key
	if to == registry.Enabled && !view.Present(env, reg, key) {
		line += " (not installed now; hidden from context until it is back)"
	}
	return line
}

func descLine(key, desc string) string {
	if desc == "" {
		return "desc cleared: " + key
	}
	return "desc set: " + key
}

func probeAndStore(ctx context.Context, o Options, paths map[string]string) error {
	infos := probe.All(ctx, o.Env, paths, o.Probe, o.Out)
	if len(infos) == 0 {
		return nil
	}
	return o.Store.Update(func(reg *registry.Registry) error {
		for k, info := range infos {
			if e := reg.Tools[k]; e != nil && e.Status == registry.Enabled {
				e.Version, e.Summary = info.Version, info.Summary
			}
		}
		return nil
	})
}
