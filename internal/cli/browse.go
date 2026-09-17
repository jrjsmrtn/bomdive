// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"os"

	"github.com/jrjsmrtn/bomdive/internal/bom"
	"github.com/jrjsmrtn/bomdive/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (a *app) browseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "browse <bom.json|dir> [<linked.json|dir> ...]",
		Short: "Navigate the document in Miller columns",
		Long: "Opens a Miller-column view — the column browser macOS Finder made familiar.\n" +
			"Each column lists the children of the selection in the column to its left, so\n" +
			"the chain of columns IS the dependency path.\n\n" +
			"Press Tab to flip the whole view between dependencies and dependents — the\n" +
			"reverse edge answers \"what pulled this in?\", which is the question a BOM\n" +
			"reader usually brings. A BOM with no dependency graph opens on its osquery\n" +
			"categories instead, because descending dependencies there shows nothing.\n\n" +
			"Name several documents to follow BOM-Links between them — a VEX and the SBOM it\n" +
			"describes, say. The leftmost column then lists the files, and v shows every\n" +
			"document's vulnerabilities.\n\n" +
			"Name a directory to load the CycloneDX documents directly inside it. A file that\n" +
			"is not CycloneDX is skipped and listed by ?; one that is and fails to load is\n" +
			"shown as a row saying why. A file named on its own must load.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			set, err := bom.LoadArgs(args)
			if err != nil {
				return err
			}
			g := set.Doc(0)
			// A TUI needs a terminal. Failing clearly beats a cryptic tcell error, and
			// beats drawing escape codes into a pipe.
			if !term.IsTerminal(int(os.Stdout.Fd())) {
				return errors.New("browse needs an interactive terminal; use `ls` or `tree` in a pipeline")
			}
			return tui.Run(g, g.Path())
		},
	}
}
