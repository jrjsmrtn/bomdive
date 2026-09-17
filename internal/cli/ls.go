package cli

import (
	"github.com/jrjsmrtn/bomdive/internal/render"
	"github.com/spf13/cobra"
)

func (a *app) lsCmd() *cobra.Command {
	var opt render.ListOptions
	c := &cobra.Command{
		Use:   "ls <bom.json> [bom-ref]",
		Short: "List components, or what one component depends on",
		Long: "Lists the components in a BOM. Given a bom-ref, lists what that component\n" +
			"depends on — one level, like ls(1) on a directory.\n\n" +
			"This is the universal command: it works on every BOM type, including the\n" +
			"OBOMs that carry no dependency graph at all, where tree has nothing to walk.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := load(args[0])
			if err != nil {
				return err
			}
			if len(args) == 2 {
				opt.From = args[1]
			}
			return a.emit(render.List(g, args[0], opt))
		},
	}
	c.Flags().StringVar(&opt.Type, "type", "", "only components of this type (library, device, data, …)")
	c.Flags().StringVar(&opt.Category, "category", "", "only components in this cdx:osquery:category")
	c.Flags().BoolVar(&opt.ByCategory, "by-category", false, "group by cdx:osquery:category (the OBOM axis)")
	c.Flags().BoolVarP(&opt.Reverse, "reverse", "r", false, "list what depends on the ref, not what it depends on")
	return c
}
