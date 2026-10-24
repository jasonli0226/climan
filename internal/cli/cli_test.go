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
