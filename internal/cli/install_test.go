package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasonli0226/climan/skills"
)

// agentHome points every agent's config dir at a fresh temp home.
func agentHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, v := range []string{"CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "XDG_CONFIG_HOME"} {
		t.Setenv(v, "")
	}
	return home
}

func TestInstallAgentFlag(t *testing.T) {
	home := agentHome(t)
	code, stdout, stderr := run(t, "install", "--agent", "pi")
	path := filepath.Join(home, ".pi", "agent", "skills", "climan", "SKILL.md")
	if code != 0 || stdout != "installed: "+path+"\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, skills.Climan) {
		t.Fatalf("installed file differs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); err == nil {
		t.Error("claude dir created")
	}
}

func TestInstallAgentFlagErrors(t *testing.T) {
	agentHome(t)
	for _, args := range [][]string{
		{"install", "--agent", "cursor"},
		{"install", "--agent", ""},
		{"install", "--agent", ","},
		{"install", "extra"},
	} {
		if code, _, _ := run(t, args...); code != 2 {
			t.Errorf("%q: code = %d, want 2", args, code)
		}
	}
	if code, _, stderr := run(t, "install", "--agent", "claude,"); code != 0 {
		t.Errorf("--agent claude,: code=%d stderr=%q", code, stderr)
	}
}

func TestInstallNoAgents(t *testing.T) {
	agentHome(t)
	code, _, stderr := run(t, "install")
	if code != 1 || !strings.Contains(stderr, "no supported agent found") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestInstallNotTerminal(t *testing.T) {
	home := agentHome(t)
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, "install")
	if code != 1 || !strings.Contains(stderr, "use --agent") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestInstallFailureExit(t *testing.T) {
	home := agentHome(t)
	skillsDir := filepath.Join(home, ".pi", "agent", "skills")
	if err := os.MkdirAll(filepath.Dir(skillsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillsDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := run(t, "install", "--agent", "claude,pi")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if code != 1 || len(lines) != 2 || !strings.HasPrefix(lines[0], "installed: ") || !strings.HasPrefix(lines[1], "failed: ") {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
}
