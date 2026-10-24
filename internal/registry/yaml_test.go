package registry

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRoundTripPreservesComments(t *testing.T) {
	data, err := os.ReadFile("testdata/commented.yaml")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := Parse("testdata/commented.yaml", data)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Tools["rg"].Notes != "use -uu for hidden files" {
		t.Fatalf("rg notes = %q", reg.Tools["rg"].Notes)
	}
	reg.Tools["rg"].Status = Ignored
	reg.Tools["zoxide"] = &Entry{Status: Pending, Source: Discovered, Path: "/home/u/.local/bin/zoxide"}

	out, err := reg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"# climan registry — hand-edited",
		"# ripgrep, my main search tool",
		"# keep this",
		"homepage: https://github.com/BurntSushi/ripgrep",
		"status: ignored",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	rg, aws, zox := strings.Index(s, "  rg:"), strings.Index(s, "  aws:"), strings.Index(s, "  zoxide:")
	if !(rg >= 0 && rg < aws && aws < zox) {
		t.Errorf("tool order rg=%d aws=%d zoxide=%d, want rg < aws < zoxide:\n%s", rg, aws, zox, s)
	}
}

func TestMarshalDropsEmptiedField(t *testing.T) {
	data, _ := os.ReadFile("testdata/commented.yaml")
	reg, err := Parse("x.yaml", data)
	if err != nil {
		t.Fatal(err)
	}
	reg.Tools["rg"].Notes = ""
	out, _ := reg.Marshal()
	if strings.Contains(string(out), "notes:") {
		t.Fatalf("emptied notes still written:\n%s", out)
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := Parse("/cfg/registry.yaml", []byte("tools: [unclosed\n"))
	if err == nil {
		t.Fatal("Parse succeeded on invalid YAML")
	}
	if !strings.Contains(err.Error(), "/cfg/registry.yaml") || !strings.Contains(err.Error(), "line") {
		t.Fatalf("error %q should name the file and line", err)
	}
}

func TestParseEmpty(t *testing.T) {
	reg, err := Parse("r.yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Tools) != 0 {
		t.Fatalf("Tools = %v, want empty", reg.Tools)
	}
	out, err := New().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "version: 1\n") {
		t.Fatalf("new registry starts %q, want version: 1", out)
	}
}

func TestParseNewerVersion(t *testing.T) {
	_, err := Parse("r.yaml", []byte("version: 2\ntools: {}\n"))
	if err == nil || !strings.Contains(err.Error(), "newer than this climan supports") {
		t.Fatalf("err = %v, want newer-version error", err)
	}
}

func TestMarshalRoundTripsValues(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	reg := New()
	reg.LastSync = now
	reg.Tools["python"] = &Entry{
		Status: Enabled, Source: Discovered, Tag: "installed", Path: "/p/python",
		Members: []string{"python", "python3"}, Shadowed: []string{"/usr/bin/python3"}, LastSeen: now,
	}
	out, err := reg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse("r.yaml", out)
	if err != nil {
		t.Fatal(err)
	}
	got := back.Tools["python"]
	if !back.LastSync.Equal(now) || got.Status != Enabled || len(got.Members) != 2 || !got.LastSeen.Equal(now) {
		t.Fatalf("round trip lost data: %+v, last_sync %v\n%s", got, back.LastSync, out)
	}
}

func TestStale(t *testing.T) {
	reg := New()
	reg.LastSync = time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	reg.Tools["old"] = &Entry{Source: Discovered, LastSeen: reg.LastSync.Add(-time.Hour)}
	reg.Tools["fresh"] = &Entry{Source: Discovered, LastSeen: reg.LastSync}
	reg.Tools["manual"] = &Entry{Source: Registered}
	if !reg.Stale("old") || reg.Stale("fresh") || reg.Stale("manual") {
		t.Fatalf("Stale old=%v fresh=%v manual=%v, want true false false",
			reg.Stale("old"), reg.Stale("fresh"), reg.Stale("manual"))
	}
}

func TestMarshalFlowToolsBecomesBlock(t *testing.T) {
	reg, err := Parse("r.yaml", []byte("version: 1\ntools: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	reg.Tools["rg"] = &Entry{Status: Pending, Source: Discovered}
	out, _ := reg.Marshal()
	if !strings.Contains(string(out), "\n  rg:\n") {
		t.Fatalf("want block-style tools mapping:\n%s", out)
	}
}

func TestParseRejectsBadEntries(t *testing.T) {
	tests := []struct{ name, data, want string }{
		{"empty entry", "tools:\n  rg:\n", `tool "rg"`},
		{"bad status", "tools:\n  rg:\n    status: enabeld\n    source: discovered\n", "enabeld"},
		{"bad source", "tools:\n  rg:\n    status: enabled\n    source: manual\n", "manual"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("/cfg/registry.yaml", []byte(tt.data))
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "/cfg/registry.yaml") {
				t.Fatalf("err = %v, want mention of %q and the file", err, tt.want)
			}
		})
	}
}
