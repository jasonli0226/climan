package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	env := Env{GOOS: "linux", Home: "/h", Vars: map[string]string{ConfigDirVar: dir}}

	cfg, path, err := LoadConfig(env)
	if err != nil || cfg.SkillsDir != "" || path != filepath.Join(dir, "config.yaml") {
		t.Fatalf("missing file: %+v %q %v", cfg, path, err)
	}

	if err := os.WriteFile(path, []byte("skills_dir: ~/agent-skills\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err = LoadConfig(env)
	if err != nil || cfg.SkillsDir != "/h/agent-skills" {
		t.Fatalf("cfg = %+v, %v", cfg, err)
	}

	if err := os.WriteFile(path, []byte("skills_dir: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err = LoadConfig(env); err == nil || !strings.Contains(err.Error(), "config.yaml") {
		t.Fatalf("err = %v, want one naming config.yaml", err)
	}
}
