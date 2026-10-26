package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasonli0226/climan/internal/platform"
)

func configDirs(agents []Agent) []string {
	var dirs []string
	for _, a := range agents {
		dirs = append(dirs, a.ConfigDir)
	}
	return dirs
}

func names(agents []Agent) string {
	var out []string
	for _, a := range agents {
		out = append(out, a.Name)
	}
	return strings.Join(out, ",")
}

func TestAgentsDefaults(t *testing.T) {
	h := t.TempDir()
	agents := Agents(platform.Env{Home: h, Vars: map[string]string{}})
	want := []string{filepath.Join(h, ".claude"), filepath.Join(h, ".pi", "agent"), filepath.Join(h, ".config", "opencode")}
	if names(agents) != "claude,pi,opencode" || strings.Join(configDirs(agents), "|") != strings.Join(want, "|") {
		t.Fatalf("agents = %+v", agents)
	}
	for _, a := range agents {
		if a.SkillsDir != filepath.Join(a.ConfigDir, "skills") {
			t.Errorf("%s skills dir = %s", a.Name, a.SkillsDir)
		}
	}
}

func TestAgentsEnvOverrides(t *testing.T) {
	vars := map[string]string{"CLAUDE_CONFIG_DIR": "/c", "PI_CODING_AGENT_DIR": "/p", "XDG_CONFIG_HOME": "/x"}
	got := configDirs(Agents(platform.Env{Home: "/h", Vars: vars}))
	want := []string{"/c", "/p", filepath.Join("/x", "opencode")}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("config dirs = %v, want %v", got, want)
	}
}

func TestAgentsEmptyVarIsUnset(t *testing.T) {
	vars := map[string]string{"CLAUDE_CONFIG_DIR": "", "PI_CODING_AGENT_DIR": "", "XDG_CONFIG_HOME": ""}
	got := configDirs(Agents(platform.Env{Home: "/h", Vars: vars}))
	want := []string{filepath.Join("/h", ".claude"), filepath.Join("/h", ".pi", "agent"), filepath.Join("/h", ".config", "opencode")}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("config dirs = %v, want %v", got, want)
	}
}

func TestDetected(t *testing.T) {
	h := t.TempDir()
	if err := os.MkdirAll(filepath.Join(h, ".pi", "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h, ".claude"), nil, 0o644); err != nil {
		t.Fatal(err) // a file, not a dir: not detected
	}
	got := Detected(Agents(platform.Env{Home: h, Vars: map[string]string{}}))
	if names(got) != "pi" {
		t.Fatalf("detected = %s, want pi", names(got))
	}
}

func TestSelect(t *testing.T) {
	agents := Agents(platform.Env{Home: "/h", Vars: map[string]string{}})
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{[]string{"pi", "claude", "pi"}, "claude,pi"},
		{[]string{"claude", ""}, "claude"},
		{[]string{" opencode "}, "opencode"},
	} {
		got, err := Select(agents, tc.in)
		if err != nil || names(got) != tc.want {
			t.Errorf("Select(%q) = %s, %v; want %s", tc.in, names(got), err, tc.want)
		}
	}
	if _, err := Select(agents, []string{"cursor"}); err == nil ||
		!strings.Contains(err.Error(), "cursor") || !strings.Contains(err.Error(), "claude, pi or opencode") {
		t.Errorf("unknown agent err = %v", err)
	}
	if _, err := Select(agents, []string{""}); err == nil {
		t.Error("empty selection: want error")
	}
}
