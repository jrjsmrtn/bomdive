// Package cli wires the commands.
package cli

import (
	"fmt"
	"os"

	"github.com/jrjsmrtn/lsxbom/internal/bom"
	"github.com/jrjsmrtn/lsxbom/internal/render"
	"github.com/spf13/cobra"
)

var (
	flagJSON bool
	flagLong bool
)

// Execute runs the CLI.
func Execute(version string) int {
	root := &cobra.Command{
		Use:   "lsxbom",
		Short: "Read a CycloneDX xBOM at the terminal",
		Long: "lsxbom reads a CycloneDX xBOM — SBOM, OBOM, HBOM — and navigates it with the\n" +
			"muscle memory of ls(1) and tree(1).\n\n" +
			"It is not a BOM generator, query tool, scanner, signer or converter.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "emit JSON instead of text")
	root.PersistentFlags().BoolVarP(&flagLong, "long", "l", false, "long format: type, purl and category")
	root.AddCommand(lsCmd(), treeCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lsxbom:", err)
		return 1
	}
	return 0
}

func load(path string) (*bom.Graph, error) { return bom.Load(path) }

func emit(r render.Result) error {
	if flagJSON {
		return render.JSON(os.Stdout, r)
	}
	return render.Text(os.Stdout, r, flagLong)
}
