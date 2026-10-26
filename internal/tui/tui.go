// Package tui is the interactive picker for `climan sync`.
package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"

	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/syncer"
)

const (
	title       = "New tools found — space to toggle, a for all, enter to confirm"
	reviewTitle = "Ignored tools — check to enable, enter to confirm (unchecked stay ignored)"
)

// ErrCancelled is returned when the user quits the picker.
var ErrCancelled = errors.New("sync cancelled; nothing changed")

// HuhChooser asks the user which new tools to enable. Every tool starts
// unchecked; checked tools become enabled and the rest ignored. With Review
// set, the candidates are already ignored and the title says so.
type HuhChooser struct{ Review bool }

func (h HuhChooser) Choose(cands []syncer.Candidate) (map[string]syncer.Decision, error) {
	opts := make([]huh.Option[string], 0, len(cands))
	for _, c := range cands {
		opts = append(opts, huh.NewOption(label(c), c.Key))
	}
	var selected []string
	field := huh.NewMultiSelect[string]().
		Title(h.title()).
		Options(opts...).
		Value(&selected).
		Height(min(len(cands)+2, 20))

	km := huh.NewDefaultKeyMap()
	km.MultiSelect.SelectAll = key.NewBinding(key.WithKeys("a", "ctrl+a"), key.WithHelp("a", "select all"))
	km.MultiSelect.SelectNone = key.NewBinding(key.WithKeys("a", "ctrl+a"), key.WithHelp("a", "select none"), key.WithDisabled())

	err := huh.NewForm(huh.NewGroup(field)).WithKeyMap(km).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, ErrCancelled
	}
	if err != nil {
		return nil, err
	}
	return decisions(cands, selected), nil
}

func (h HuhChooser) title() string {
	if h.Review {
		return reviewTitle
	}
	return title
}

// label is one picker row: key (with family size), shim kind, path.
func label(c syncer.Candidate) string {
	parts := []string{c.Key}
	if len(c.Members) > 1 {
		parts[0] = fmt.Sprintf("%s (%d names)", c.Key, len(c.Members))
	}
	if c.Shim != "" {
		parts = append(parts, c.Shim)
	}
	return strings.Join(append(parts, c.Path), "  ")
}

// decisions enables the selected keys and ignores every other candidate.
func decisions(cands []syncer.Candidate, selected []string) map[string]syncer.Decision {
	picked := map[string]bool{}
	for _, k := range selected {
		picked[k] = true
	}
	out := make(map[string]syncer.Decision, len(cands))
	for _, c := range cands {
		status := registry.Ignored
		if picked[c.Key] {
			status = registry.Enabled
		}
		out[c.Key] = syncer.Decision{Status: status}
	}
	return out
}
