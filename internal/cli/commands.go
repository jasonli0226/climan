package cli

import (
	"context"
	"flag"
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

func now() time.Time { return time.Now().UTC().Truncate(time.Second) }

// load returns the env, the store and the current registry.
func load() (platform.Env, registry.Store, *registry.Registry, error) {
	env := platform.Current()
	dir, err := env.ConfigDir()
	if err != nil {
		return env, registry.Store{}, nil, err
	}
	store := registry.Store{Dir: dir}
	reg, err := store.Load()
	return env, store, reg, err
}

// parseInterleaved parses flags that may come before, between or after
// positional args; stdlib flag stops at the first positional.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var names []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return names, nil
		}
		names = append(names, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func runContext(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "climan context: takes no arguments\n\n%s", usage)
		return exitUsage
	}
	env, _, reg, err := load()
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprint(stdout, view.Context(env, reg))
	return exitOK
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	modes := []struct {
		name string
		mode view.Mode
		set  *bool
	}{
		{name: "all", mode: view.All}, {name: "pending", mode: view.Pending},
		{name: "ignored", mode: view.Ignored}, {name: "stale", mode: view.Stale},
	}
	for i := range modes {
		modes[i].set = fs.Bool(modes[i].name, false, "show "+modes[i].name+" tools")
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	mode := view.Enabled
	var chosen []string
	for _, m := range modes {
		if *m.set {
			mode = m.mode
			chosen = append(chosen, "--"+m.name)
		}
	}
	if len(chosen) > 1 || fs.NArg() > 0 {
		fmt.Fprintf(stderr, "climan list: use at most one of --all, --pending, --ignored, --stale (got %s)\n",
			strings.Join(append(chosen, fs.Args()...), " "))
		return exitUsage
	}

	env, _, reg, err := load()
	if err != nil {
		return fail(stderr, err)
	}
	var core []discover.Tool
	if mode == view.All {
		core = discover.Group(discover.Scan(env))
	}
	fmt.Fprint(stdout, view.List(env, reg, mode, core))
	return exitOK
}

// descFlag records whether --desc was given, so --desc "" can clear.
type descFlag struct {
	value string
	set   bool
}

func (d *descFlag) String() string     { return d.value }
func (d *descFlag) Set(s string) error { d.value, d.set = s, true; return nil }

func runStatus(cmd string, to registry.Status, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var desc descFlag
	if to == registry.Enabled {
		fs.Var(&desc, "desc", "your own one-line description (enable only, one tool)")
	}
	names, err := parseInterleaved(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(names) == 0 {
		fmt.Fprintf(stderr, "climan %s: name at least one tool\n\n%s", cmd, usage)
		return exitUsage
	}
	if desc.set && len(names) != 1 {
		fmt.Fprintf(stderr, "climan %s: --desc needs exactly one tool, got %d\n", cmd, len(names))
		return exitUsage
	}

	env := platform.Current()
	dir, err := env.ConfigDir()
	if err != nil {
		return fail(stderr, err)
	}
	var d *string
	if desc.set {
		d = &desc.value
	}
	err = status.Set(context.Background(), status.Options{
		Env: env, Store: registry.Store{Dir: dir}, Now: now, Probe: probe.Probe, Out: stdout,
	}, names, to, d)
	if err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
