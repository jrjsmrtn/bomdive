package columns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jrjsmrtn/lsxbom/internal/bom"
)

func load(t *testing.T, name string) *bom.Graph {
	t.Helper()
	g, err := bom.Load(filepath.Join("..", "..", "testdata", name+".cdx.json"))
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return g
}

// selectByName moves the cursor to a named entry in the active column, so a test
// navigates deliberately rather than assuming where the cursor happens to land.
func selectByName(t *testing.T, m *Model, name string) {
	t.Helper()
	c := m.Active()
	for i, n := range c.Visible() {
		if n.Name == name {
			c.Cursor = i
			return
		}
	}
	var got []string
	for _, n := range c.Visible() {
		got = append(got, n.Name)
	}
	t.Fatalf("no entry named %q in the active column; have %v", name, got)
}

func TestOpensOnRootsForAGraphBOM(t *testing.T) {
	m := New(load(t, "diamond"))
	if m.ByCategory() {
		t.Error("opened on categories for a BOM that has a graph")
	}
	if got := m.Active().Title; got != "roots" {
		t.Errorf("entry column title = %q, want roots", got)
	}
}

// A graphless BOM must open somewhere navigable. Descending dependencies would
// show nothing at all, so the entry column is the category axis.
func TestOpensOnCategoriesWhenThereIsNoGraph(t *testing.T) {
	m := New(load(t, "obom-categories"))
	if !m.ByCategory() {
		t.Fatal("did not open on categories for a BOM declaring no graph")
	}
	if got := len(m.Active().Entries); got != 4 {
		t.Errorf("categories = %d, want 4", got)
	}
}

func TestDerivedRootsAreLabelledInTheColumnTitle(t *testing.T) {
	m := New(load(t, "rootless"))
	if !strings.Contains(m.Active().Title, "derived") {
		t.Errorf("title = %q; a derived root must say so", m.Active().Title)
	}
}

func TestRightDescendsAndLeftRetraces(t *testing.T) {
	m := New(load(t, "diamond"))
	if !m.Right() {
		t.Fatal("could not descend from the root")
	}
	if len(m.Columns()) != 2 {
		t.Fatalf("columns = %d, want 2", len(m.Columns()))
	}
	if !m.Left() {
		t.Fatal("could not retrace")
	}
	if len(m.Columns()) != 1 {
		t.Errorf("columns = %d after Left, want 1", len(m.Columns()))
	}
	if m.Left() {
		t.Error("Left succeeded at the entry column; there is nowhere to go")
	}
}

// The chain must never grow a pane the user immediately has to back out of.
func TestRightIsANoOpOnALeaf(t *testing.T) {
	m := New(load(t, "diamond"))
	// app -> a -> shared, and shared is a leaf.
	selectByName(t, m, "app")
	m.Right()
	selectByName(t, m, "a")
	m.Right()
	selectByName(t, m, "shared")

	// The earlier version looped `for m.Right() {}`, which HID the defect: an empty
	// column was appended during the loop, and the final call then failed for the
	// right reason anyway. Assert the column count around a single call on a known
	// leaf instead.
	atLeaf := len(m.Columns())
	if m.Right() {
		t.Error("descended from a leaf")
	}
	if got := len(m.Columns()); got != atLeaf {
		t.Errorf("columns = %d after a failed descent, want %d (an empty pane was appended)", got, atLeaf)
	}
}

// The path is the answer to "what pulled this in?", permanently on screen.
func TestPathIsTheChainOfSelections(t *testing.T) {
	m := New(load(t, "diamond"))
	m.Right()
	p := m.Path()
	if len(p) != 2 {
		t.Fatalf("path = %v, want two entries", p)
	}
	if p[0] != "app" {
		t.Errorf("path starts at %q, want app", p[0])
	}
}

// Direction is a MODE: flipping it reverses the whole view, not one column.
func TestToggleDirectionFlipsTheWholeView(t *testing.T) {
	m := New(load(t, "diamond"))
	if m.Direction() != Forward {
		t.Fatal("did not start forward")
	}
	m.Right() // roots -> a,b
	m.ToggleDirection()
	if m.Direction() != Reverse {
		t.Fatal("direction did not flip")
	}
	// From "a", reverse must reach "app", which depends on it.
	found := false
	for _, n := range m.Active().Visible() {
		if n.Name == "app" {
			found = true
		}
	}
	if !found {
		var got []string
		for _, n := range m.Active().Visible() {
			got = append(got, n.Name)
		}
		t.Errorf("reverse column = %v, want it to contain app", got)
	}
}

func TestDirectionNamesItself(t *testing.T) {
	if Forward.String() != "dependencies" || Reverse.String() != "dependents" {
		t.Error("a direction that cannot name itself cannot label a header")
	}
}

func TestFilterNarrowsTheActiveColumn(t *testing.T) {
	m := New(load(t, "obom-categories"))
	selectByName(t, m, "launchd_services")
	m.Right()
	before := len(m.Active().Visible())
	m.Filter("alpha") // "svc" matches both entries and would not narrow
	after := len(m.Active().Visible())
	if after >= before || after == 0 {
		t.Errorf("filter gave %d of %d entries", after, before)
	}
	m.Filter("")
	if len(m.Active().Visible()) != before {
		t.Error("clearing the filter did not restore the column")
	}
}

