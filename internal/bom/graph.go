package bom

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

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

	// Doc is which loaded document the node belongs to — 0 when one is loaded. A
	// bom-ref is unique only within its own document, so across documents a node's
	// identity is (Doc, Ref).
	Doc int
}

// Label is the human-facing name for a node, falling back to its bom-ref.
//
// A component may carry NO name — seen in a real HBOM device list — and every
// surface that rendered Name raw showed a blank: an empty row in the column view,
// "@1.0.0" from ls, a bare "├──" from tree, and an empty segment in the path.
// Patching one of those was not a fix; this is the single place that decides.
func (n Node) Label() string {
	if n.Name != "" {
		return n.Name
	}
	if n.Ref != "" {
		return n.Ref
	}
	return "(unnamed)"
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

	// HasDepsKey is whether the document carried a `dependencies` field at all.
	// It rides along in Coverage rather than being fetched separately because
	// every surface that reports the numbers needs it to report them HONESTLY —
	// see State.
	HasDepsKey bool
}

// DanglingSample is how many dangling refs a human-readable surface names before
// it stops.
//
// Measured: a public 837-component npm SBOM carries 278 of them. Printing all of
// them filled the browse overlay to the full screen and pushed the explanation it
// annotates off the top — the list drowned its own point. The COUNT is what a
// reader acts on; the list is data, and data belongs in --json, which carries
// Dangling uncapped.
const DanglingSample = 5

// SampleDangling returns at most DanglingSample refs and how many were left out.
func (c Coverage) SampleDangling() (shown []string, omitted int) {
	if len(c.Dangling) <= DanglingSample {
		return c.Dangling, 0
	}
	return c.Dangling[:DanglingSample], len(c.Dangling) - DanglingSample
}

// Complete reports whether every component appears in the dependency graph.
func (c Coverage) Complete() bool { return c.Components > 0 && c.InGraph == c.Components }

// GraphState is what the document SAYS about relations, which is not the same as
// how many relations it has.
//
// Three of these four states show 0% coverage and mean different things, and two
// of them were being reported identically: a BOM with no `dependencies` field was
// labelled "partial", the same word used for a declared graph that genuinely misses
// components. "Partial" is a defect in the document; silence is not a claim at all.
// Making the distinction ONCE, here, is what stops a surface flattening it again —
// the tree renderer got it right and `ls` and the column view did not.
type GraphState int

const (
	// GraphComplete: relations are declared and reach every component.
	GraphComplete GraphState = iota
	// GraphPartial: relations are declared, and some components sit outside them.
	GraphPartial
	// GraphDeclaredEmpty: `dependencies` is present and empty — the document
	// ASSERTS there are no relations. Every OBOM measured does this.
	GraphDeclaredEmpty
	// GraphUndeclared: no `dependencies` field — the generator said NOTHING about
	// relations, which is not the same as asserting there are none.
	GraphUndeclared
)

// State classifies the document. Order matters: what the document declared is
// asked before how much it covers, because coverage is meaningless otherwise.
func (c Coverage) State() GraphState {
	switch {
	case !c.HasDepsKey:
		return GraphUndeclared
	case c.Edges == 0:
		return GraphDeclaredEmpty
	case c.Complete():
		return GraphComplete
	default:
		return GraphPartial
	}
}

// Label is the terse form, for a status bar with no room to explain itself. Empty
// for the unremarkable case, so a caller can test it rather than special-casing.
func (s GraphState) Label() string {
	switch s {
	case GraphPartial:
		return "partial"
	case GraphDeclaredEmpty:
		return "no dependency graph"
	case GraphUndeclared:
		return "relations undeclared"
	default:
		return ""
	}
}

// Explain is the long form: one sentence saying what the numbers mean, with the
// numbers in it. This is what `?` shows and what every non-interactive surface
// prints as a note, so the terse label always has somewhere to be explained.
func (c Coverage) Explain() string {
	switch c.State() {
	case GraphUndeclared:
		return fmt.Sprintf("coverage is 0 of %d because this BOM has no `dependencies` field at "+
			"all: the generator said NOTHING about relations, which is not the same as asserting "+
			"there are none. Nothing is missing — there was never a graph to be missing from.",
			c.Components)
	case GraphDeclaredEmpty:
		return fmt.Sprintf("coverage is 0 of %d because `dependencies` is present and EMPTY: the "+
			"document asserts there are no relations between its components. That is a statement, "+
			"not a gap — every OBOM measured looks like this.", c.Components)
	case GraphPartial:
		return fmt.Sprintf("the dependency graph reaches %d of %d components (%d%%) over %d "+
			"edge(s). The other %d are in the document, but nothing declares a relation to or "+
			"from them — so a tree cannot reach them and `ls` is the only way to see them.",
			c.InGraph, c.Components, c.percent(), c.Edges, c.Components-c.InGraph)
	default:
		return fmt.Sprintf("the dependency graph reaches all %d components over %d edge(s): "+
			"a tree walk shows the whole document.", c.Components, c.Edges)
	}
}

