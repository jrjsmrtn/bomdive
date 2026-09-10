// Package columns is the Miller-column view's model: the column chain, the cursor,
// the direction mode and the detail pane.
//
// IT DRAWS NOTHING. Keeping the logic free of any TUI library is what makes it
// testable, and it makes the library choice cheap to reverse — the widget layer
// only paints what this decides.
package columns

import (
	"sort"
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
	g   *bom.Graph
	dir Direction
	// cols is the chain, left to right. cols[0] is always the entry column.
	cols []Column
	// byCategory makes the entry column list categories rather than components,
	// which is the only navigable axis when a BOM declares no dependency graph.
	byCategory bool
}

// New builds the view over a loaded BOM.
//
// The entry column is chosen from the document rather than assumed: a BOM that
// declares no dependency graph opens on its categories, because descending
// dependencies would show nothing at all.
func New(g *bom.Graph) *Model {
	// HasCategories is load-bearing, not defensive. A document that declares no
	// graph AND carries no categories has no category axis: grouping it yields one
	// "(no category)" bucket holding everything, so the view announced it was
	// browsing categories and made the reader descend through a level that says
	// nothing. Falling back to a flat component list is the honest shape.
	m := &Model{g: g, byCategory: g.DeclaresNoGraph() && g.HasCategories()}
	m.cols = []Column{m.entryColumn()}
	return m
}

// Graph exposes the underlying BOM, for a caller that needs coverage or identity.
func (m *Model) Graph() *bom.Graph { return m.g }

// Direction reports which way the view faces.
func (m *Model) Direction() Direction { return m.dir }

// ByCategory reports whether the entry column lists categories.
func (m *Model) ByCategory() bool { return m.byCategory }

// Columns returns the chain.
func (m *Model) Columns() []Column { return m.cols }

// Axis names what the entry column lists. It exists so a caller can EXPLAIN the
// view without re-deriving the choice — entryColumn switches on it, so there is
// one decision and not two that can drift apart.
type Axis int

const (
	// AxisRoots: components nothing depends on, descending the dependency graph.
	AxisRoots Axis = iota
	// AxisCategories: cdx:osquery:category values, the OBOM navigation axis.
	AxisCategories
	// AxisFlat: every component, because there is no relation or category to
	// group by.
	AxisFlat
)

// Axis reports which entry column this document got.
func (m *Model) Axis() Axis {
	switch {
	case m.byCategory:
		return AxisCategories
	case m.g.Coverage().Edges == 0:
		return AxisFlat
	default:
		if roots, _ := m.g.Roots(); len(roots) == 0 {
			return AxisFlat
		}
		return AxisRoots
	}
}

func (m *Model) entryColumn() Column {
	switch m.Axis() {
	case AxisFlat:
		return Column{Title: "components", Entries: m.g.Components()}
	case AxisCategories:
		cats := m.g.Categories()
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
			// A synthetic node standing for the category. Ref is empty, which is how
			// the detail pane and descent tell a category from a component.
			entries = append(entries, bom.Node{Name: label, Type: "category", Category: c})
		}
		return Column{Title: "categories", Entries: entries}
	}

	roots, synthetic := m.g.Roots()
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
	// A category entry exists only because entryColumn found members for it, so it
	// always has some. Counting them would rebuild the whole category map per row.
	if n.Type == "category" && n.Ref == "" {
		return true
	}
	if m.dir == Reverse {
		return m.g.HasParents(n.Ref)
	}
	return m.g.HasChildren(n.Ref)
}

func (m *Model) childrenOf(n bom.Node) []bom.Node {
	if n.Type == "category" && n.Ref == "" {
		return m.g.Categories()[n.Category]
	}
	if m.dir == Reverse {
		return m.g.Parents(n.Ref)
	}
	return m.g.Children(n.Ref)
}

// ToggleDirection flips the whole view between dependencies and dependents.
//
// The chain is rebuilt from the current selection rather than kept, because the
// columns to the left describe a route that the flipped graph does not have.
func (m *Model) ToggleDirection() {
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
	if sel.Type == "category" && sel.Ref == "" {
		return []KV{
			{"category", sel.Name},
			{"components", itoa(len(m.g.Categories()[sel.Category]))},
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
	if sel.Category != "" {
		kv = append(kv, KV{"category", sel.Category})
	}
	kv = append(kv, KV{"dependencies", itoa(len(m.g.Children(sel.Ref)))})
	kv = append(kv, KV{"dependents", itoa(len(m.g.Parents(sel.Ref)))})
	for _, p := range m.g.Properties(sel.Ref) {
		kv = append(kv, KV{p.Name, p.Value})
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
