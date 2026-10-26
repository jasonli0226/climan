package cli

import (
	"fmt"
	"io"
	"runtime/debug"
)

// Version is set at release build time with
// -ldflags "-X github.com/jasonli0226/climan/internal/cli.Version=v1.2.3".
var Version = ""

// devVersion is reported by a build with no release or module version.
const devVersion = "dev"

func runVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "climan %s\n", version())
	return exitOK
}

// version prefers the release stamp, then the module version that
// `go install ...@vX` records, then devVersion.
func version() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return devVersion
}