// Percent is the coverage percentage, rounded down. Exported because three
// surfaces computed it inline and could each round differently.
func (c Coverage) Percent() int { return c.percent() }

func (c Coverage) percent() int {
	if c.Components == 0 {
		return 0
	}
	return c.InGraph * 100 / c.Components
}

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

	// payload is counted at load, so a surface can say what an empty component
	// list MEANS. Its Components field is unused; Contents fills it from nodes.
	payload Contents

	// vulns are the `vulnerabilities` records, as written. serial and version identify
	// this document, so a BOM-Link can find it; digest tells a byte-identical copy from
	// a document that merely claims the same serial. path, doc and set place it in the
	// session: every graph belongs to a Set, of one document when loaded alone.
	vulns   []Vulnerability
	serial  string
	version int
	digest  [32]byte
	path    string
	doc     int
	set     *Set

	props map[string][]Property
}

// Load reads a CycloneDX JSON document.
func Load(path string) (*Graph, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	format, r, err := detectFormat(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var doc cdx.BOM
	// The decoder resolves the SPEC version from the document itself; only the
	// ENCODING has to be chosen here.
	if err := cdx.NewBOMDecoder(r, format).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// A document that parses but is not a BOM would otherwise render as an empty
	// one: "0 of 0 components", exit 0. That is the confidently-wrong shape this
	// tool exists to avoid — pointing at the wrong file must be an error, not a
	// plausible answer.
	//
	// The check is FORMAT-AWARE, because the two encodings identify themselves
	// differently and the library's own tags say so: BOMFormat is `xml:"-"` and
	// XMLNS is `json:"-"`. A JSON document carries bomFormat; an XML one carries
	// the CycloneDX namespace on its root element and no bomFormat at all. A
	// bomFormat-only guard silently rejects every valid XML BOM.
	if err := checkIsBOM(&doc, format); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	g := build(&doc)
	g.path = path
	g.digest = sha256.Sum256(raw)
	return g, nil
}

// checkIsBOM rejects a document that parsed but is not a CycloneDX BOM.
func checkIsBOM(doc *cdx.BOM, format cdx.BOMFileFormat) error {
	if format == cdx.BOMFileFormatXML {
		if !strings.Contains(doc.XMLNS, "cyclonedx.org/schema/bom") {
			return fmt.Errorf("not a CycloneDX document (xmlns=%q)", doc.XMLNS)
		}
		return nil
	}
	if doc.BOMFormat != "CycloneDX" {
		return fmt.Errorf("not a CycloneDX document (bomFormat=%q)", doc.BOMFormat)
	}
	if doc.SpecVersion == 0 {
		return fmt.Errorf("no specVersion")
	}
	return nil
}

// detectFormat sniffs JSON or XML from the CONTENT, not the file extension.
//
// Extensions lie: a BOM arrives as .bom, .cdx, .txt or no name at all, and picking
// the decoder from a suffix means a correctly-named file works while an identical
// one does not. Sniffing the first meaningful byte is both simpler and right.
//
// A UTF-8 byte order mark is skipped. Real generators emit them, and an unhandled
// BOM makes a valid document fail with a decoder error naming the wrong cause.
func detectFormat(r io.Reader) (cdx.BOMFileFormat, io.Reader, error) {
	br := bufio.NewReader(r)

	if mark, err := br.Peek(3); err == nil && bytes.Equal(mark, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}
	for {
		b, err := br.Peek(1)
		if err != nil {
			return 0, nil, fmt.Errorf("empty or unreadable document")
		}
		switch b[0] {
		case ' ', '\t', '\n', '\r':
			_, _ = br.Discard(1)
		case '{':
			return cdx.BOMFileFormatJSON, br, nil
		case '<':
			return cdx.BOMFileFormatXML, br, nil
		default:
			return 0, nil, fmt.Errorf("not JSON or XML (starts with %q)", string(b[0]))
		}
	}
}

func build(doc *cdx.BOM) *Graph {
	g := &Graph{
		id:    identify(doc),
		nodes: map[string]Node{},
		out:   map[string][]string{},
		in:    map[string][]string{},
		props: map[string][]Property{},
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
					if c.BOMRef != "" {
						g.props[c.BOMRef] = append(g.props[c.BOMRef], Property{p.Name, p.Value})
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

	g.payload = payloadOf(doc)

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
			}
		}
	}

	// Dangling is decided AFTER every edge is read, not during. Whether a ref
	// resolves can depend on an edge seen later — the declared root resolves only
	// once it is known to drive the graph — so deciding mid-loop made the answer
	// depend on the order `dependencies` happened to be written in.
	for _, targets := range g.out {
		for _, dst := range targets {
			if _, ok := g.Node(dst); !ok {
				g.dangling = append(g.dangling, dst)
			}
		}
	}
	sort.Strings(g.dangling)
	g.dangling = dedupe(g.dangling)

	g.serial = strings.TrimPrefix(doc.SerialNumber, "urn:uuid:")
	g.version = doc.Version
	g.loadVulnerabilities(doc)
	NewSet(g) // a document loaded alone is a set of one
	return g
}

