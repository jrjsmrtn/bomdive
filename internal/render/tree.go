// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package render

import "github.com/jrjsmrtn/bomdive/internal/bom"

// TreeOptions mirrors tree(1) where the vocabulary carries over.
type TreeOptions struct {
	From     string // walk from this ref; empty walks from derived roots
	MaxDepth int    // -L; 0 means unlimited
}

// Tree walks the dependency graph.
//
// It REFUSES INFORMATIVELY rather than printing nothing when the document declares
// no graph. An empty tree and a document that says it has no relations look
// identical on screen, and only one of them means the tool worked.
func Tree(g *bom.Graph, source string, opt TreeOptions) Result {
	r := New("tree", source, g)
	r.From = opt.From

	// Why there is nothing to walk is stated by the graph-state note that New
	// attaches. What belongs HERE is only what is specific to this command: that
	// the walk produced nothing, and where to go instead.
	if g.DeclaresNoGraph() || !g.HasDependenciesKey() {
		r.Notes = append(r.Notes, "there is nothing to walk: "+navigateInstead(g))
		return r
	}

	starts := []string{opt.From}
	if opt.From == "" {
		roots, synthetic := g.Roots()
		r.SyntheticRoots = synthetic
		starts = starts[:0]
		for _, n := range roots {
			starts = append(starts, n.Ref)
		}
		if synthetic && len(starts) > 0 {
			r.Notes = append(r.Notes,
				"roots are DERIVED from components nothing depends on; this document's declared "+
					"root is not in its dependency graph")
		}
	}
	if len(starts) == 0 {
		r.Notes = append(r.Notes, "no roots found to walk from")
		return r
	}

	for _, start := range starts {
		g.Walk(start, opt.MaxDepth, func(v bom.Visit) bool {
			e := entryOf(g, v.Node)
			e.Depth, e.Repeat, e.Cycle = v.Depth, v.Repeat, v.Cycle
			r.Entries = append(r.Entries, e)
			return true
		})
	}

	if !r.Coverage.Complete {
		r.Notes = append(r.Notes, "this tree does NOT show every component — see coverage")
	}
	return r
}

// navigateInstead names the axis that IS navigable. Advising --by-category on a
// document carrying no categories sends a reader to a single "(no category)"
// bucket, which is the degenerate-axis failure the column view had.
func navigateInstead(g *bom.Graph) string {
	if g.HasCategories() {
		return "use `bomdive ls --by-category` to navigate this document by category"
	}
	return "use `bomdive ls` to list its components"
}
