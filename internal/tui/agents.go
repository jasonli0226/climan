package tui

import (
	"errors"

	"github.com/charmbracelet/huh"
)

const agentsTitle = "Install the climan skill into — space to toggle, enter to confirm"

// ErrInstallCancelled is returned when the user quits the agent picker.
var ErrInstallCancelled = errors.New("install cancelled; nothing written")

// PickAgents asks which agents to install into. Every agent starts checked.
func PickAgents(names []string) ([]string, error) {
	opts := make([]huh.Option[string], 0, len(names))
	for _, n := range names {
		opts = append(opts, huh.NewOption(n, n).Selected(true))
	}
	var selected []string
	field := huh.NewMultiSelect[string]().
		Title(agentsTitle).
		Options(opts...).
		Value(&selected)
	err := huh.NewForm(huh.NewGroup(field)).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, ErrInstallCancelled
	}
	return selected, err
}
