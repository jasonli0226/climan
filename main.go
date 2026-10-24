// Command climan catalogs the CLI tools installed on this machine.
package main

import (
	"os"

	"github.com/jasonli0226/climan/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
