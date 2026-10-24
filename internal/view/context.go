package view

import (
	"sort"
	"strings"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
)

const (
	contextHeader = "Installed CLI tools on this machine (from climan). Prefer one of these over writing a script; run `<tool> --help` for usage."
	contextEmpty  = "No tools enabled yet. Run: climan sync"
)

// Context renders the inventory agents read at the start of every task:
// enabled tools that are present, one line each.
func Context(env platform.Env, reg *registry.Registry) string {
	var keys []string
	for key, e := range reg.Tools {
		if e.Status == registry.Enabled && Present(env, reg, key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(contextHeader + "\n")
	if len(keys) == 0 {
		b.WriteString(contextEmpty + "\n")
	}
	for _, key := range keys {
		b.WriteString(contextLine(key, reg.Tools[key]) + "\n")
	}
	return b.String()
}

// contextLine is "<key> (also <members>): <description> [<tags>]". A weak or
// missing description leaves the name alone: a misleading line is worse.
func contextLine(key string, e *registry.Entry) string {
	line := key
	var others []string
	for _, m := range e.Members {
		if m != key {
			others = append(others, m)
		}
	}
	if len(others) > 0 {
		line += " (also " + strings.Join(others, ", ") + ")"
	}
	if text, weak := Description(e); text != "" && !weak {
		line += ": " + text
		if len(e.Tags) > 0 {
			line += " [" + strings.Join(e.Tags, ", ") + "]"
		}
	}
	return line
}