// Contents is what the document carries at top level.
//
// It exists so a surface can explain an empty component list rather than drawing a
// blank pane. A standalone VEX, a services-only document and a document holding
// nothing but metadata all list zero components, and all three rendered
// identically: "0 of 0 (0%)" over an empty column, which reads as the tool having
// failed.
type Contents struct {
	Components      int
	Vulnerabilities int
	Services        int

	// Declarations is a CycloneDX Attestations section; Attestations counts the
	// attestations inside it. The section is recorded separately from the count,
	// because a declarations section holding only claims or evidence is still one.
	Declarations bool
	Attestations int

	// Definitions is a section defining standards for others to attest against;
	// Standards counts them.
	Definitions bool
	Standards   int
}

// payloadOf counts what a document carries at top level. Components is the RAW
// count — every listed component, addressable or not — which is what decides
// whether the document is an inventory at all. identify and build both call this,
// so there is one count and not two.
func payloadOf(doc *cdx.BOM) Contents {
	var c Contents
	if doc.Components != nil {
		c.Components = len(*doc.Components)
	}
	if doc.Vulnerabilities != nil {
		c.Vulnerabilities = len(*doc.Vulnerabilities)
	}
	if doc.Services != nil {
		c.Services = len(*doc.Services)
	}
	if d := doc.Declarations; d != nil {
		c.Declarations = true
		if d.Attestations != nil {
			c.Attestations = len(*d.Attestations)
		}
	}
	if d := doc.Definitions; d != nil {
		c.Definitions = true
		if d.Standards != nil {
			c.Standards = len(*d.Standards)
		}
	}
	return c
}

// Contents reports what the document carries. Components here is what lsxbom can
// LIST — components with a bom-ref — not the raw count payloadOf takes.
func (g *Graph) Contents() Contents {
	c := g.payload
	c.Components = len(g.nodes)
	return c
}

