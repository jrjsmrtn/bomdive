package cli

import (
	"github.com/jrjsmrtn/lsxbom/internal/render"
	"github.com/spf13/cobra"
)

func (a *app) treeCmd() *cobra.Command {
	var opt render.TreeOptions
	c := &cobra.Command{
		Use:   "tree <bom.json> [bom-ref]",
		Short: "Walk the dependency graph",
		Long: "Walks the dependency graph from its roots, or from a given bom-ref.\n\n" +
			"The graph is a DAG with cycles, so a shared component is expanded once and\n" +
			"back-referenced afterwards, and a cycle is marked rather than truncated.\n" +
			"Coverage is always reported: a tree that shows 3 of 9,314 components without\n" +
			"saying so is worse than no tree.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := load(args[0])
			if err != nil {
				return err
			}
			if len(args) == 2 {
				opt.From = args[1]
			}
			return a.emit(render.Tree(g, args[0], opt))
		},
	}
	c.Flags().IntVarP(&opt.MaxDepth, "level", "L", 0, "descend at most this many levels (0 = unlimited)")
	return c
}
