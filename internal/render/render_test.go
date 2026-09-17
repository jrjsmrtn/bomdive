package render

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrjsmrtn/bomdive/internal/bom"
)

func load(t *testing.T, name string) *bom.Graph {
	t.Helper()
	return loadFile(t, name+".cdx.json")
}

func loadFile(t *testing.T, file string) *bom.Graph {
	t.Helper()
	g, err := bom.Load(filepath.Join("..", "..", "testdata", file))
	if err != nil {
		t.Fatalf("load %s: %v", file, err)
	}
	return g
}

// fixtureFiles returns the manifest's FILE names, not its fixture names.
//
// Reconstructing "name + .cdx.json" is what these helpers used to do, and it broke
// the moment the corpus gained XML fixtures — the manifest already records the
// real filename, so deriving one is inventing a fact the manifest states.
func fixtureFiles(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("manifest empty; property tests below would pass vacuously")
	}
	files := make([]string, 0, len(m))
	for _, e := range m {
		files = append(files, e.File)
	}
	return files
}

// THE correctness guarantee, as a property over every fixture and both commands.
// ADR-0004 makes coverage a guarantee rather than a flag; an example test would
// only prove it for the cases someone remembered to write.
func TestCoverageAppearsInEveryTextOutput(t *testing.T) {
	for _, name := range fixtureFiles(t) {
		g := loadFile(t, name)
		for _, r := range []Result{
			List(g, name, ListOptions{}),
			Tree(g, name, TreeOptions{}),
		} {
			var buf bytes.Buffer
			if err := Text(&buf, r, false); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "coverage:") {
				t.Errorf("%s/%s: text output has no coverage line", name, r.Command)
			}
		}
	}
}

func TestCoverageAppearsInEveryJSONOutput(t *testing.T) {
	for _, name := range fixtureFiles(t) {
		g := loadFile(t, name)
		for _, r := range []Result{
			List(g, name, ListOptions{}),
			Tree(g, name, TreeOptions{}),
		} {
			var buf bytes.Buffer
			if err := JSON(&buf, r); err != nil {
				t.Fatal(err)
			}
			var back map[string]any
			if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
				t.Fatalf("%s: invalid JSON: %v", name, err)
			}
			cov, ok := back["coverage"].(map[string]any)
			if !ok {
				t.Errorf("%s/%s: JSON has no coverage object", name, r.Command)
				continue
			}
			for _, k := range []string{"components", "in_graph", "percent", "complete"} {
				if _, ok := cov[k]; !ok {
					t.Errorf("%s/%s: coverage lacks %q", name, r.Command, k)
				}
			}
			// A pipeline cannot distinguish null from an empty list without care.
			if back["entries"] == nil {
				t.Errorf("%s/%s: entries is null, want []", name, r.Command)
			}
		}
	}
}

// A graphless BOM must be told apart from a failed walk, in BOTH surfaces.
func TestGraphlessBOMRefusesRatherThanRenderingNothing(t *testing.T) {
	g := load(t, "obom-categories")
	r := Tree(g, "obom", TreeOptions{})
	if len(r.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(r.Entries))
	}
	if !r.DeclaresNoGraph {
		t.Error("DeclaresNoGraph = false on a BOM with dependencies:[]")
	}
	// Assert the DISTINGUISHING claim, not a phrase that happened to be in the
	// wording. "present and EMPTY" is the half that separates this from a document
	// carrying no `dependencies` field at all; a note missing it has lost the point.
	joined := strings.Join(r.Notes, " ")
	if !strings.Contains(joined, "present and EMPTY") {
		t.Errorf("notes do not explain the empty result: %v", r.Notes)
	}
	if !strings.Contains(joined, "--by-category") {
		t.Error("notes do not point at the command that does work")
	}
}

