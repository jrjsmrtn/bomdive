package cli

import (
	"errors"
	"os"

	"github.com/jrjsmrtn/lsxbom/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (a *app) browseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "browse <bom.json>",
		Short: "Navigate the BOM in a Finder-style column view",
		Long: "Opens a Miller-column view: each column lists the children of the selection\n" +
			"in the column to its left, so the chain of columns IS the dependency path.\n\n" +
			"Press Tab to flip the whole view between dependencies and dependents — the\n" +
			"reverse edge answers \"what pulled this in?\", which is the question a BOM\n" +
			"reader usually brings. A BOM with no dependency graph opens on its osquery\n" +
			"categories instead, because descending dependencies there shows nothing.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := load(args[0])
			if err != nil {
				return err
			}
			// A TUI needs a terminal. Failing clearly beats a cryptic tcell error, and
			// beats drawing escape codes into a pipe.
			if !term.IsTerminal(int(os.Stdout.Fd())) {
				return errors.New("browse needs an interactive terminal; use `ls` or `tree` in a pipeline")
			}
			return tui.Run(g, args[0])
		},
	}
}
