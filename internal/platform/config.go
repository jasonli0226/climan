package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the optional settings file in the config dir.
const ConfigFile = "config.yaml"

// Config holds climan's settings.
type Config struct {
	SkillsDir string `yaml:"skills_dir"` // default --out for `climan skill`
}

// LoadConfig reads config.yaml from the config dir. A missing file is an
// empty Config. It also returns the file's path for messages.
func LoadConfig(env Env) (Config, string, error) {
	dir, err := env.ConfigDir()
	if err != nil {
		return Config{}, "", err
	}
	path := filepath.Join(dir, ConfigFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, path, nil
	}
	if err != nil {
		return Config{}, path, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, path, fmt.Errorf("%s: %w", path, err)
	}
	cfg.SkillsDir = env.ExpandHome(cfg.SkillsDir)
	return cfg, path, nil
}

// ExpandHome replaces a leading "~" in p with the home directory.
func (e Env) ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return e.Home + p[1:]
	}
	return p
}
