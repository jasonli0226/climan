// Package reconcile merges a PATH scan into the registry. It never deletes
// entries and never touches registered ones.
package reconcile

import (
	"sort"
	"time"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/registry"
)

// Result lists what changed in one reconcile.
type Result struct {
	New   []string // keys added as pending (prompt candidates), sorted
	Noise []string // keys added as ignored because they are noise, sorted
	Stale []string // discovered keys not found this run, sorted
}

// Reconcile updates reg from tools found at time now.
func Reconcile(reg *registry.Registry, tools []discover.Tool, now time.Time) Result {
	var res Result
	seen := map[string]bool{}

	for _, t := range tools {
		seen[t.Key] = true
		e, exists := reg.Tools[t.Key]
		switch {
		case exists && e.Source == registry.Registered:
			continue
		case !exists && t.Tag == discover.TagCore:
			continue
		case !exists:
			e = &registry.Entry{Status: registry.Pending, Source: registry.Discovered}
			if t.Noise {
				e.Status = registry.Ignored
				res.Noise = append(res.Noise, t.Key)
			} else {
				res.New = append(res.New, t.Key)
			}
			reg.Tools[t.Key] = e
		}
		observe(e, t, now)
	}

	for key, e := range reg.Tools {
		if !seen[key] && e.Source == registry.Discovered {
			res.Stale = append(res.Stale, key)
		}
	}
	reg.LastSync = now

	sort.Strings(res.New)
	sort.Strings(res.Noise)
	sort.Strings(res.Stale)
	return res
}

// Adopt creates the pending discovered entry sync would create for t, stores
// it under t.Key and returns it.
func Adopt(reg *registry.Registry, t discover.Tool, now time.Time) *registry.Entry {
	e := &registry.Entry{Status: registry.Pending, Source: registry.Discovered}
	observe(e, t, now)
	reg.Tools[t.Key] = e
	return e
}

// observe records what the scan saw. Status and curated or probed fields
// (Version, Summary, Tags, Notes) are left alone.
func observe(e *registry.Entry, t discover.Tool, now time.Time) {
	e.Tag = string(t.Tag)
	e.LastSeen = now
	e.Members = nil
	e.Shadowed = nil
	winner := t.Members[0]
	for _, m := range t.Members {
		e.Members = append(e.Members, m.Name)
		e.Shadowed = append(e.Shadowed, m.Shadowed...)
		if m.Name == t.Key {
			winner = m
		}
	}
	e.Path = winner.Path
	e.Shim = winner.Shim
}
