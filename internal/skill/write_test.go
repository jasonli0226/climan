package skill

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasonli0226/climan/internal/platform"
	"github.com/jasonli0226/climan/internal/registry"
)

type helpRecorder struct{ calls []string }

func (h *helpRecorder) help(_ context.Context, _ platform.Env, path string) string {
	h.calls = append(h.calls, path)
	return "usage: tool"
}

func setup(t *testing.T) (*registry.Registry, string, *helpRecorder, *bytes.Buffer, Options) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "rg")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := registry.New()
	reg.LastSync = time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	reg.Tools["rg"] = &registry.Entry{
		Status: registry.Enabled, Source: registry.Discovered, Path: bin,
		LastSeen: reg.LastSync, Tags: []string{"search"}, Desc: "fast grep",
	}
	h := &helpRecorder{}
	var out bytes.Buffer
	return reg, t.TempDir(), h, &out, Options{Env: platform.Env{GOOS: "linux"}, Help: h.help, Out: &out}
}

func TestWriteLayout(t *testing.T) {
	reg, outDir, _, out, o := setup(t)
	if err := Write(context.Background(), o, reg, []string{"rg"}, outDir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(outDir, "search", "rg", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), Marker) {
		t.Fatalf("read %s: %v\n%s", path, err, data)
	}
	if out.String() != "wrote: "+path+"\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestWriteOverwritesOwn(t *testing.T) {
	reg, outDir, _, _, o := setup(t)
	if err := Write(context.Background(), o, reg, []string{"rg"}, outDir); err != nil {
		t.Fatal(err)
	}
	reg.Tools["rg"].Desc = "even faster grep"
	if err := Write(context.Background(), o, reg, []string{"rg"}, outDir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(outDir, "search", "rg", "SKILL.md"))
	if !strings.Contains(string(data), "even faster grep") {
		t.Fatalf("not overwritten:\n%s", data)
	}
}

func TestWriteSkipsHandWritten(t *testing.T) {
	reg, outDir, _, out, o := setup(t)
	path := filepath.Join(outDir, "search", "rg", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("my own skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(context.Background(), o, reg, []string{"rg"}, outDir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "my own skill\n" || out.String() != "skipped: "+path+" (not written by climan)\n" {
		t.Fatalf("file %q output %q", data, out.String())
	}
}

func TestWriteHelpOnlyWhenPresent(t *testing.T) {
	reg, outDir, h, _, o := setup(t)
	reg.Tools["alias://gs"] = &registry.Entry{Status: registry.Enabled, Source: registry.Registered, Desc: "git status"}
	reg.Tools["old"] = &registry.Entry{
		Status: registry.Enabled, Source: registry.Discovered, Path: "/gone/old",
		LastSeen: reg.LastSync.Add(-time.Hour),
	}
	if err := Write(context.Background(), o, reg, []string{"rg", "alias://gs", "old"}, outDir); err != nil {
		t.Fatal(err)
	}
	if len(h.calls) != 1 || h.calls[0] != reg.Tools["rg"].Path {
		t.Fatalf("help calls = %v, want only rg", h.calls)
	}
}
