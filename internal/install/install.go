package install

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// FileName is the skill file inside the climan skill folder.
const FileName = "SKILL.md"

// skillDir is the folder the skill gets inside an agent's skills dir.
const skillDir = "climan"

// Outcome is what Install did for one agent.
type Outcome string

const (
	Installed Outcome = "installed"
	Unchanged Outcome = "unchanged"
	Skipped   Outcome = "skipped"
	Failed    Outcome = "failed"
)

// Result reports one agent's install.
type Result struct {
	Agent, Path string
	Outcome     Outcome
	Reason      string
}

func (r Result) String() string {
	s := string(r.Outcome) + ": " + r.Path
	if r.Reason != "" {
		s += " (" + r.Reason + ")"
	}
	return s
}

// Install writes doc to <skills dir>/climan/SKILL.md for each agent, in
// order, and never stops early. It skips symlinks and files without Marker.
func Install(agents []Agent, doc []byte) []Result {
	results := make([]Result, 0, len(agents))
	for _, a := range agents {
		results = append(results, installAgent(a, doc))
	}
	return results
}

func installAgent(a Agent, doc []byte) Result {
	dir := filepath.Join(a.SkillsDir, skillDir)
	file := filepath.Join(dir, FileName)
	for _, p := range []string{dir, file} {
		if target, ok := symlink(p); ok {
			return Result{Agent: a.Name, Path: p, Outcome: Skipped, Reason: "symlink to " + target}
		}
	}
	r := Result{Agent: a.Name, Path: file}
	old, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		r.Outcome, r.Reason = Failed, err.Error()
		return r
	case !bytes.Contains(old, []byte(Marker)):
		r.Outcome, r.Reason = Skipped, "not written by climan"
		return r
	case bytes.Equal(old, doc):
		r.Outcome = Unchanged
		return r
	}
	if err := writeAtomic(file, doc); err != nil {
		r.Outcome, r.Reason = Failed, err.Error()
		return r
	}
	r.Outcome = Installed
	return r
}

// symlink reports whether p is a symlink, and its target.
func symlink(p string) (string, bool) {
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(p)
	if err != nil {
		target = "?"
	}
	return target, true
}

// writeAtomic replaces path via a synced temp file and rename, so a crash
// never leaves a truncated file without the marker.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".SKILL-*.md")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(name, 0o644)
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}