func TestFilterIsCaseInsensitive(t *testing.T) {
	m := New(load(t, "obom-categories"))
	selectByName(t, m, "launchd_services")
	m.Right()
	m.Filter("ALPHA")
	if len(m.Active().Visible()) == 0 {
		t.Error("uppercase filter matched nothing")
	}
}

func TestCursorStaysInBoundsWhenFilterShrinksTheColumn(t *testing.T) {
	m := New(load(t, "obom-categories"))
	selectByName(t, m, "launchd_services")
	m.Right()
	for range m.Active().Entries {
		m.Down()
	}
	m.Filter("svc-alpha")
	if _, ok := m.Active().Selected(); !ok {
		t.Error("selection lost after filtering")
	}
}

// No per-category schema: every property, in document order.
func TestDetailShowsEveryPropertyInDocumentOrder(t *testing.T) {
	m := New(load(t, "obom-categories"))
	selectByName(t, m, "certificates")
	m.Right()
	selectByName(t, m, "cert-one")
	var keys []string
	for _, kv := range m.Detail() {
		keys = append(keys, kv.Key)
	}
	for _, want := range []string{"name", "bom-ref", "cdx:osquery:category"} {
		if !slices.Contains(keys, want) {
			t.Errorf("detail missing %q; got %v", want, keys)
		}
	}

	// Compare against the ORDER IN THE FILE rather than two hand-picked keys.
	// Mutation testing showed the earlier version could not detect sorting at all,
	// because the two keys it compared happened to be alphabetical already.
	want := propertyOrderFromFixture(t, "obom-categories", "cert-one")
	if len(want) < 2 {
		t.Fatal("fixture has too few properties for this test to mean anything")
	}
	if slices.IsSorted(want) {
		t.Fatal("fixture properties are alphabetical, so sorting would be undetectable")
	}
	var got []string
	for _, k := range keys {
		if slices.Contains(want, k) {
			got = append(got, k)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("property order = %v, want document order %v", got, want)
	}
}

// propertyOrderFromFixture reads the raw fixture so the expectation cannot drift
// from the file it is checking.
func propertyOrderFromFixture(t *testing.T, fixture, component string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", fixture+".cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components []struct {
			Name       string `json:"name"`
			Properties []struct {
				Name string `json:"name"`
			} `json:"properties"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range doc.Components {
		if c.Name == component {
			out := make([]string, 0, len(c.Properties))
			for _, p := range c.Properties {
				out = append(out, p.Name)
			}
			return out
		}
	}
	t.Fatalf("no component %q in %s", component, fixture)
	return nil
}

func TestDetailOnACategoryCounts(t *testing.T) {
	m := New(load(t, "obom-categories"))
	d := m.Detail()
	if len(d) == 0 || d[0].Key != "category" {
		t.Fatalf("detail for a category = %v", d)
	}
}

func TestDetailReportsBothDirections(t *testing.T) {
	m := New(load(t, "diamond"))
	m.Right()
	var deps, dependents bool
	for _, kv := range m.Detail() {
		if kv.Key == "dependencies" {
			deps = true
		}
		if kv.Key == "dependents" {
			dependents = true
		}
	}
	if !deps || !dependents {
		t.Error("detail must report both edge directions; the reverse one is the useful question")
	}
}

// Cycles must not make navigation unbounded: descending repeatedly is bounded by
// the user, and each step must remain well-formed.
func TestCyclicBOMNavigatesWithoutBlowingUp(t *testing.T) {
	m := New(load(t, "cycle-direct"))
	for i := 0; i < 50 && m.Right(); i++ {
	}
	if len(m.Columns()) < 2 {
		t.Error("could not descend into a cyclic BOM at all")
	}
	if _, ok := m.Active().Selected(); !ok {
		t.Error("selection lost while walking a cycle")
	}
}

func TestEveryFixtureOpensWithoutPanicking(t *testing.T) {
	for _, f := range []string{
		"tree-simple", "diamond", "cycle-direct", "cycle-self", "cycle-deep",
		"cycle-semantics", "rootless", "multi-root", "partial-coverage",
		"no-dependencies-empty", "no-dependencies-absent", "obom-categories",
		"hbom-depth1", "duplicate-names", "purl-less", "dangling-ref",
		"spec-14", "spec-15", "spec-17",
	} {
		t.Run(f, func(t *testing.T) {
			m := New(load(t, f))
			m.Down()
			m.Up()
			m.Right()
			m.Left()
			m.ToggleDirection()
			_ = m.Detail()
			_ = m.Path()
		})
	}
}

// The reported symptom: the column list showed the ref but the header path did not.
func TestPathShowsARefForAnUnnamedComponent(t *testing.T) {
	m := New(load(t, "many-properties"))
	selectByName(t, m, "alf")
	m.Right()
	// the "alf" category holds a named and an unnamed component
	c := m.Active()
	for i, n := range c.Visible() {
		if n.Name == "" {
			c.Cursor = i
		}
	}
	for _, seg := range m.Path() {
		if strings.TrimSpace(seg) == "" {
			t.Errorf("path contains an empty segment: %q", m.Path())
		}
	}
}

func TestFilterMatchesAnUnnamedComponentByItsRef(t *testing.T) {
	m := New(load(t, "many-properties"))
	selectByName(t, m, "alf")
	m.Right()
	m.Filter("unnamed") // part of the ref, not of any name
	if len(m.Active().Visible()) == 0 {
		t.Error("an unnamed component cannot be found by its ref")
	}
}
