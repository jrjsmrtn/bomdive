// Command bomdive reads a CycloneDX xBOM at the terminal.
package main

import (
	"os"

	"github.com/jrjsmrtn/bomdive/internal/cli"
)

// version is overridden at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() { os.Exit(cli.Run(version, os.Args[1:], os.Stdout, os.Stderr)) }
