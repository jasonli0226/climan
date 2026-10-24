// Package rules reads rules.yaml, which decides tool statuses for
// `sync --auto`.
package rules

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/platform"
)

const (
	ActionEnable = "enable"
	ActionIgnore = "ignore"
)

// Rule matches tools by install dir, name glob, or both.
type Rule struct {
	Dir    string   `yaml:"dir"`
	Name   string   `yaml:"name"`
	Action string   `yaml:"action"`
	Tags   []string `yaml:"tags"`
}

// Rules is an ordered rule list; the first match wins.
type Rules struct {
	Rules []Rule `yaml:"rules"`
	env   platform.Env
}

// Load reads and validates the rules file at p.
func Load(p string, env platform.Env) (Rules, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Rules{}, fmt.Errorf("no rules file at %s; create it or use --batch", p)
	}
	if err != nil {
		return Rules{}, err
	}
	var r Rules
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Rules{}, fmt.Errorf("%s: %w", p, err)
	}
	for i, rule := range r.Rules {
		if err := validate(rule); err != nil {
			return Rules{}, fmt.Errorf("%s: rule %d: %w", p, i+1, err)
		}
	}
	r.env = env
	return r, nil
}

func validate(r Rule) error {
	if r.Dir == "" && r.Name == "" {
		return errors.New("needs dir, name, or both")
	}
	if r.Action != ActionEnable && r.Action != ActionIgnore {
		return fmt.Errorf("action %q must be %q or %q", r.Action, ActionEnable, ActionIgnore)
	}
	if r.Name != "" {
		if _, err := path.Match(r.Name, ""); err != nil {
			return fmt.Errorf("name %q: %w", r.Name, err)
		}
	}
	return nil
}

// Match returns the first rule matching t.
func (r Rules) Match(t discover.Tool) (Rule, bool) {
	dir := ""
	if len(t.Members) > 0 {
		dir = r.normDir(t.Members[0].Dir)
	}
	for _, rule := range r.Rules {
		if rule.Dir != "" && r.normDir(r.expand(rule.Dir)) != dir {
			continue
		}
		if rule.Name != "" {
			if ok, _ := path.Match(rule.Name, t.Key); !ok {
				continue
			}
		}
		return rule, true
	}
	return Rule{}, false
}

func (r Rules) expand(dir string) string {
	if dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
		return r.env.Home + dir[1:]
	}
	return dir
}

// normDir compares dirs by env's conventions: on windows, case and
// separator style are ignored.
func (r Rules) normDir(dir string) string {
	if r.env.GOOS == "windows" {
		return strings.TrimRight(strings.ToLower(strings.ReplaceAll(dir, "/", `\`)), `\`)
	}
	return path.Clean(dir)
}
