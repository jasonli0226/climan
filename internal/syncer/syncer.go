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
	Probe   probe.Func
	Out     io.Writer
	// Review offers the ignored tools found by the scan instead of the
	// pending ones, so the user can enable some after all.
	Review bool
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
	found := reconcile.Reconcile(preview, tools, now).New
	from := registry.Pending
	if o.Review {
		from = registry.Ignored
	}
	cands := candidates(offered(preview, byKey, from), byKey, preview)

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
		toProbe = apply(reg, decisions, from)
		return nil
	})
	if err != nil {
		return err
	}
	if err := probeAll(ctx, o, toProbe); err != nil {
		return err
	}
	printSummary(o.Out, union(found, res.New), union(keysOf(cands), res.New), decisions, len(res.Stale))
	return nil
}

// offered is every discovered tool found by this scan whose status is from,
// sorted. For pending that is new tools plus those left pending by an
// earlier --batch or --auto sync. Noise, ignored on sight, is never offered.
func offered(reg *registry.Registry, byKey map[string]discover.Tool, from registry.Status) []string {
	var keys []string
	for k, e := range reg.Tools {
		t, ok := byKey[k]
		if ok && !t.Noise && e.Source == registry.Discovered && e.Status == from {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

func candidates(keys []string, byKey map[string]discover.Tool, reg *registry.Registry) []Candidate {
	var out []Candidate
	for _, k := range keys {
		e := reg.Tools[k]
		out = append(out, Candidate{Key: k, Path: e.Path, Shim: e.Shim, Members: e.Members, Tool: byKey[k]})
	}
	return out
}

// apply sets the status of each decided tool that still has the status it
// was offered with (from), so a decision survives another sync adding the
// tool meanwhile, but never overrides a status set in the meantime. It
// returns the keys of newly enabled installed tools, which are the ones to
// probe.
func apply(reg *registry.Registry, decisions map[string]Decision, from registry.Status) []string {
	keys := make([]string, 0, len(decisions))
	for k := range decisions {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var enabled []string
	for _, k := range keys {
		d := decisions[k]
		e := reg.Tools[k]
		if e == nil || e.Source != registry.Discovered || e.Status != from || d.Status == "" || d.Status == from {
			continue
		}
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
	paths := map[string]string{}
	for _, k := range keys {
		if e := reg.Tools[k]; e != nil && e.Path != "" {
			paths[k] = e.Path
		}
	}
	infos := probe.All(ctx, o.Env, paths, o.Probe, o.Out)
	return o.Store.Update(func(reg *registry.Registry) error {
		for k, info := range infos {
			if e := reg.Tools[k]; e != nil && e.Status == registry.Enabled {
				e.Version, e.Summary = info.Version, info.Summary
			}
		}
		return nil
	})
}

// printSummary counts newly found tools as new, and every tool that was up
// for a decision (offered, or found after the prompt opened) by outcome.
func printSummary(w io.Writer, newKeys, decided []string, decisions map[string]Decision, stale int) {
	count := map[registry.Status]int{}
	for _, k := range decided {
		s := decisions[k].Status
		if s == "" {
			s = registry.Pending
		}
		count[s]++
	}
	fmt.Fprintf(w, "sync: new: %d, enabled: %d, ignored: %d, pending: %d, stale: %d\n",
		len(newKeys), count[registry.Enabled], count[registry.Ignored], count[registry.Pending], stale)
}

func keysOf(cands []Candidate) []string {
	keys := make([]string, 0, len(cands))
	for _, c := range cands {
		keys = append(keys, c.Key)
	}
	return keys
}

// union returns a followed by the keys of b not already in a.
func union(a, b []string) []string {
	out := slices.Clone(a)
	for _, k := range b {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}
