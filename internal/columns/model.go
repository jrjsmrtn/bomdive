// Package columns is the Miller-column view's model: the column chain, the cursor,
// the direction mode and the detail pane.
//
// IT DRAWS NOTHING. Keeping the logic free of any TUI library is what makes it
// testable, and it makes the library choice cheap to reverse — the widget layer
// only paints what this decides.
package columns

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jrjsmrtn/lsxbom/internal/bom"
)

// Direction is which way the whole view faces (ADR-0007).
//
// It is a MODE, not a per-column choice. The column chain is a path, not a
// hierarchy: in a filesystem "left" is the parent and also where you came from, and
// those coincide — in a DAG they do not, because a component has many dependents.
// Moving left retraces your route; flipping direction reverses every column.
type Direction int

const (
	Forward Direction = iota // columns list what the selection depends on
	Reverse                  // columns list what depends on the selection
)

func (d Direction) String() string {
	if d == Reverse {
		return "dependents"
	}
	return "dependencies"
}

// Column is one pane: a title, its entries, and where the cursor sits.
type Column struct {
	Title   string
	Entries []bom.Node
	Cursor  int
	// Filter is the type-to-filter text applied to this column.
	Filter string
}

// Selected returns the highlighted node, if the column is not empty.
func (c *Column) Selected() (bom.Node, bool) {
	v := c.visible()
	if len(v) == 0 {
		return bom.Node{}, false
	}
	if c.Cursor >= len(v) {
		c.Cursor = len(v) - 1
	}
	return v[c.Cursor], true
}

// Visible returns the entries after filtering.
func (c *Column) Visible() []bom.Node { return c.visible() }

func (c *Column) visible() []bom.Node {
	if c.Filter == "" {
		return c.Entries
	}
	needle := strings.ToLower(c.Filter)
	out := make([]bom.Node, 0, len(c.Entries))
	for _, n := range c.Entries {
		// Filter on the LABEL, so an unnamed component can be found by its ref.
		if strings.Contains(strings.ToLower(n.Label()), needle) {
			out = append(out, n)
		}
	}
	return out
}

// KV is one row of the detail pane.
type KV struct{ Key, Value string }

// Model is the whole view.
type Model struct {
	// set is every document named on the command line; one document is a set of one.
	// Every row carries the document it belongs to, and every question about a row is
	// asked of THAT document: a bom-ref is unique only within its own.
	set  *bom.Set
	dir  Direction
	cols []Column

	// mode is the component or the vulnerability axis (ADR-0009). vdir is the
	// vulnerability axis's own direction, and saved is the component chain as it was
	// left, restored when the reader switches back.
	mode  Mode
	vdir  Direction
	saved []Column
}

// New builds the view over a loaded BOM, and over every document loaded with it.
//
// The entry column is chosen from the documents rather than assumed. Several named:
// the files, so none of them is out of reach (ADR-0009 [B]). One named: its own first
// column — a column listing one file is a column of one entry, which is not an axis.
func New(g *bom.Graph) *Model {
	m := &Model{set: g.Set()}
	m.cols = []Column{m.entryColumn()}
	return m
}

// Set exposes every loaded document.
func (m *Model) Set() *bom.Set { return m.set }

func (m *Model) multi() bool { return m.set.Len() > 1 }

// Graph is the document the view is currently describing: the file under the cursor
// in the files column, or the document of the selected row. The header, coverage and
// `?` describe it.
func (m *Model) Graph() *bom.Graph { return m.set.Doc(m.currentDoc()) }

func (m *Model) currentDoc() int {
	col := m.active()
	if m.mode == ModeComponents && m.multi() {
		col = &m.cols[0]
	}
	if sel, ok := col.Selected(); ok {
		return sel.Doc
	}
	return 0
}

func (m *Model) graphOf(n bom.Node) *bom.Graph { return m.set.Doc(n.Doc) }

// Direction reports which way the view faces.
func (m *Model) Direction() Direction { return m.dir }

// byCategoryOf: a document declaring no graph opens on its categories — but only if it
// HAS categories. HasCategories is load-bearing: without it a document with neither
// grouped into one "(no category)" bucket, and the view announced it was browsing
// categories while making the reader descend through a level that said nothing.
func byCategoryOf(g *bom.Graph) bool { return g.DeclaresNoGraph() && g.HasCategories() }

// ByCategory reports whether the current document's first column lists categories.
func (m *Model) ByCategory() bool { return byCategoryOf(m.Graph()) }

// Columns returns the chain.
func (m *Model) Columns() []Column { return m.cols }

// Axis names what the entry column lists. It exists so a caller can EXPLAIN the view
// without re-deriving the choice — entryColumn switches on it, so there is one decision
// and not two that can drift apart.
type Axis int

