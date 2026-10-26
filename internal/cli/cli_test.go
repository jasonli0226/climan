package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasonli0226/climan/internal/registry"
)

// sandbox points PATH and the config dir at temp dirs holding one tool.
func sandbox(t *testing.T) (configDir string) {
	t.Helper()
	bin := t.TempDir()
	name := "mytool"
	if os.PathSeparator == '\\' {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	configDir = filepath.Join(t.TempDir(), "climan")
	t.Setenv("PATH", bin)
	t.Setenv("CLIMAN_CONFIG_DIR", configDir)
	return configDir
}

func pipeStdin(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return r
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(args, pipeStdin(t), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestSyncFlagsExclusive(t *testing.T) {
	sandbox(t)
	code, _, stderr := run(t, "sync", "--batch", "--auto")
	if code != 2 || !strings.Contains(stderr, "--batch") || !strings.Contains(stderr, "--auto") {
		t.Fatalf("code=%d stderr=%q, want 2 naming both flags", code, stderr)
	}
}

func TestNoInputAlias(t *testing.T) {
	dir := sandbox(t)
	code, stdout, stderr := run(t, "sync", "--no-input")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "new: 1") {
		t.Fatalf("stdout = %q, want new: 1", stdout)
	}
	reg, err := registry.Store{Dir: dir}.Load()
	if err != nil {
		t.Fatal(err)
	}
	if e := reg.Tools["mytool"]; e == nil || e.Status != registry.Pending {
		t.Fatalf("mytool = %+v, want pending", e)
	}
}

func TestAutoWithoutRulesFile(t *testing.T) {
	sandbox(t)
	code, _, stderr := run(t, "sync", "--auto")
	if code != 1 || !strings.Contains(stderr, "no rules file at") {
		t.Fatalf("code=%d stderr=%q, want 1 with missing rules hint", code, stderr)
	}
}

func TestNonTTYWithoutFlag(t *testing.T) {
	sandbox(t)
	code, _, stderr := run(t, "sync")
	if code != 1 || !strings.Contains(stderr, "stdin is not a terminal; use --batch or --auto") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestUnknownCommand(t *testing.T) {
	code, _, stderr := run(t, "frobnicate")
	if code != 2 || !strings.Contains(stderr, "usage: climan") {
		t.Fatalf("code=%d stderr=%q, want 2 with usage", code, stderr)
	}
}

func TestNoCommand(t *testing.T) {
	code, _, stderr := run(t)
	if code != 2 || !strings.Contains(stderr, "usage: climan") {
		t.Fatalf("code=%d stderr=%q, want 2 with usage", code, stderr)
	}
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	code, stdout, stderr := run(t, args...)
	if code != 0 {
		t.Fatalf("%v: code=%d stderr=%q", args, code, stderr)
	}
	return stdout
}

func TestContextCommand(t *testing.T) {
	sandbox(t)
	mustRun(t, "sync", "--batch")
	mustRun(t, "enable", "mytool", "--desc", "does x")
	if out := mustRun(t, "context"); !strings.HasSuffix(out, "mytool: does x\n") {
		t.Fatalf("context = %q", out)
	}
}

func TestContextNoRegistry(t *testing.T) {
	sandbox(t)
	if out := mustRun(t, "context"); !strings.Contains(out, "No tools enabled yet") {
		t.Fatalf("context = %q", out)
	}
}

func TestListNoRegistry(t *testing.T) {
	sandbox(t)
	if out := mustRun(t, "list"); out != "" {
		t.Fatalf("list = %q, want empty", out)
	}
}

func TestListCommand(t *testing.T) {
	sandbox(t)
	mustRun(t, "sync", "--batch")
	if out := mustRun(t, "list", "--pending"); !strings.HasPrefix(out, "mytool") {
		t.Fatalf("list --pending = %q", out)
	}
}

func TestListFlagsExclusive(t *testing.T) {
	sandbox(t)
	code, _, stderr := run(t, "list", "--all", "--stale")
	if code != 2 || !strings.Contains(stderr, "--all") || !strings.Contains(stderr, "--stale") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestEnableFlagAfterName(t *testing.T) {
	dir := sandbox(t)
	mustRun(t, "enable", "mytool", "--desc", "x")
	reg, err := registry.Store{Dir: dir}.Load()
	if err != nil || reg.Tools["mytool"].Desc != "x" {
		t.Fatalf("mytool = %+v, %v", reg.Tools["mytool"], err)
	}
}

func TestDescNeedsOneName(t *testing.T) {
	sandbox(t)
	if code, _, _ := run(t, "enable", "a", "b", "--desc", "x"); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestIgnoreRejectsDesc(t *testing.T) {
	sandbox(t)
	if code, _, _ := run(t, "ignore", "mytool", "--desc", "x"); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestEnableUnknown(t *testing.T) {
	sandbox(t)
	code, _, stderr := run(t, "enable", "nope")
	if code != 1 || !strings.Contains(stderr, "unknown tool: nope") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestEnableNoNames(t *testing.T) {
	sandbox(t)
	code, _, stderr := run(t, "enable")
	if code != 2 || !strings.Contains(stderr, "usage: climan") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestRegisterCommand(t *testing.T) {
	dir := sandbox(t)
	out := mustRun(t, "register", "alias://gs", "--desc", "git status", "--tags", "git, vcs,")
	if out != "registered: alias://gs\n" {
		t.Fatalf("stdout = %q", out)
	}
	reg, _ := registry.Store{Dir: dir}.Load()
	if e := reg.Tools["alias://gs"]; e == nil || strings.Join(e.Tags, " ") != "git vcs" {
		t.Fatalf("entry = %+v", e)
	}
}

func TestRegisterBadID(t *testing.T) {
	sandbox(t)
	if code, _, _ := run(t, "register", "gs"); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestDescribeCommand(t *testing.T) {
	sandbox(t)
	mustRun(t, "sync", "--batch")
	if out := mustRun(t, "describe", "mytool"); !strings.HasPrefix(out, "mytool\n") {
		t.Fatalf("describe = %q", out)
	}
}

func TestDescribeUnknownCommand(t *testing.T) {
	sandbox(t)
	code, _, stderr := run(t, "describe", "nope")
	if code != 1 || !strings.Contains(stderr, "unknown tool: nope") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestDescribeNeedsOneName(t *testing.T) {
	sandbox(t)
	for _, args := range [][]string{{"describe"}, {"describe", "a", "b"}} {
		if code, _, _ := run(t, args...); code != 2 {
			t.Errorf("%v: code = %d, want 2", args, code)
		}
	}
}
