package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jasonli0226/climan/internal/discover"
	"github.com/jasonli0226/climan/internal/platform"
)

const specExample = `rules:
  - dir: ~/.cargo/bin
    action: enable
  - name: "*-completion"
    action: ignore
  - dir: ~/.local/share/pnpm
    action: enable
    tags: [js]
`

var linux = platform.Env{GOOS: "linux", Home: "/home/u"}

func load(t *testing.T, env platform.Env, content string) (Rules, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(p, env)
}

func toolIn(key, dir string) discover.Tool {
	return discover.Tool{Key: key, Members: []discover.Found{{Name: key, Dir: dir}}}
}

func TestMatchSpecExample(t *testing.T) {
	r, err := load(t, linux, specExample)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		tool       discover.Tool
		wantAction string
		wantTags   []string
	}{
		{toolIn("rg", "/home/u/.cargo/bin"), "enable", nil},
		{toolIn("foo-completion", "/usr/local/bin"), "ignore", nil},
		{toolIn("pnpm", "/home/u/.local/share/pnpm/"), "enable", []string{"js"}},
	}
	for _, tt := range tests {
		rule, ok := r.Match(tt.tool)
		if !ok || rule.Action != tt.wantAction || !reflect.DeepEqual(rule.Tags, tt.wantTags) {
			t.Errorf("Match(%s) = %+v, %v; want %s %v", tt.tool.Key, rule, ok, tt.wantAction, tt.wantTags)
		}
	}
	if _, ok := r.Match(toolIn("zoxide", "/home/u/.local/bin")); ok {
		t.Error("Match(zoxide) matched, want no match")
	}
}

func TestMatchNeedsDirAndName(t *testing.T) {
	r, err := load(t, linux, "rules:\n  - dir: /opt/bin\n    name: \"go*\"\n    action: enable\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Match(toolIn("gopls", "/opt/bin")); !ok {
		t.Error("gopls in /opt/bin should match")
	}
	if _, ok := r.Match(toolIn("gopls", "/usr/local/bin")); ok {
		t.Error("gopls elsewhere should not match")
	}
	if _, ok := r.Match(toolIn("rg", "/opt/bin")); ok {
		t.Error("rg in /opt/bin should not match")
	}
}

func TestMatchFirstWins(t *testing.T) {
	r, err := load(t, linux, "rules:\n  - name: \"*\"\n    action: ignore\n  - name: rg\n    action: enable\n")
	if err != nil {
		t.Fatal(err)
	}
	if rule, _ := r.Match(toolIn("rg", "/x")); rule.Action != "ignore" {
		t.Fatalf("action = %q, want ignore (first match)", rule.Action)
	}
}

func TestMatchWindowsDirCaseInsensitive(t *testing.T) {
	win := platform.Env{GOOS: "windows", Home: `C:\Users\u`}
	r, err := load(t, win, "rules:\n  - dir: ~/scoop/shims\n    action: enable\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Match(toolIn("rg", `C:\USERS\U\scoop\Shims`)); !ok {
		t.Fatal("windows dir match should ignore case and separators")
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct{ name, content, want string }{
		{"no matcher", "rules:\n  - action: enable\n", "rule 1"},
		{"bad action", "rules:\n  - name: x\n    action: delete\n", "delete"},
		{"bad glob", "rules:\n  - name: \"[\"\n    action: enable\n", "rule 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := load(t, linux, tt.content)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want mention of %q", err, tt.want)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "rules.yaml")
	_, err := Load(p, linux)
	if err == nil || !strings.Contains(err.Error(), "no rules file at "+p) || !strings.Contains(err.Error(), "--batch") {
		t.Fatalf("err = %v, want missing-file hint", err)
	}
}