const (
	// AxisRoots: components nothing depends on, descending the dependency graph.
	AxisRoots Axis = iota
	// AxisCategories: cdx:osquery:category values, the OBOM navigation axis.
	AxisCategories
	// AxisFlat: every component, because there is no relation or category to group by.
	AxisFlat
	// AxisFiles: the documents named, when there are two or more.
	AxisFiles
)

// Axis reports which entry column this session got.
func (m *Model) Axis() Axis {
	if m.multi() {
		return AxisFiles
	}
	return docAxis(m.set.Doc(0))
}

// DocAxis is the first column the current document gets on its own — what descending
// into its file opens.
func (m *Model) DocAxis() Axis { return docAxis(m.Graph()) }

func docAxis(g *bom.Graph) Axis {
	switch {
	case byCategoryOf(g):
		return AxisCategories
	case g.Coverage().Edges == 0:
		return AxisFlat
	default:
		if roots, _ := g.Roots(); len(roots) == 0 {
			return AxisFlat
		}
		return AxisRoots
	}
}

func (m *Model) entryColumn() Column {
	if m.multi() {
		return m.filesColumn()
	}
	return entryColumnOf(m.set.Doc(0))
}

// filesColumn lists the documents named, in the order given, each with its kind.
func (m *Model) filesColumn() Column {
	entries := make([]bom.Node, 0, m.set.Len())
	for i, g := range m.set.Docs() {
		entries = append(entries, bom.Node{Type: typeFile, Doc: i, Category: strconv.Itoa(i), Name: fileLabel(g)})
	}
	return Column{Title: "files", Entries: entries}
}

func fileLabel(g *bom.Graph) string {
	name := filepath.Base(g.Path())
	if g.Path() == "" {
		name = "document " + strconv.Itoa(g.Doc()+1)
	}
	return name + " — " + string(g.Identity().Kind)
}

func entryColumnOf(g *bom.Graph) Column {
	switch docAxis(g) {
	case AxisFlat:
		return Column{Title: "components", Entries: g.Components()}
	case AxisCategories:
		cats := g.Categories()
		names := make([]string, 0, len(cats))
		for k := range cats {
			names = append(names, k)
		}
		sort.Strings(names)
		var entries []bom.Node
		for _, c := range names {
			label := c
			if label == "" {
				label = "(no category)"
			}
			// A synthetic node standing for the category: Ref empty, Type says which,
			// Category carries the key — and Doc, so it is asked of its own document.
			entries = append(entries, bom.Node{Name: label, Type: typeCategory, Category: c, Doc: g.Doc()})
		}
		return Column{Title: "categories", Entries: entries}
	}
	roots, synthetic := g.Roots()
	title := "roots"
	if synthetic {
		title = "roots (derived)"
	}
	return Column{Title: title, Entries: roots}
}

// active is the rightmost column, which is the one the cursor is in.
func (m *Model) active() *Column { return &m.cols[len(m.cols)-1] }

// Active returns the focused column.
func (m *Model) Active() *Column { return m.active() }

// Down moves the cursor down one.
func (m *Model) Down() {
	c := m.active()
	if n := len(c.Visible()); n > 0 && c.Cursor < n-1 {
		c.Cursor++
	}
}

// Up moves the cursor up one.
func (m *Model) Up() {
	if c := m.active(); c.Cursor > 0 {
		c.Cursor--
	}
}

// Right descends into the selection, appending a column.
//
// It is a no-op when the selection has no children, so the chain never grows an
// empty pane the user then has to back out of.
func (m *Model) Right() bool {
	sel, ok := m.active().Selected()
	if !ok || !m.Descendable(sel) {
		return false
	}
	next := m.childrenOf(sel)
	if len(next) == 0 {
		return false
	}
	m.cols = append(m.cols, Column{Title: sel.Label(), Entries: next})
	return true
}

// Left retraces one step. It does NOT go to a parent — a DAG node has many, and
// this is history rather than an edge.
func (m *Model) Left() bool {
	if len(m.cols) <= 1 {
		return false
	}
	m.cols = m.cols[:len(m.cols)-1]
	return true
}

// Descendable reports whether an entry has anything below it, in the direction the
// view currently faces. It is what Right guards on AND what the view marks with an
// arrow, so the indicator cannot promise a descent that Right then refuses.
//
// It answers without BUILDING the level, because the view asks once per visible row
// on every repaint. TestDescendableAgreesWithChildrenOf ties it to childrenOf, which
// is the honest way to keep a fast path from drifting from the slow one it mirrors.
func (m *Model) Descendable(n bom.Node) bool {
	if m.mode == ModeVulnerabilities {
		return m.vulnDescendable(n)
	}
	switch {
	case isEntry(n, typeFile):
		// A VEX file inventories nothing, so it opens on nothing — and says so by
		// carrying no arrow, rather than descending into an empty pane.
		return len(entryColumnOf(m.graphOf(n)).Entries) > 0
	case isEntry(n, typeCategory):
		// A category entry exists only because entryColumn found members for it.
		return true
	case m.dir == Reverse:
		return m.graphOf(n).HasParents(n.Ref)
	default:
		return m.graphOf(n).HasChildren(n.Ref)
	}
}

