// Package probe runs a tool's --version and --help in a restricted
// environment and keeps the first useful line of each. Probe output is
// untrusted text: it is cleaned and stored, never interpreted.
package probe

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jasonli0226/climan/internal/platform"
)

const (
	timeout   = 2 * time.Second
	waitDelay = 500 * time.Millisecond
	outputCap = 64 << 10
	lineCap   = 200
	helpLines = 80
)

// Info is what a probe learned about a tool.
type Info struct{ Version, Summary string }

// Probe runs `path --version` then `path --help`. If --version times out,
// --help is skipped.
func Probe(ctx context.Context, env platform.Env, path string) Info {
	version, timedOut := run(ctx, env, path, "--version")
	if timedOut {
		return Info{}
	}
	summary, _ := run(ctx, env, path, "--help")
	return Info{Version: firstLine(version), Summary: firstLine(summary)}
}

// Help returns `path --help` output for a skill file, cleaned to plain text:
// escape sequences removed, tabs expanded, other control characters dropped,
// lines capped at lineCap runes and the whole at helpLines lines.
func Help(ctx context.Context, env platform.Env, path string) string {
	out, timedOut := run(ctx, env, path, "--help")
	if timedOut {
		return ""
	}
	return cleanHelp(out)
}

// run executes path with one argument and returns its output, capped at
// outputCap. The exit status is ignored: many tools exit non-zero after
// printing help.
func run(ctx context.Context, env platform.Env, path, arg string) (out []byte, timedOut bool) {
	dir, err := os.MkdirTemp("", "climan-probe-")
	if err != nil {
		return nil, false
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, arg)
	cmd.Dir = dir
	cmd.Env = probeEnv(env)
	cmd.Stdin = nil // the null device
	buf := &capped{}
	cmd.Stdout, cmd.Stderr = buf, buf
	cmd.WaitDelay = waitDelay
	cmd.SysProcAttr = sysProcAttr()

	_ = cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, true
	}
	return buf.buf.Bytes(), false
}

// probeEnv is the minimal environment a probed tool sees.
func probeEnv(env platform.Env) []string {
	keep := []string{"PATH", "HOME", "USERPROFILE", "LANG"}
	if env.GOOS == "windows" {
		keep = append(keep, "SystemRoot")
	}
	var out []string
	for _, k := range keep {
		if v, ok := env.Vars[k]; ok {
			out = append(out, k+"="+v)
		}
	}
	return append(out, "NO_COLOR=1", "TERM=dumb")
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b[@-_]`)

// firstLine returns the first non-blank line of out with escape sequences
// and control characters removed, trimmed to lineCap runes.
func firstLine(out []byte) string {
	text := ansiEscape.ReplaceAllString(string(out), "")
	for _, line := range strings.Split(text, "\n") {
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if r := []rune(line); len(r) > lineCap {
			line = string(r[:lineCap])
		}
		return line
	}
	return ""
}

// cleanHelp turns raw help output into at most helpLines plain lines.
func cleanHelp(out []byte) string {
	text := ansiEscape.ReplaceAllString(string(out), "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.ReplaceAll(line, "\t", "    ")
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line)
		if r := []rune(line); len(r) > lineCap {
			line = string(r[:lineCap])
		}
		lines = append(lines, strings.TrimRight(line, " "))
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	if len(lines) > helpLines {
		lines = lines[:helpLines]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// capped keeps the first outputCap bytes and silently drops the rest.
type capped struct{ buf bytes.Buffer }

func (c *capped) Write(p []byte) (int, error) {
	if room := outputCap - c.buf.Len(); room > 0 {
		if len(p) > room {
			c.buf.Write(p[:room])
		} else {
			c.buf.Write(p)
		}
	}
	return len(p), nil
}
