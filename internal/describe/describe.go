// Package describe implements `climan describe`: everything known about
// one tool.
package describe

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/probe"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/status"
	"github.com/jasonli0226/climan/internal/view"
)

// Options wires Run to its environment.
type Options struct {
	Env   platform.Env
	Store registry.Store
	Probe probe.Func // nil: no probe
	Out   io.Writer
}

// labelWidth fits the longest label, "last seen:".
const labelWidth = 10

// Run describes the tool called name: a registry key or member name, or a
// command found by a fresh scan. With Probe set, it probes the tool now and
// stores the result when the tool is in the registry.
func Run(ctx context.Context, o Options, name string) error {
	reg, err := o.Store.Load()
	if err != nil {
		return err
	}
	if key, ok := status.Lookup(o.Env, reg, name); ok {
		e := reg.Tools[key]
		fmt.Fprint(o.Out, entryBlock(o.Env, reg, key, e))
		return probeEntry(ctx, o, key, e.Path, true)
	}
	t, ok := status.LookupScan(o.Env, discover.Group(discover.Scan(o.Env)), name)
	if !ok {
		return fmt.Errorf("%w: %s", status.ErrUnknown, name)
	}
	fmt.Fprint(o.Out, scanBlock(t))
	return probeEntry(ctx, o, t.Key, t.Members[0].Path, false)
}

func entryBlock(env platform.Env, reg *registry.Registry, key string, e *registry.Entry) string {
	var b strings.Builder
	b.WriteString(key + "\n")
	field(&b, "status", string(e.Status))
	field(&b, "source", string(e.Source))
	field(&b, "tag", e.Tag)
	field(&b, "path", e.Path)
	if len(e.Members) > 1 || (len(e.Members) == 1 && e.Members[0] != key) {
		field(&b, "members", strings.Join(e.Members, ", "))
	}
	field(&b, "shim", e.Shim)
	field(&b, "shadowed", strings.Join(e.Shadowed, ", "))
	field(&b, "version", e.Version)
	field(&b, "summary", e.Summary)
	field(&b, "desc", e.Desc)
	field(&b, "tags", strings.Join(e.Tags, ", "))
	field(&b, "notes", strings.TrimRight(e.Notes, "\n"))
	if !e.LastSeen.IsZero() {
		field(&b, "last seen", e.LastSeen.UTC().Format(time.RFC3339))
	}
	field(&b, "state", state(env, reg, key, e))
	return b.String()
}

func scanBlock(t discover.Tool) string {
	var b strings.Builder
	b.WriteString(t.Key + " (not in registry)\n")
	field(&b, "tag", string(t.Tag))
	field(&b, "path", t.Members[0].Path)
	var names, shadowed []string
	for _, m := range t.Members {
		names = append(names, m.Name)
		shadowed = append(shadowed, m.Shadowed...)
	}
	if len(names) > 1 {
		field(&b, "members", strings.Join(names, ", "))
	}
	field(&b, "shim", t.Members[0].Shim)
	field(&b, "shadowed", strings.Join(shadowed, ", "))
	return b.String()
}

func state(env platform.Env, reg *registry.Registry, key string, e *registry.Entry) string {
	switch {
	case view.Missing(env, key, e):
		return "missing"
	case reg.Stale(key):
		return "stale"
	}
	return "present"
}

// field writes "  <label>: <value>", aligning values and indenting
// continuation lines; empty values are skipped.
func field(b *strings.Builder, label, value string) {
	if value == "" {
		return
	}
	indent := strings.Repeat(" ", 2+labelWidth+1)
	lines := strings.Split(value, "\n")
	fmt.Fprintf(b, "  %-*s %s\n", labelWidth, label+":", lines[0])
	for _, l := range lines[1:] {
		b.WriteString(indent + l + "\n")
	}
}

func probeEntry(ctx context.Context, o Options, key, path string, store bool) error {
	if o.Probe == nil || path == "" {
		return nil
	}
	info := o.Probe(ctx, o.Env, path)
	fmt.Fprintf(o.Out, "  probed version: %s\n  probed summary: %s\n", info.Version, info.Summary)
	if !store {
		return nil
	}
	return o.Store.Update(func(reg *registry.Registry) error {
		if e := reg.Tools[key]; e != nil {
			e.Version, e.Summary = info.Version, info.Summary
		}
		return nil
	})
}
