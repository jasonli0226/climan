// Package registry holds the YAML file that records every known tool.
package registry

import (
	"time"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the registry file format this climan writes.
const SchemaVersion = 1

// Status is the user's decision about a tool.
type Status string

const (
	Pending Status = "pending"
	Enabled Status = "enabled"
	Ignored Status = "ignored"
)

// Source says how an entry was created.
type Source string

const (
	Discovered Source = "discovered"
	Registered Source = "registered"
)

// Entry is one tool. Every field is owned by climan; keys in the file that
// are not fields here are preserved untouched.
type Entry struct {
	Status   Status    `yaml:"status"`
	Source   Source    `yaml:"source"`
	Tag      string    `yaml:"tag,omitempty"`
	Path     string    `yaml:"path,omitempty"`
	Members  []string  `yaml:"members,omitempty"`
	Shim     string    `yaml:"shim,omitempty"`
	Shadowed []string  `yaml:"shadowed,omitempty"`
	LastSeen time.Time `yaml:"last_seen,omitempty"`
	Version  string    `yaml:"version,omitempty"`
	Summary  string    `yaml:"summary,omitempty"`
	Desc     string    `yaml:"desc,omitempty"` // written by the user only
	Tags     []string  `yaml:"tags,omitempty"`
	Notes    string    `yaml:"notes,omitempty"`
}

// Registry is the parsed registry file.
type Registry struct {
	Version  int // SchemaVersion on write
	LastSync time.Time
	Tools    map[string]*Entry
	doc      *yaml.Node // original document; nil for a new registry
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{Version: SchemaVersion, Tools: map[string]*Entry{}}
}

// Stale reports whether a discovered tool was missing from the latest sync.
func (r *Registry) Stale(key string) bool {
	e, ok := r.Tools[key]
	return ok && e.Source == Discovered && e.LastSeen.Before(r.LastSync)
}
