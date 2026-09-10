package bom

import (
	"fmt"
	"os"
	"sort"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// OSQueryCategory is the property an OBOM uses to group components. It is the
// navigation axis when there is no dependency graph: 3,109 of 3,110 components in
// a measured OBOM carried it (POC-7).
const OSQueryCategory = "cdx:osquery:category"

// Node is one component, reduced to what a renderer needs.
//
// Ref is the only reliable key. In a measured OBOM every bom-ref was unique across
// 3,110 components while names were not (2,708), and 28-40% carried no purl at all.
// Anything keyed on Name or PURL collides or drops a third of the inventory.
type Node struct {
	Ref      string
	Name     string
	Version  string
	Type     string
	PURL     string
	Category string // cdx:osquery:category, empty when absent
}

// Coverage is how much of the document the dependency graph actually reaches.
//
// It is reported on every traversal rather than offered as a flag: rendering 3 of
// 9,314 components as though they were the BOM is the failure ADR-0004 forbids.
type Coverage struct {
	Components int      // components in the document
	InGraph    int      // components appearing in the dependency graph
	Edges      int      // dependsOn entries
	Dangling   []string // dependsOn targets matching no component, sorted
}

// Complete reports whether every component appears in the dependency graph.
func (c Coverage) Complete() bool { return c.Components > 0 && c.InGraph == c.Components }

// Graph is a loaded BOM, navigable one level at a time.
type Graph struct {
	id    Identity
	nodes map[string]Node
	order []string // document order, so output is deterministic

	out map[string][]string
	in  map[string][]string

	hasDepsKey bool
	edges      int
	dangling   []string
}

// Load reads a CycloneDX JSON document.
func Load(path string) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var doc cdx.BOM
	// AutodetectJSON is unused here on purpose: the decoder resolves the spec
	// version from the document itself, across 1.0-1.7.
	if err := cdx.NewBOMDecoder(f, cdx.BOMFileFormatJSON).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return build(&doc), nil
}

func build(doc *cdx.BOM) *Graph {
	g := &Graph{
		id:    identify(doc),
		nodes: map[string]Node{},
		out:   map[string][]string{},
		in:    map[string][]string{},
	}

	if doc.Components != nil {
		for _, c := range *doc.Components {
			n := Node{
				Ref: c.BOMRef, Name: c.Name, Version: c.Version,
				Type: string(c.Type), PURL: c.PackageURL,
			}
			if c.Properties != nil {
				for _, p := range *c.Properties {
					if p.Name == OSQueryCategory {
						n.Category = p.Value
					}
				}
			}
			if n.Ref == "" {
				continue // unaddressable: nothing can reference it, so it cannot be navigated
			}
			if _, seen := g.nodes[n.Ref]; !seen {
				g.order = append(g.order, n.Ref)
			}
			g.nodes[n.Ref] = n
		}
	}

	// `dependencies` present-and-empty is the document asserting it has no
	// relations; absent means the generator said nothing. Callers must be able to
	// tell those apart, so record which it was before reading any entries.
	g.hasDepsKey = doc.Dependencies != nil

	if doc.Dependencies != nil {
		for _, d := range *doc.Dependencies {
			if d.Dependencies == nil {
				continue
			}
			for _, dst := range *d.Dependencies {
				g.out[d.Ref] = append(g.out[d.Ref], dst)
				g.in[dst] = append(g.in[dst], d.Ref)
				g.edges++
				if _, ok := g.nodes[dst]; !ok {
					g.dangling = append(g.dangling, dst)
				}
			}
		}
	}
	sort.Strings(g.dangling)
	g.dangling = dedupe(g.dangling)
	return g
}

// Identity reports what the document says it is.
func (g *Graph) Identity() Identity { return g.id }

// DeclaresNoGraph reports a document that carries `dependencies` and leaves it
// empty — an assertion that there are no relations, which every measured OBOM makes.
// Distinct from HasDependenciesKey being false, where the generator said nothing.
func (g *Graph) DeclaresNoGraph() bool { return g.hasDepsKey && g.edges == 0 }

// HasDependenciesKey reports whether the document carried a `dependencies` field.
func (g *Graph) HasDependenciesKey() bool { return g.hasDepsKey }

// Node returns one component by ref.
func (g *Graph) Node(ref string) (Node, bool) { n, ok := g.nodes[ref]; return n, ok }

// Components returns every component in document order.
func (g *Graph) Components() []Node {
	out := make([]Node, 0, len(g.order))
	for _, ref := range g.order {
		out = append(out, g.nodes[ref])
	}
	return out
}

// Children returns what ref depends on: ONE level, never a subtree.
//
// A dependsOn target with no matching component is skipped here and surfaced
// through Coverage.Dangling instead, so a renderer never has to invent a node.
func (g *Graph) Children(ref string) []Node { return g.resolve(g.out[ref]) }

// Parents returns what depends on ref: one level, the reverse edge.
//
// This direction has no filesystem analogue, and it is the one a BOM reader most
// often wants — "what pulled this in". ADR-0006 leaves which way a column view
// faces as an open question; the model answers both.
func (g *Graph) Parents(ref string) []Node { return g.resolve(g.in[ref]) }

func (g *Graph) resolve(refs []string) []Node {
	out := make([]Node, 0, len(refs))
	for _, r := range refs {
		if n, ok := g.nodes[r]; ok {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Ref < out[j].Ref
	})
	return out
}

// Roots returns the entry points for a traversal, and whether they were derived.
//
// synthetic is true when the roots were computed rather than read from
// metadata.component. That happens often: a measured syft SBOM declared a root whose
// bom-ref was a content hash while the graph was keyed by purl, so walking from the
// declared root reached ZERO components. A caller MUST surface synthetic=true —
// silently inventing a root claims something the document does not say.
func (g *Graph) Roots() (roots []Node, synthetic bool) {
	if _, ok := g.out[g.id.RootRef]; ok && g.id.RootRef != "" {
		if n, known := g.nodes[g.id.RootRef]; known {
			return []Node{n}, false
		}
	}
	var refs []string
	for ref := range g.out {
		if len(g.in[ref]) == 0 {
			refs = append(refs, ref)
		}
	}
	return g.resolve(refs), true
}

// Coverage reports how much of the document the graph reaches.
func (g *Graph) Coverage() Coverage {
	seen := map[string]bool{}
	for src, targets := range g.out {
		if _, ok := g.nodes[src]; ok {
			seen[src] = true
		}
		for _, dst := range targets {
			if _, ok := g.nodes[dst]; ok {
				seen[dst] = true
			}
		}
	}
	return Coverage{
		Components: len(g.nodes),
		InGraph:    len(seen),
		Edges:      g.edges,
		Dangling:   g.dangling,
	}
}

// Categories groups components by cdx:osquery:category, for a BOM with no graph.
// Components carrying no category are grouped under "".
func (g *Graph) Categories() map[string][]Node {
	out := map[string][]Node{}
	for _, ref := range g.order {
		n := g.nodes[ref]
		out[n.Category] = append(out[n.Category], n)
	}
	return out
}

func dedupe(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := s[:1]
	for _, x := range s[1:] {
		if x != out[len(out)-1] {
			out = append(out, x)
		}
	}
	return out
}
