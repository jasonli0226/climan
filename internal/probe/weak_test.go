package probe

import "testing"

func TestWeak(t *testing.T) {
	tests := []struct {
		summary, version string
		want             bool
	}{
		{"", "", true},
		{"zoxide 0.9.9", "zoxide 0.9.9", true},
		{"Usage: tool [x]", "", true},
		{"usage tool", "", true},
		{"aws: [ERROR]: An error occurred (ParamValidation): the following arguments are required: command", "", true},
		{"error: missing command", "", true},
		{"tool: unknown option --help", "", true},
		{"npm <command>", "", true},
		{"tool [options] file", "", true},
		{"tool --flag", "", true},
		{"search", "", false},
		{"usage-stats tool", "", false},
		{"Search files", "", false},
		{"tool - frob widgets", "", false},
		{"Run a command from a local or remote npm package", "", false},
	}
	for _, tt := range tests {
		if got := Weak(tt.summary, tt.version); got != tt.want {
			t.Errorf("Weak(%q, %q) = %v, want %v", tt.summary, tt.version, got, tt.want)
		}
	}
}
