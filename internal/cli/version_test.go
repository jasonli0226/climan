package cli

import "testing"

func TestVersionCommand(t *testing.T) {
	old := Version
	Version = "v1.2.3"
	t.Cleanup(func() { Version = old })
	for _, arg := range []string{"version", "--version", "-v"} {
		if out := mustRun(t, arg); out != "climan v1.2.3\n" {
			t.Errorf("%s = %q, want climan v1.2.3", arg, out)
		}
	}
}

func TestVersionFallback(t *testing.T) {
	old := Version
	Version = ""
	t.Cleanup(func() { Version = old })
	// A test binary has no module version, so the fallback is "dev".
	if out := mustRun(t, "version"); out != "climan dev\n" {
		t.Fatalf("version = %q, want climan dev", out)
	}
}
