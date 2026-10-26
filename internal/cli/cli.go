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
  sync [--batch | --no-input | --auto]           scan PATH and pick new tools
  list [--all | --pending | --ignored | --stale] show tools (enabled by default)
  list --unreviewed                              enabled tools with no desc yet
  enable <tool>... [--desc "text"]               use tools; --desc sets your description
  ignore <tool>...                               stop offering tools
  register <id> [--desc "text"] [--tags a,b]     add file://, alias:// or mcp:// by hand
  describe <tool> [--probe]                      show everything known about a tool
  context                                        print the tool list for agents
  install [--agent claude,pi,opencode]           put the climan skill where agents load it
  version                                        print climan's version
`
)

// newInteractive builds the TUI chooser.
var newInteractive = func() syncer.Chooser { return tui.HuhChooser{} }

// pickAgents asks which detected agents to install into.
var pickAgents = tui.PickAgents

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
	case "list":
		return runList(args[1:], stdout, stderr)
	case "enable":
		return runStatus("enable", registry.Enabled, args[1:], stdout, stderr)
	case "ignore":
		return runStatus("ignore", registry.Ignored, args[1:], stdout, stderr)
	case "register":
		return runRegister(args[1:], stdout, stderr)
	case "describe":
		return runDescribe(args[1:], stdout, stderr)
	case "context":
		return runContext(args[1:], stdout, stderr)
	case "install":
		return runInstall(args[1:], stdin, stdout, stderr)
	case "version", "--version", "-v":
		return runVersion(stdout)
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
		Now:     now,
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