// ExplainEmpty says why there is nothing to list, and false when there IS.
//
// lsxbom navigates COMPONENTS. Saying so, and naming what the document holds
// instead, is the difference between "this tool does not cover that" and "this tool
// is broken" — which is what an unexplained empty pane looks like.
func (c Contents) ExplainEmpty() (string, bool) {
	if c.Components > 0 {
		return "", false
	}
	notABOM := "this is a CycloneDX document but NOT a bill of materials: it inventories " +
		"nothing"
	if kind, ok := c.nonBOMKind(); ok {
		switch kind {
		case KindAttestation:
			return fmt.Sprintf("%s. It is an attestation — a declarations section carrying "+
				"%d attestation(s) of conformance to a standard, with the claims and evidence "+
				"behind them. Nothing is missing here — lsxbom navigates components, and this "+
				"document has none to navigate.", notABOM, c.Attestations), true
		case KindDefinitions:
			return fmt.Sprintf("%s. It DEFINES %d standard(s) — requirements others attest "+
				"against — rather than describing a product. Nothing is missing here — lsxbom "+
				"navigates components, and this document has none to navigate.",
				notABOM, c.Standards), true
		}
	}
	switch {
	case c.Vulnerabilities > 0 && c.Services > 0:
		return fmt.Sprintf("this document declares NO components: it carries %d vulnerability "+
			"record(s) and %d service(s). lsxbom navigates components, so there is nothing "+
			"here to list.", c.Vulnerabilities, c.Services), true
	case c.Vulnerabilities > 0:
		return fmt.Sprintf("%s and carries %d vulnerability record(s). A standalone VEX "+
			"asserts which vulnerabilities affect a product described in ANOTHER document. "+
			"Nothing is missing here — lsxbom navigates components, and this document has "+
			"none to navigate.", notABOM, c.Vulnerabilities), true
	case c.Services > 0:
		return fmt.Sprintf("this document declares NO components: it carries %d service(s). "+
			"lsxbom navigates components, not services, so there is nothing here to list.",
			c.Services), true
	default:
		return "this document declares NO components, and no vulnerabilities, services, " +
			"declarations or definitions either — it carries metadata and nothing else. That is schema-valid: it names a " +
			"product without inventorying it.", true
	}
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
//
// A declared root that drives the graph but is absent from `components` resolves
// to a synthesised node. Doing it HERE rather than in Roots is what makes Walk,
// Children and the detail pane agree — fixing only Roots left the tree rooted and
// empty, which is a worse failure than having no root at all.
func (g *Graph) Node(ref string) (Node, bool) {
	if n, ok := g.nodes[ref]; ok {
		return n, true
	}
	if ref != "" && ref == g.id.RootRef {
		if _, drives := g.out[ref]; drives {
			return g.metadataRoot(), true
		}
	}
	return Node{}, false
}

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

// HasChildren and HasParents answer "is there anything to descend into" without
// resolving and SORTING a whole level, which the column view would otherwise do
// once per visible row — 4,889 of them on a measured OBOM, every repaint.
//
// They filter exactly as resolve does. A ref whose every target is dangling
// resolves to nothing, so testing len(g.out[ref]) instead would mark a row
// descendable that Right refuses to descend — an indicator that lies.
func (g *Graph) HasChildren(ref string) bool { return g.anyResolves(g.out[ref]) }

// HasParents is the same question facing the other way.
func (g *Graph) HasParents(ref string) bool { return g.anyResolves(g.in[ref]) }

func (g *Graph) anyResolves(refs []string) bool {
	for _, r := range refs {
		if _, ok := g.Node(r); ok {
			return true
		}
	}
	return false
}

// resolve turns refs into nodes, dropping any that name nothing.
//
// It asks Node rather than reading g.nodes, so ONE rule decides what a ref names:
// a component, or the declared root when that root drives the graph. Reading the
// map directly made the root invisible to every reverse edge — a document's direct
// dependencies reported NO dependents while `dependencies` plainly declared one,
// because metadata.component is the SUBJECT of a document rather than a member of
// its inventory and so is absent from `components`. Same root cause as POC-8, one
// layer down: Node knew about the root and the two functions built on it did not.
func (g *Graph) resolve(refs []string) []Node {
	out := make([]Node, 0, len(refs))
	for _, r := range refs {
		if n, ok := g.Node(r); ok {
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
		// The declared root drives the graph but is NOT listed in `components`.
		// That is legitimate and common in a merged host view: metadata.component
		// is the SUBJECT of the document, not a member of its inventory. Dropping
		// it left such a BOM with no roots at all — "no roots found to walk from"
		// on a document carrying 33 edges. Found by POC-8, on the one shape ADR-0004
		// had never measured.
		n, _ := g.Node(g.id.RootRef)
		return []Node{n}, false
	}
	var refs []string
	for ref := range g.out {
		if len(g.in[ref]) == 0 {
			refs = append(refs, ref)
		}
	}
	return g.resolve(refs), true
}

// metadataRoot synthesises a Node for a declared root that the components list
// does not contain, so a renderer has something to anchor on.
func (g *Graph) metadataRoot() Node {
	return Node{Ref: g.id.RootRef, Name: g.id.RootName, Type: g.id.RootType, Doc: g.doc}
}

// Coverage reports how much of the document the graph reaches.
//
// It counts g.nodes and NOT the declared root, unlike resolve. The two answer
// different questions: coverage asks how much of the document's INVENTORY the
// graph reaches, and metadata.component is the subject of the document rather than
// an item in it. Counting it would put InGraph above Components and make a fully
// covered document report as incomplete.
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
		HasDepsKey: g.hasDepsKey,
	}
}

// HasCategories reports whether ANY component carries a cdx:osquery:category.
//
// Without it, a document with no categories still groups into a single "(no
// category)" bucket, and the column view opened on one entry containing everything
// while announcing it was "browsing categories". A degenerate axis is worse than
// no axis: it adds a navigation step and implies a structure the document lacks.
func (g *Graph) HasCategories() bool {
	for _, n := range g.nodes {
		if n.Category != "" {
			return true
		}
	}
	return false
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

// Property is one component property, in document order.
type Property struct{ Name, Value string }

// Properties returns every property of a component, in DOCUMENT ORDER.
//
// Order is preserved rather than sorted: for an OBOM these are osquery fields, and
// the generator's ordering carries meaning that sorting destroys (ADR-0007).
func (g *Graph) Properties(ref string) []Property { return g.props[ref] }
