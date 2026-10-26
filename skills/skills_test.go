package skills

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jasonli0226/climan/internal/install"
)

var specName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func TestClimanFrontmatter(t *testing.T) {
	parts := strings.SplitN(string(Climan), "---\n", 3)
	if len(parts) != 3 || parts[0] != "" {
		t.Fatalf("no frontmatter in embedded skill")
	}
	var fm struct{ Name, Description string }
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		t.Fatal(err)
	}
	if fm.Name != "climan" || !specName.MatchString(fm.Name) || len(fm.Name) > 64 {
		t.Errorf("name = %q", fm.Name)
	}
	if fm.Description == "" || len(fm.Description) > 1024 {
		t.Errorf("description has %d chars", len(fm.Description))
	}
	if first, _, _ := strings.Cut(parts[2], "\n"); first != install.Marker {
		t.Errorf("line after frontmatter = %q, want the marker", first)
	}
	if strings.Contains(parts[2], "climan skill") {
		t.Error("body still mentions climan skill")
	}
}