// Absent key and empty array are different claims and must read differently.
func TestAbsentDependenciesReadsDifferentlyFromEmpty(t *testing.T) {
	empty := Tree(load(t, "no-dependencies-empty"), "e", TreeOptions{})
	absent := Tree(load(t, "no-dependencies-absent"), "a", TreeOptions{})
	if !empty.DeclaresNoGraph || absent.DeclaresNoGraph {
		t.Error("the two cases are being conflated")
	}
	if strings.Join(empty.Notes, " ") == strings.Join(absent.Notes, " ") {
		t.Error("both cases produce identical notes; a reader cannot tell them apart")
	}
}

func TestSyntheticRootsAreFlaggedInBothSurfaces(t *testing.T) {
	r := Tree(load(t, "rootless"), "rootless", TreeOptions{})
	if !r.SyntheticRoots {
		t.Fatal("SyntheticRoots = false where the declared root is absent from the graph")
	}
	var text bytes.Buffer
	_ = Text(&text, r, false)
	if !strings.Contains(text.String(), "DERIVED") {
		t.Error("text output does not say the roots were derived")
	}
	var js bytes.Buffer
	_ = JSON(&js, r)
	if !strings.Contains(js.String(), `"synthetic_roots": true`) {
		t.Error("JSON output does not flag synthetic roots")
	}
}

func TestTreeGlyphsMarkRepeatAndCycleDifferently(t *testing.T) {
	var diamond, cycle bytes.Buffer
	_ = Text(&diamond, Tree(load(t, "diamond"), "d", TreeOptions{}), false)
	_ = Text(&cycle, Tree(load(t, "cycle-direct"), "c", TreeOptions{}), false)

	if !strings.Contains(diamond.String(), "already shown") {
		t.Error("diamond: shared node not marked as a back-reference")
	}
	if strings.Contains(diamond.String(), "cycle") {
		t.Error("diamond: a shared node was labelled a cycle; they are different things")
	}
	if !strings.Contains(cycle.String(), "cycle") {
		t.Error("cycle: loop not marked")
	}
}

func TestTreeDrawingIsWellFormed(t *testing.T) {
	var buf bytes.Buffer
	_ = Text(&buf, Tree(load(t, "diamond"), "d", TreeOptions{}), false)
	got := buf.String()
	for _, want := range []string{"├── ", "└── ", "app@1.0.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("tree output missing %q\n%s", want, got)
		}
	}
}

func TestListWorksOnEveryFixture(t *testing.T) {
	for _, name := range fixtureFiles(t) {
		g := loadFile(t, name)
		if r := List(g, name, ListOptions{}); len(r.Entries) == 0 && len(g.Components()) > 0 {
			t.Errorf("%s: ls returned nothing for %d components", name, len(g.Components()))
		}
	}
}

func TestReverseListsDependents(t *testing.T) {
	g := load(t, "diamond")
	var shared string
	for _, n := range g.Components() {
		if n.Name == "shared" {
			shared = n.Ref
		}
	}
	r := List(g, "d", ListOptions{From: shared, Reverse: true})
	if len(r.Entries) != 2 {
		t.Errorf("dependents of shared = %d, want 2 (a and b)", len(r.Entries))
	}
}

func TestUnknownRefIsReportedNotSilentlyEmpty(t *testing.T) {
	r := List(load(t, "diamond"), "d", ListOptions{From: "pkg:generic/nope@9"})
	if len(r.Notes) == 0 {
		t.Error("an unknown ref produced an empty listing with no explanation")
	}
}

// The OBOM navigation path — the one that matters most, per POC-7, and the one
// coverage showed was untested.
func TestByCategoryGroupsAndCounts(t *testing.T) {
	r := List(load(t, "obom-categories"), "o", ListOptions{ByCategory: true})
	var headers, members int
	counts := map[string]int{}
	for _, e := range r.Entries {
		if e.Type == "category" {
			headers++
			counts[e.Name] = e.Children
		} else {
			members++
			if e.Depth != 1 {
				t.Errorf("member %q has depth %d, want 1", e.Name, e.Depth)
			}
		}
	}
	if headers != 4 {
		t.Errorf("category headers = %d, want 4", headers)
	}
	if members != 5 {
		t.Errorf("members = %d, want 5", members)
	}
	if counts["launchd_services"] != 2 {
		t.Errorf("launchd_services count = %d, want 2", counts["launchd_services"])
	}
}

