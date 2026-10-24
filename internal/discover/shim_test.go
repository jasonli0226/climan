package discover

import (
	"path/filepath"
	"strings"
	"testing"
)

const pyenvShim = "#!/usr/bin/env bash\nset -e\nexec \"/home/u/.pyenv/libexec/pyenv\" exec \"$program\" \"$@\"\n"

func TestShimPyenv(t *testing.T) {
	p := writeFile(t, t.TempDir(), "python", pyenvShim, 0o755)
	if got := DetectShim(hostEnv(), p); got != "pyenv" {
		t.Fatalf("DetectShim = %q, want pyenv", got)
	}
}

func TestShimLargeScriptIgnored(t *testing.T) {
	big := pyenvShim + strings.Repeat("# padding\n", 512)
	p := writeFile(t, t.TempDir(), "tool", big, 0o755)
	if got := DetectShim(hostEnv(), p); got != "" {
		t.Fatalf("DetectShim = %q, want \"\" for a 5 KiB script", got)
	}
}

func TestShimNotScript(t *testing.T) {
	p := writeFile(t, t.TempDir(), "bin", "\x7fELF\x02\x01\x01pyenv", 0o755)
	if got := DetectShim(hostEnv(), p); got != "" {
		t.Fatalf("DetectShim = %q, want \"\" for a binary", got)
	}
}

func TestShimDirName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shims")
	p := writeFile(t, dir, "node", "#!/bin/sh\nexec real-node \"$@\"\n", 0o755)
	if got := DetectShim(hostEnv(), p); got != "shims-dir" {
		t.Fatalf("DetectShim = %q, want shims-dir", got)
	}
}

func TestShimScoop(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "rg.exe", "MZ", 0o644)
	writeFile(t, dir, "rg.shim", "path = \"C:\\Users\\u\\scoop\\apps\\rg\\current\\rg.exe\"\nargs = \n", 0o644)
	want := `scoop:C:\Users\u\scoop\apps\rg\current\rg.exe`
	if got := DetectShim(windowsEnv(), p); got != want {
		t.Fatalf("DetectShim = %q, want %q", got, want)
	}
}

func TestShimNpmCmd(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "npm")
	p := writeFile(t, dir, "tsc.cmd", "@ECHO off\r\n", 0o644)
	if got := DetectShim(windowsEnv(), p); got != "npm-cmd" {
		t.Fatalf("DetectShim = %q, want npm-cmd", got)
	}
}

func TestScanSetsShim(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, exeName("python"), pyenvShim, 0o755)
	found := Scan(hostEnv(dir))
	if len(found) != 1 || found[0].Shim != "pyenv" {
		t.Fatalf("found = %+v, want one entry with Shim pyenv", found)
	}
}
