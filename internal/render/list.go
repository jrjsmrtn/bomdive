package render

import (
	"sort"
	"strings"

	"github.com/jrjsmrtn/lsxbom/internal/bom"
)

// ListOptions mirrors the ls(1) vocabulary a reader already has.
type ListOptions struct {
	From       string // list what this ref depends on; empty lists the whole document
	Type       string // filter by component type
	Category   string // filter by cdx:osquery:category
	ByCategory bool   // group by category instead of listing flat
	Reverse    bool   // list what depends on From, rather than what it depends on
}

// List produces a one-level listing.
//
// It is the universal command: it works on every BOM type measured, including the
// 14 of 14 OBOMs that carry no dependency graph at all. Where tree has nothing to
// walk, this still has everything to show.
func List(g *bom.Graph, source string, opt ListOptions) Result {
	r := New("ls", source, g)
	r.From = opt.From

	var nodes []bom.Node
	switch {
	case opt.From != "" && opt.Reverse:
		nodes = g.Parents(opt.From)
	case opt.From != "":
		nodes = g.Children(opt.From)
	default:
		nodes = g.Components()
	}

	if opt.From != "" {
		if _, ok := g.Node(opt.From); !ok {
			r.Notes = append(r.Notes, "no component with bom-ref "+opt.From)
			return r
		}
	}

	nodes = filter(nodes, opt)

	if opt.ByCategory {
		r.Entries = byCategory(g, nodes)
		r.Notes = append(r.Notes, "grouped by cdx:osquery:category")
	} else {
		for _, n := range nodes {
			r.Entries = append(r.Entries, entryOf(g, n))
		}
	}

	// Say why a listing is flat, rather than letting a reader infer the tool failed.
	if opt.From == "" && g.DeclaresNoGraph() {
		r.Notes = append(r.Notes,
			"this BOM declares no dependency graph (`dependencies` is present and empty) — "+
				"navigate it with --by-category")
	}
	return r
}

func filter(nodes []bom.Node, opt ListOptions) []bom.Node {
	if opt.Type == "" && opt.Category == "" {
		return nodes
	}
	out := nodes[:0:0]
	for _, n := range nodes {
		if opt.Type != "" && !strings.EqualFold(n.Type, opt.Type) {
			continue
		}
		if opt.Category != "" && !strings.EqualFold(n.Category, opt.Category) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// byCategory renders the OBOM navigation axis: a category header entry followed by
// its members. Categories are DISCOVERED from the document, never hardcoded — the
// vocabulary is platform-dependent, so a fixed list would be wrong on most hosts
// and wrong in a way that hides categories rather than erroring (POC-7).
func byCategory(g *bom.Graph, nodes []bom.Node) []Entry {
	groups := map[string][]bom.Node{}
	for _, n := range nodes {
		groups[n.Category] = append(groups[n.Category], n)
	}
	names := make([]string, 0, len(groups))
	for k := range groups {
		names = append(names, k)
	}
	sort.Strings(names)

	var out []Entry
	for _, cat := range names {
		label := cat
		if label == "" {
			label = "(no category)"
		}
		out = append(out, Entry{Name: label, Type: "category", Children: len(groups[cat])})
		for _, n := range groups[cat] {
			e := entryOf(g, n)
			e.Depth = 1
			out = append(out, e)
		}
	}
	return out
}