// Categories are DISCOVERED from the document, never hardcoded: the vocabulary is
// platform-dependent, so a fixed list would be wrong on most hosts (POC-7).
func TestByCategoryDiscoversUnknownCategories(t *testing.T) {
	r := List(load(t, "obom-categories"), "o", ListOptions{ByCategory: true})
	want := map[string]bool{
		"certificates": false, "launchd_services": false,
		"listening_ports": false, "systemd_units": false,
	}
	for _, e := range r.Entries {
		if e.Type == "category" {
			if _, known := want[e.Name]; !known {
				t.Errorf("unexpected category %q", e.Name)
			}
			want[e.Name] = true
		}
	}
	for cat, seen := range want {
		if !seen {
			t.Errorf("category %q not discovered", cat)
		}
	}
}

func TestComponentsWithoutACategoryAreGroupedVisibly(t *testing.T) {
	r := List(load(t, "purl-less"), "p", ListOptions{ByCategory: true})
	found := false
	for _, e := range r.Entries {
		if e.Type == "category" && e.Name == "(no category)" {
			found = true
		}
	}
	if !found {
		t.Error("uncategorised components vanished instead of being grouped visibly")
	}
}

func TestTypeFilter(t *testing.T) {
	g := load(t, "hbom-depth1")
	all := List(g, "h", ListOptions{})
	devices := List(g, "h", ListOptions{Type: "device"})
	firmware := List(g, "h", ListOptions{Type: "firmware"})
	if len(devices.Entries) == 0 || len(devices.Entries) >= len(all.Entries) {
		t.Errorf("type filter did not narrow: %d of %d", len(devices.Entries), len(all.Entries))
	}
	if len(firmware.Entries) != 1 {
		t.Errorf("firmware = %d, want 1", len(firmware.Entries))
	}
	for _, e := range devices.Entries {
		if e.Type != "device" {
			t.Errorf("type filter leaked a %q", e.Type)
		}
	}
}

func TestCategoryFilter(t *testing.T) {
	r := List(load(t, "obom-categories"), "o", ListOptions{Category: "launchd_services"})
	if len(r.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(r.Entries))
	}
	for _, e := range r.Entries {
		if e.Category != "launchd_services" {
			t.Errorf("category filter leaked %q", e.Category)
		}
	}
}

// Long format must fall back to bom-ref when there is no purl — 28-40% of a real
// OBOM has none, and showing a blank identifier would be useless.
func TestLongFormatFallsBackToRefWhenNoPurl(t *testing.T) {
	var buf bytes.Buffer
	_ = Text(&buf, List(load(t, "purl-less"), "p", ListOptions{}), true)
	out := buf.String()
	if !strings.Contains(out, "osquery:gatekeeper:data:no-purl-1") {
		t.Errorf("long format did not fall back to bom-ref:\n%s", out)
	}
	if !strings.Contains(out, "pkg:generic/has-purl@1.0.0") {
		t.Error("long format did not show the purl where one exists")
	}
}

func TestLongFormatShowsCategory(t *testing.T) {
	var buf bytes.Buffer
	_ = Text(&buf, List(load(t, "obom-categories"), "o", ListOptions{}), true)
	if !strings.Contains(buf.String(), "[launchd_services]") {
		t.Error("long format omits the category, which is the OBOM navigation axis")
	}
}

func TestByCategoryRendersHeadersInText(t *testing.T) {
	var buf bytes.Buffer
	_ = Text(&buf, List(load(t, "obom-categories"), "o", ListOptions{ByCategory: true}), false)
	if !strings.Contains(buf.String(), "launchd_services/") {
		t.Errorf("category header not rendered:\n%s", buf.String())
	}
}

