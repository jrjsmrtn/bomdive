// Package render turns a traversal into text or JSON.
//
// Both surfaces are produced from ONE result type. That is deliberate: ADR-0004
// makes coverage a correctness guarantee rather than a flag, and the cheapest way
// to keep a guarantee is to make it structurally impossible to omit. A renderer
// cannot forget to print coverage, because coverage is a field of the thing it is
// handed rather than something it must remember to fetch.
package render

import "github.com/jrjsmrtn/lsxbom/internal/bom"

// Entry is one line of output.
type Entry struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Type     string `json:"type,omitempty"`
	PURL     string `json:"purl,omitempty"`
	Category string `json:"category,omitempty"`

	// Depth, Repeat and Cycle are meaningful for a tree and zero for a listing.
	Depth  int  `json:"depth,omitempty"`
	Repeat bool `json:"repeat,omitempty"`
	Cycle  bool `json:"cycle,omitempty"`

	// Children is how many components this one depends on. In a flat listing it is
	// what tells a reader whether descending is even possible — which is how a
	// sparse graph becomes visible rather than misleading.
	Children int `json:"children"`
}

// Coverage mirrors bom.Coverage for serialisation, plus the derived percentage a
// human reader actually wants.
type Coverage struct {
	Components int      `json:"components"`
	InGraph    int      `json:"in_graph"`
	Percent    int      `json:"percent"`
	Edges      int      `json:"edges"`
	Complete   bool     `json:"complete"`
	Dangling   []string `json:"dangling,omitempty"`
}

// Result is everything a renderer needs. Every field that carries a caveat is
// mandatory rather than optional, so no surface can quietly drop one.
type Result struct {
	Command string `json:"command"`
	Source  string `json:"source"`

	// Kind and Identity say what the document claims to be, so a reader can tell
	// an empty result from a misread document.
	Kind     string `json:"kind"`
	Identity string `json:"identity"`

	// From is the ref that was listed or walked, empty for a whole-document listing.
	From string `json:"from,omitempty"`

	// SyntheticRoots is true when roots were derived rather than declared. A caller
	// MUST surface it: presenting an invented root as declared claims something the
	// document does not say.
	SyntheticRoots bool `json:"synthetic_roots"`

	// DeclaresNoGraph is the document asserting it has no relations, which is
	// different from a generator having computed none. HasDependencies records
	// whether the field was there at all.
	DeclaresNoGraph bool `json:"declares_no_graph"`
	HasDependencies bool `json:"has_dependencies"`

	Coverage Coverage `json:"coverage"`
	Entries  []Entry  `json:"entries"`

	// Notes carry anything a reader must know to read the entries correctly.
	Notes []string `json:"notes,omitempty"`
}

func coverageOf(c bom.Coverage) Coverage {
	pct := 0
	if c.Components > 0 {
		pct = c.InGraph * 100 / c.Components
	}
	return Coverage{
		Components: c.Components, InGraph: c.InGraph, Percent: pct,
		Edges: c.Edges, Complete: c.Complete(), Dangling: c.Dangling,
	}
}

func entryOf(g *bom.Graph, n bom.Node) Entry {
	return Entry{
		Ref: n.Ref, Name: n.Name, Version: n.Version, Type: n.Type,
		PURL: n.PURL, Category: n.Category, Children: len(g.Children(n.Ref)),
	}
}

// New builds the common part of a Result, so every command starts from the same
// mandatory caveats rather than assembling them by hand.
func New(command, source string, g *bom.Graph) Result {
	id := g.Identity()
	return Result{
		Command: command, Source: source,
		Kind: string(id.Kind), Identity: id.Describe(),
		DeclaresNoGraph: g.DeclaresNoGraph(),
		HasDependencies: g.HasDependenciesKey(),
		Coverage:        coverageOf(g.Coverage()),
	}
}
