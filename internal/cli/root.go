// Package cli wires the commands.
package cli

import (
	"fmt"
	"io"

	"github.com/jrjsmrtn/lsxbom/internal/bom"
	"github.com/jrjsmrtn/lsxbom/internal/render"
	"github.com/spf13/cobra"
)

// app holds per-invocation state.
//
// Flags are not package-level globals, for two reasons that are worth separating.
// The real one: Run takes its streams, so a test can drive the CLI in-process
// without capturing os.Stdout, and nothing mutable is shared between callers.
//
// The reason that does NOT apply, recorded so nobody re-derives it: cobra examples
// use package globals, and it is tempting to say a second Run would inherit the
// first invocation's flags. Mutation testing showed it would not — cobra
// re-registers every flag with its default on each Run, which resets them. Avoid
// the globals on design grounds, not on a bug that does not exist here.
type app struct {
	out  io.Writer
	err  io.Writer
	json bool
	long bool
}

// Run executes the CLI with explicit args and streams, and returns the exit code.
//
// Everything is a parameter so a caller can drive it without a subprocess and
// without capturing os.Stdout.
func Run(version string, args []string, stdout, stderr io.Writer) int {
	a := &app{out: stdout, err: stderr}

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
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().BoolVar(&a.json, "json", false, "emit JSON instead of text")
	root.PersistentFlags().BoolVarP(&a.long, "long", "l", false, "long format: type, purl and category")
	root.AddCommand(a.lsCmd(), a.treeCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "lsxbom:", err)
		return 1
	}
	return 0
}

func (a *app) emit(r render.Result) error {
	if a.json {
		return render.JSON(a.out, r)
	}
	return render.Text(a.out, r, a.long)
}

func load(path string) (*bom.Graph, error) { return bom.Load(path) }
