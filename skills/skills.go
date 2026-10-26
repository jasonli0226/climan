// Package skills holds the agent skills climan ships, built into the binary
// so `climan install` works without a checkout.
package skills

import _ "embed"

// Climan is skills/climan/SKILL.md.
//
//go:embed climan/SKILL.md
var Climan []byte
