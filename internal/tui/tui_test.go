package tui

import (
	"reflect"
	"testing"

	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/syncer"
)

func TestLabel(t *testing.T) {
	family := syncer.Candidate{
		Key: "python", Members: []string{"python", "python3", "python3.12"},
		Shim: "pyenv", Path: "/h/.pyenv/shims/python",
	}
	if got, want := label(family), "python (3 names)  pyenv  /h/.pyenv/shims/python"; got != want {
		t.Errorf("label = %q, want %q", got, want)
	}
	plain := syncer.Candidate{Key: "rg", Members: []string{"rg"}, Path: "/h/.cargo/bin/rg"}
	if got, want := label(plain), "rg  /h/.cargo/bin/rg"; got != want {
		t.Errorf("label = %q, want %q", got, want)
	}
}

func TestDecisions(t *testing.T) {
	cands := []syncer.Candidate{{Key: "a"}, {Key: "b"}}
	got := decisions(cands, []string{"a"})
	want := map[string]syncer.Decision{
		"a": {Status: registry.Enabled},
		"b": {Status: registry.Ignored},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %v, want %v", got, want)
	}
}
