package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
)

// Mode selects which tools List shows.
type Mode int

const (
	Enabled Mode = iota
	Pending
	Ignored
	Stale
	All
	Unreviewed // enabled, with no desc from the user or an agent yet
)

const coreStatus = "core"

type row struct{ key, status, text string }

// List renders the registry for the user. core holds tools from a fresh
// scan; with All, the core ones missing from the registry are added.
func List(env platform.Env, reg *registry.Registry, mode Mode, core []discover.Tool) string {
	var rows []row
	for key, e := range reg.Tools {
		if !shows(reg, key, e, mode) {
			continue
		}
		rows = append(rows, row{key: key + marker(env, reg, key, e), status: string(e.Status), text: listText(e)})
	}
	if mode == All {
		for _, t := range core {
			if t.Tag == discover.TagCore && reg.Tools[t.Key] == nil {
				rows = append(rows, row{key: t.Key, status: coreStatus})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].key < rows[j].key })

	width := 0
	for _, r := range rows {
		width = max(width, len(r.key))
	}
	var b strings.Builder
	for _, r := range rows {
		var line string
		if mode == All {
			line = fmt.Sprintf("%-*s  %-7s  %s", width, r.key, r.status, r.text)
		} else {
			line = fmt.Sprintf("%-*s  %s", width, r.key, r.text)
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	return b.String()
}

func shows(reg *registry.Registry, key string, e *registry.Entry, mode Mode) bool {
	switch mode {
	case Enabled:
		return e.Status == registry.Enabled
	case Pending:
		return e.Status == registry.Pending
	case Ignored:
		return e.Status == registry.Ignored
	case Stale:
		return reg.Stale(key)
	case Unreviewed:
		return e.Status == registry.Enabled && strings.TrimSpace(e.Desc) == ""
	default:
		return true
	}
}

func marker(env platform.Env, reg *registry.Registry, key string, e *registry.Entry) string {
	switch {
	case reg.Stale(key):
		return " (stale)"
	case Missing(env, key, e):
		return " (missing)"
	}
	return ""
}

// listText shows the description, flagging a weak summary with "?" so the
// user knows which tools need a desc.
func listText(e *registry.Entry) string {
	text, weak := Description(e)
	if !weak {
		return text
	}
	return strings.TrimSpace("? " + text)
}
