// Package cli parses climan's command line.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/probe"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/rules"
	"github.com/jasonli0226/climan/internal/syncer"
	"github.com/jasonli0226/climan/internal/tui"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2

	usage = `usage: climan <command> [flags]

commands:
  sync [--batch | --no-input | --auto]   scan PATH and pick new tools
`
)

// newInteractive builds the TUI chooser.
var newInteractive = func() syncer.Chooser { return tui.HuhChooser{} }

// Run executes climan with args (without the program name) and returns the
// process exit code.
func Run(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "sync":
		return runSync(args[1:], stdin, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "climan: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

func runSync(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var batch, auto bool
	fs.BoolVar(&batch, "batch", false, "do not prompt; new tools stay pending")
	fs.BoolVar(&batch, "no-input", false, "alias for --batch")
	fs.BoolVar(&auto, "auto", false, "do not prompt; apply rules.yaml")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if batch && auto {
		fmt.Fprintln(stderr, "climan sync: --batch and --auto cannot be used together")
		return exitUsage
	}

	env := platform.Current()
	dir, err := env.ConfigDir()
	if err != nil {
		return fail(stderr, err)
	}
	chooser, err := pickChooser(env, dir, batch, auto, stdin)
	if err != nil {
		return fail(stderr, err)
	}
	err = syncer.Run(context.Background(), syncer.Options{
		Env:     env,
		Store:   registry.Store{Dir: dir},
		Chooser: chooser,
		Now:     func() time.Time { return time.Now().UTC().Truncate(time.Second) },
		Probe:   probe.Probe,
		Out:     stdout,
	})
	if err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

func pickChooser(env platform.Env, dir string, batch, auto bool, stdin *os.File) (syncer.Chooser, error) {
	switch {
	case batch:
		return syncer.BatchChooser{}, nil
	case auto:
		r, err := rules.Load(filepath.Join(dir, "rules.yaml"), env)
		if err != nil {
			return nil, err
		}
		return syncer.AutoChooser{Rules: r}, nil
	case !term.IsTerminal(int(stdin.Fd())):
		return nil, errors.New("stdin is not a terminal; use --batch or --auto")
	default:
		return newInteractive(), nil
	}
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "climan: %v\n", err)
	return exitError
}
