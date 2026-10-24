package syncer

import (
	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/registry"
	"github.com/jasonli0226/climan/internal/rules"
)

// Candidate is a newly found tool offered for a decision.
type Candidate struct {
	Key, Path, Shim string
	Members         []string
	Tool            discover.Tool
}

// Decision is the status (and extra tags) chosen for a candidate.
type Decision struct {
	Status registry.Status // may be Pending
	Tags   []string
}

// Chooser decides candidate statuses. Contract: Choose returns a Decision
// for every candidate key; Run treats a missing key as Pending.
type Chooser interface {
	Choose(cands []Candidate) (map[string]Decision, error)
}

// BatchChooser leaves every candidate pending (sync --batch).
type BatchChooser struct{}

func (BatchChooser) Choose(cands []Candidate) (map[string]Decision, error) {
	out := map[string]Decision{}
	for _, c := range cands {
		out[c.Key] = Decision{Status: registry.Pending}
	}
	return out, nil
}

// AutoChooser applies rules.yaml (sync --auto); unmatched tools stay pending.
type AutoChooser struct{ Rules rules.Rules }

func (a AutoChooser) Choose(cands []Candidate) (map[string]Decision, error) {
	out := map[string]Decision{}
	for _, c := range cands {
		d := Decision{Status: registry.Pending}
		if rule, ok := a.Rules.Match(c.Tool); ok {
			d = Decision{Status: registry.Ignored, Tags: rule.Tags}
			if rule.Action == rules.ActionEnable {
				d.Status = registry.Enabled
			}
		}
		out[c.Key] = d
	}
	return out, nil
}
