// Command lsxbom reads a CycloneDX xBOM at the terminal.
package main

import (
	"os"

	"github.com/jrjsmrtn/lsxbom/internal/cli"
)

// version is overridden at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() { os.Exit(cli.Execute(version)) }