func (m *Model) childrenOf(n bom.Node) []bom.Node {
	if m.mode == ModeVulnerabilities {
		return m.vulnChildren(n)
	}
	switch {
	case isEntry(n, typeFile):
		return entryColumnOf(m.graphOf(n)).Entries
	case isEntry(n, typeCategory):
		return m.graphOf(n).Categories()[n.Category]
	case m.dir == Reverse:
		return m.graphOf(n).Parents(n.Ref)
	default:
		return m.graphOf(n).Children(n.Ref)
	}
}

// ToggleDirection flips the whole view between dependencies and dependents.
//
// The chain is rebuilt from the current selection rather than kept, because the
// columns to the left describe a route that the flipped graph does not have.
func (m *Model) ToggleDirection() {
	if m.mode == ModeVulnerabilities {
		m.toggleVulnDirection()
		return
	}
	sel, ok := m.active().Selected()
	if m.dir == Forward {
		m.dir = Reverse
	} else {
		m.dir = Forward
	}
	if !ok || sel.Ref == "" {
		m.cols = []Column{m.entryColumn()}
		return
	}
	m.cols = []Column{{Title: sel.Label(), Entries: []bom.Node{sel}}}
	m.Right()
}

// Filter sets the type-to-filter text on the active column and resets its cursor.
func (m *Model) Filter(s string) {
	c := m.active()
	c.Filter = s
	c.Cursor = 0
}

// Path is the chain of selections from the entry column to the cursor. It is the
// answer to "what pulled this in?" — permanently on screen rather than traced back
// through indentation.
func (m *Model) Path() []string {
	out := make([]string, 0, len(m.cols))
	for i := range m.cols {
		if sel, ok := m.cols[i].Selected(); ok {
			out = append(out, sel.Label())
		}
	}
	return out
}

// Detail is the rightmost pane: the selection's identity and EVERY property, in
// document order.
//
// No per-category schema (ADR-0007). 39 osquery categories carry between 1 and 37
// distinct keys, the vocabulary is open and platform-dependent, and a schema-driven
// pane would render a new category blank rather than erroring.
func (m *Model) Detail() []KV {
	sel, ok := m.active().Selected()
	if !ok {
		return nil
	}
	if m.mode == ModeVulnerabilities {
		if kv, handled := m.vulnDetail(sel); handled {
			return kv
		}
	}
	g := m.graphOf(sel)
	switch {
	case isEntry(sel, typeFile):
		return fileDetail(g)
	case isEntry(sel, typeCategory):
		return []KV{
			{"category", sel.Name},
			{"components", itoa(len(g.Categories()[sel.Category]))},
		}
	}
	kv := []KV{{"name", sel.Name}}
	if sel.Version != "" {
		kv = append(kv, KV{"version", sel.Version})
	}
	kv = append(kv, KV{"type", sel.Type})
	if sel.PURL != "" {
		kv = append(kv, KV{"purl", sel.PURL})
	}
	kv = append(kv, KV{"bom-ref", sel.Ref})
	if m.multi() {
		kv = append(kv, KV{"document", filepath.Base(g.Path())})
	}
	if sel.Category != "" {
		kv = append(kv, KV{"category", sel.Category})
	}
	kv = append(kv, KV{"dependencies", itoa(len(g.Children(sel.Ref)))})
	kv = append(kv, KV{"dependents", itoa(len(g.Parents(sel.Ref)))})
	kv = append(kv, m.vulnSummary(sel)...)
	for _, p := range g.Properties(sel.Ref) {
		kv = append(kv, KV{p.Name, p.Value})
	}
	return kv
}

// fileDetail describes one named document: what it is, what identifies it to a
// BOM-Link, and how many of its own references resolved.
func fileDetail(g *bom.Graph) []KV {
	c := g.Contents()
	kv := []KV{{"file", g.Path()}, {"kind", g.Identity().Describe()}}
	if g.Serial() != "" {
		kv = append(kv, KV{"serial", g.Serial()}, KV{"version", itoa(g.Version())})
	}
	kv = append(kv, KV{"components", itoa(c.Components)}, KV{"vulnerabilities", itoa(c.Vulnerabilities)})
	counts := g.ReferenceCounts()
	total := 0
	for _, n := range counts {
		total += n
	}
	if total > 0 {
		kv = append(kv, KV{"affects", fmt.Sprintf("%d of %d references resolved", counts[bom.RefResolved], total)})
	}
	return kv
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
