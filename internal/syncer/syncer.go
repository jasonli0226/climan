// Package syncer runs `climan sync`: scan, ask, write, probe.
package syncer

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/probe"
	"github.com/jasonli0226/climan/internal/reconcile"
	"github.com/jasonli0226/climan/internal/registry"
)

// Options wires Run to its environment.
type Options struct {
	Env     platform.Env
	Store   registry.Store
	Chooser Chooser
	Now     func() time.Time
	Probe   func(ctx context.Context, env platform.Env, path string) probe.Info
	Out     io.Writer
}

// Run scans PATH, asks the Chooser about new tools, and writes the registry.
// The Chooser runs outside the lock (the user may take minutes); the
// registry is re-read under the lock before writing, so edits made in the
// meantime are kept. Newly enabled installed tools are probed afterwards,
// also outside the lock.
func Run(ctx context.Context, o Options) error {
	tools := discover.Group(discover.Scan(o.Env))
	byKey := map[string]discover.Tool{}
	for _, t := range tools {
		byKey[t.Key] = t
	}
	now := o.Now()

	preview, err := o.Store.Load()
	if err != nil {
		return err
	}
	cands := candidates(reconcile.Reconcile(preview, tools, now).New, byKey, preview)

	decisions := map[string]Decision{}
	if len(cands) > 0 {
		if decisions, err = o.Chooser.Choose(cands); err != nil {
			return err
		}
	}

	var res reconcile.Result
	var toProbe []string
	err = o.Store.Update(func(reg *registry.Registry) error {
		res = reconcile.Reconcile(reg, tools, now)
		toProbe = apply(reg, res.New, decisions)
		return nil
	})
	if err != nil {
		return err
	}
	if err := probeAll(ctx, o, toProbe); err != nil {
		return err
	}
	printSummary(o.Out, res.New, decisions, len(res.Stale))
	return nil
}

func candidates(keys []string, byKey map[string]discover.Tool, reg *registry.Registry) []Candidate {
	var out []Candidate
	for _, k := range keys {
		e := reg.Tools[k]
		out = append(out, Candidate{Key: k, Path: e.Path, Shim: e.Shim, Members: e.Members, Tool: byKey[k]})
	}
	return out
}

// apply sets each new tool's status and returns the keys of newly enabled
// installed tools, which are the ones to probe.
func apply(reg *registry.Registry, newKeys []string, decisions map[string]Decision) []string {
	var enabled []string
	for _, k := range newKeys {
		d, ok := decisions[k]
		if !ok || d.Status == "" {
			d.Status = registry.Pending
		}
		e := reg.Tools[k]
		e.Status = d.Status
		for _, tag := range d.Tags {
			if !slices.Contains(e.Tags, tag) {
				e.Tags = append(e.Tags, tag)
			}
		}
		if e.Status == registry.Enabled && e.Tag == string(discover.TagInstalled) {
			enabled = append(enabled, k)
		}
	}
	return enabled
}

func probeAll(ctx context.Context, o Options, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	reg, err := o.Store.Load()
	if err != nil {
		return err
	}
	infos := map[string]probe.Info{}
	for _, k := range keys {
		if e := reg.Tools[k]; e != nil && e.Path != "" {
			infos[k] = o.Probe(ctx, o.Env, e.Path)
		}
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

func printSummary(w io.Writer, newKeys []string, decisions map[string]Decision, stale int) {
	count := map[registry.Status]int{}
	for _, k := range newKeys {
		s := decisions[k].Status
		if s == "" {
			s = registry.Pending
		}
		count[s]++
	}
	fmt.Fprintf(w, "sync: new: %d, enabled: %d, ignored: %d, pending: %d, stale: %d\n",
		len(newKeys), count[registry.Enabled], count[registry.Ignored], count[registry.Pending], stale)
}
