//go:build !windows

package probe

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jasonli0226/climan/internal/platform"
)

func testEnv() platform.Env {
	return platform.Env{GOOS: "linux", Home: os.Getenv("HOME"), Vars: map[string]string{
		"PATH": os.Getenv("PATH"), "HOME": os.Getenv("HOME"), "SECRET_TOKEN": "hunter2",
	}}
}

// script writes an executable shell script whose body handles $1.
func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProbeVersionAndSummary(t *testing.T) {
	p := script(t, `case "$1" in --version) echo "tool 1.2.3";; --help) printf '\n  Usage: tool [x]\nmore\n';; esac`)
	got := Probe(context.Background(), testEnv(), p)
	want := Info{Version: "tool 1.2.3", Summary: "Usage: tool [x]"}
	if got != want {
		t.Fatalf("Probe = %+v, want %+v", got, want)
	}
}

func TestProbeTimeout(t *testing.T) {
	p := script(t, "sleep 10")
	start := time.Now()
	got := Probe(context.Background(), testEnv(), p)
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("Probe took %v, want < 3s", d)
	}
	if got != (Info{}) {
		t.Fatalf("Probe = %+v, want empty", got)
	}
}

func TestProbeStdinClosed(t *testing.T) {
	p := script(t, `read x; echo "got:$x"`)
	start := time.Now()
	got := Probe(context.Background(), testEnv(), p)
	if time.Since(start) > time.Second || got.Version != "got:" {
		t.Fatalf("Probe = %+v after %v, want got: quickly", got, time.Since(start))
	}
}

func TestProbeMinimalEnv(t *testing.T) {
	t.Setenv("SECRET_TOKEN", "hunter2")
	p := script(t, `echo "secret=${SECRET_TOKEN:-none} color=$NO_COLOR term=$TERM"`)
	got := Probe(context.Background(), testEnv(), p)
	if got.Version != "secret=none color=1 term=dumb" {
		t.Fatalf("Version = %q, want secret=none color=1 term=dumb", got.Version)
	}
}

func TestProbeCwdIsTemp(t *testing.T) {
	cwd, _ := os.Getwd()
	p := script(t, "touch made-here; pwd")
	got := Probe(context.Background(), testEnv(), p)
	if _, err := os.Stat(filepath.Join(cwd, "made-here")); err == nil {
		os.Remove(filepath.Join(cwd, "made-here"))
		t.Fatal("probe wrote into the caller's working directory")
	}
	if got.Version == "" || got.Version == cwd {
		t.Fatalf("probe cwd = %q, want a fresh temp dir", got.Version)
	}
	if _, err := os.Stat(got.Version); err == nil {
		t.Fatalf("probe temp dir %s not removed", got.Version)
	}
}

func TestProbeStripsControlChars(t *testing.T) {
	p := script(t, `printf '\033[31mred\033[0m\a\n'`)
	if got := Probe(context.Background(), testEnv(), p); got.Version != "red" {
		t.Fatalf("Version = %q, want red", got.Version)
	}
}

func TestProbeOutputCap(t *testing.T) {
	p := script(t, `head -c 1048576 /dev/zero | tr '\0' 'a'`)
	got := Probe(context.Background(), testEnv(), p)
	if n := utf8.RuneCountInString(got.Version); n == 0 || n > 200 {
		t.Fatalf("Version has %d runes, want 1..200", n)
	}
}

func TestProbeNonZeroExitStillUsed(t *testing.T) {
	p := script(t, `case "$1" in --help) echo "usage: tool"; exit 1;; esac`)
	if got := Probe(context.Background(), testEnv(), p); got.Summary != "usage: tool" {
		t.Fatalf("Summary = %q, want usage: tool", got.Summary)
	}
}

func TestProbeEnvList(t *testing.T) {
	env := testEnv()
	got := strings.Join(probeEnv(env), " ")
	if strings.Contains(got, "SECRET_TOKEN") || !strings.Contains(got, "TERM=dumb") {
		t.Fatalf("probeEnv = %s", got)
	}
}

func TestProbeEnvDisablesPagers(t *testing.T) {
	got := strings.Join(probeEnv(testEnv()), " ")
	for _, want := range []string{"PAGER=cat", "MANPAGER=cat", "AWS_PAGER="} {
		if !strings.Contains(got, want) {
			t.Errorf("probeEnv = %s, missing %s", got, want)
		}
	}
}

func TestProbeSkipsErrorLine(t *testing.T) {
	p := script(t, `case "$1" in --help) printf 'error: missing command\nTool frobs widgets\n';; esac`)
	if got := Probe(context.Background(), testEnv(), p); got.Summary != "Tool frobs widgets" {
		t.Fatalf("Summary = %q, want Tool frobs widgets", got.Summary)
	}
}

func TestProbeDashHFallback(t *testing.T) {
	p := script(t, `case "$1" in --help) echo "tool: unknown option --help";; -h) echo "Tool does things well";; esac`)
	if got := Probe(context.Background(), testEnv(), p); got.Summary != "Tool does things well" {
		t.Fatalf("Summary = %q, want Tool does things well", got.Summary)
	}
}

func TestProbeHelpSubcommandWhenSuggested(t *testing.T) {
	p := script(t, `case "$1" in
--help|-h) printf 'tool: [ERROR]: the following arguments are required: command\n\nusage: tool <command>\nTo see help text, you can run:\n\n  tool help\n';;
help) printf '\033[1mNAME\033[0m\n       tool -\n\n\033[1mDESCRIPTION\033[0m\n       The Tool CLI is a unified tool\n       to manage widgets.\n\nSYNOPSIS\n       tool [options]\n';;
esac`)
	got := Probe(context.Background(), testEnv(), p)
	if want := "The Tool CLI is a unified tool to manage widgets."; got.Summary != want {
		t.Fatalf("Summary = %q, want %q", got.Summary, want)
	}
}

func TestProbeHelpSubcommandNotRunUnlessSuggested(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran-help")
	p := script(t, `case "$1" in --help|-h) echo "usage: tool <file>";; help) touch `+marker+`;; esac`)
	Probe(context.Background(), testEnv(), p)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("probe ran `tool help` although the tool never suggested it")
	}
}
