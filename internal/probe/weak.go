package probe

import "strings"

// errorMarkers are lowercase fragments of lines a tool prints when it
// rejects its arguments instead of describing itself.
var errorMarkers = []string{
	"error:", "[error]", "error occurred", "required", "unknown option",
	"unrecognized", "invalid option", "not found", "no such",
}

// Weak reports whether summary fails to describe its tool: empty, the
// version line, a usage or synopsis line, or an error message.
func Weak(summary, version string) bool {
	lower := strings.ToLower(summary)
	fields := strings.Fields(summary)
	switch {
	case len(fields) == 0, summary == version:
		return true
	case strings.HasPrefix(lower, "usage:"), strings.HasPrefix(lower, "usage "):
		return true
	case len(fields) > 1 && isSynopsis(fields):
		return true
	}
	return isError(summary)
}

// isError reports a line that reads like an error message.
func isError(line string) bool {
	lower := strings.ToLower(line)
	for _, m := range errorMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// isSynopsis reports a line like "npm <command>" or "tool [options] file":
// a name followed by a placeholder or a flag.
func isSynopsis(fields []string) bool {
	second := fields[1]
	return strings.HasPrefix(second, "<") || strings.HasPrefix(second, "[") ||
		(strings.HasPrefix(second, "-") && len(second) > 1)
}
