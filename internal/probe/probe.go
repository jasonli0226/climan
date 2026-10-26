// Package probe runs a tool's --version and help commands in a restricted
// environment and keeps one useful line of each. Probe output is untrusted
// text: it is cleaned and stored, and only searched for a `<name> help` hint.
package probe

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
)

// Info is what a probe learned about a tool.
type Info struct{ Version, Summary string }

// Probe runs `path --version` then `path --help`. If --version times out,
// nothing else runs. While the summary is still weak it tries `path -h`,
// then `path help`, but only when the tool's own output suggested
// `<name> help`: a bare word is not safe to pass to an arbitrary tool.
func Probe(ctx context.Context, env platform.Env, path string) Info {
	out, timedOut := run(ctx, env, path, "--version")
	if timedOut {
		return Info{}
	}
	version := firstLine(out)
	help, timedOut := run(ctx, env, path, "--help")
	summary := summarize(help)
	if timedOut || !Weak(summary, version) {
		return Info{Version: version, Summary: summary}
	}
	short, _ := run(ctx, env, path, "-h")
	if s := summarize(short); !Weak(s, version) {
		return Info{Version: version, Summary: s}
	}
	if suggestsHelp(toolName(env, path), help, short) {
		long, _ := run(ctx, env, path, "help")
		if s := summarize(long); !Weak(s, version) {
			return Info{Version: version, Summary: s}
		}
	}
	return Info{Version: version, Summary: summary}
}

// suggestsHelp reports whether any output tells the user to run
// `<name> help`, as aws does.
func suggestsHelp(name string, outs ...[]byte) bool {
	re := regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(name) + ` help(\s|$)`)
	for _, out := range outs {
		if re.Match(out) {
			return true
		}
	}
	return false
}

// toolName is the command name a tool's help text would use for itself.
func toolName(env platform.Env, path string) string {
	name := filepath.Base(path)
	if env.GOOS == "windows" {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return name
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
	// Pagers would wait on a terminal; `aws help` starts one by default.
	return append(out, "NO_COLOR=1", "TERM=dumb", "PAGER=cat", "MANPAGER=cat", "AWS_PAGER=")
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b[@-_]`)

// firstLine returns the first non-blank line of out with escape sequences
// and control characters removed, trimmed to lineCap runes.
func firstLine(out []byte) string {
	for _, line := range cleanLines(out) {
		if line != "" {
			return capLine(line)
		}
	}
	return ""
}

// summarize picks the line of help output that best describes the tool:
// the paragraph under a man-page DESCRIPTION heading, else the first line
// that is not an error message, else the first line.
func summarize(out []byte) string {
	lines := cleanLines(out)
	if d := descriptionSection(lines); d != "" {
		return capLine(d)
	}
	for _, line := range lines {
		if line != "" && !isError(line) {
			return capLine(line)
		}
	}
	return firstLine(out)
}

// descriptionSection joins the lines of the first paragraph after a
// DESCRIPTION heading, as `aws help` and man pages print it.
func descriptionSection(lines []string) string {
	for i, line := range lines {
		if line != "DESCRIPTION" {
			continue
		}
		var para []string
		for _, next := range lines[i+1:] {
			if next == "" && len(para) > 0 {
				break
			}
			if next != "" {
				para = append(para, next)
			}
		}
		return strings.Join(strings.Fields(strings.Join(para, " ")), " ")
	}
	return ""
}

// cleanLines splits out into lines with escape sequences and control
// characters removed and surrounding space trimmed.
func cleanLines(out []byte) []string {
	text := ansiEscape.ReplaceAllString(string(out), "")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line))
	}
	return lines
}

func capLine(line string) string {
	if r := []rune(line); len(r) > lineCap {
		return string(r[:lineCap])
	}
	return line
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