// EVERY surface must explain the graph state, not just tree.
//
// This is the regression test for the defect that produced it: the absent-versus-
// empty distinction was written out by the tree renderer alone, so `bomdive ls` and
// the column view printed a bare "0%" that a reader could only read as a fault.
// The note now comes from render.New, and this asserts both commands carry it into
// their RENDERED TEXT — not merely into a struct field a renderer may ignore.
func TestBothSurfacesExplainTheGraphState(t *testing.T) {
	for _, fx := range []string{"partial-coverage", "no-dependencies-empty", "no-dependencies-absent"} {
		g := load(t, fx)
		want := g.Coverage().Explain()
		for _, r := range []Result{
			List(g, fx, ListOptions{}),
			Tree(g, fx, TreeOptions{}),
		} {
			var out bytes.Buffer
			if err := Text(&out, r, false); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s %s: rendered text does not explain the graph state.\nwant: %s\ngot:\n%s",
					r.Command, fx, want, out.String())
			}
		}
	}
}

// A complete graph must NOT be annotated: a caveat on every document is noise, and
// noise is how a real caveat stops being read.
func TestACompleteGraphGetsNoStateNote(t *testing.T) {
	g := load(t, "tree-simple")
	r := Tree(g, "tree-simple", TreeOptions{})
	for _, n := range r.Notes {
		if strings.Contains(n, "coverage is") || strings.Contains(n, "dependency graph reaches") {
			t.Errorf("a complete graph carries a state note: %q", n)
		}
	}
	if r.GraphState != "complete" {
		t.Errorf("GraphState = %q, want complete", r.GraphState)
	}
}

// The suggestion must not send a reader to an axis the document does not have.
// --by-category on a document with no categories lands in a single "(no category)"
// bucket, which looks like the tool losing the components.
func TestByCategoryIsSuggestedOnlyWhereItHelps(t *testing.T) {
	withCats := strings.Join(List(load(t, "obom-categories"), "o", ListOptions{}).Notes, " ")
	if !strings.Contains(withCats, "--by-category") {
		t.Error("a document WITH categories is not told about --by-category")
	}
	without := strings.Join(List(load(t, "no-dependencies-empty"), "n", ListOptions{}).Notes, " ")
	if strings.Contains(without, "--by-category") {
		t.Errorf("a document with NO categories is sent to --by-category anyway: %q", without)
	}
}

// An empty document must explain itself on EVERY surface, and must not be handed
// the graph-state note instead — with nothing to inventory there is nothing for a
// graph to cover, so "0 of 0 because `dependencies` is absent" answers a question
// nobody asked.
func TestAnEmptyDocumentExplainsItselfOnBothSurfaces(t *testing.T) {
	for _, fx := range []string{"vex-standalone", "services-only", "metadata-only",
		"attestation-only", "definitions-only"} {
		g := load(t, fx)
		want, _ := g.Contents().ExplainEmpty()
		for _, r := range []Result{
			List(g, fx, ListOptions{}),
			Tree(g, fx, TreeOptions{}),
		} {
			var out bytes.Buffer
			if err := Text(&out, r, false); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s %s: does not explain the empty listing.\nwant: %s\ngot:\n%s",
					r.Command, fx, want, out.String())
			}
			if strings.Contains(out.String(), "because `dependencies`") {
				t.Errorf("%s %s: reports graph state for a document with no components",
					r.Command, fx)
			}
		}
	}
}

// The identity line must not call a VEX an SBOM — it is the first thing printed.
func TestTheIdentityLineNamesAVEX(t *testing.T) {
	var out bytes.Buffer
	if err := Text(&out, List(load(t, "vex-standalone"), "v", ListOptions{}), false); err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(out.String(), "\n", 2)[0]
	if !strings.HasPrefix(first, "VEX") {
		t.Errorf("identity line = %q, want it to start with VEX", first)
	}
}
