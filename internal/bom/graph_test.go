package bom

import (
	"encoding/json"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture loads a committed fixture by name. The corpus is generated and every
// fixture's shape is asserted by testdata/verify.py, so a test failing here means
// the CODE is wrong — the fixture has already been checked against its claim.
func fixture(t *testing.T, name string) *Graph {
	t.Helper()
	g, err := Load(filepath.Join("..", "..", "testdata", name+".cdx.json"))
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return g
}

func TestIdentity(t *testing.T) {
	for _, tc := range []struct {
		fixture      string
		want         Kind
		rootDeclared bool
		rootType     string
	}{
		{"obom-categories", KindOBOM, true, "operating-system"},
		{"no-dependencies-empty", KindOBOM, true, "operating-system"},
		{"hbom-depth1", KindHBOM, true, "device"},
		{"tree-simple", KindSBOM, true, "library"},
		{"rootless", KindSBOM, true, "file"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			id := fixture(t, tc.fixture).Identity()
			if id.Kind != tc.want {
				t.Errorf("Kind = %q, want %q (%s)", id.Kind, tc.want, id.Describe())
			}
			if id.RootDeclared != tc.rootDeclared {
				t.Errorf("RootDeclared = %v, want %v", id.RootDeclared, tc.rootDeclared)
			}
			if id.RootType != tc.rootType {
				t.Errorf("RootType = %q, want %q", id.RootType, tc.rootType)
			}
		})
	}
}

// A lifecycle entry is either {"phase":...} or {"name":...}. Reading only .phase
// misclassifies the second form — the bug POC-7's first version had.
func TestIdentityCustomLifecycleForm(t *testing.T) {
	doc := `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1,
	  "metadata":{"lifecycles":[{"name":"Deployed","description":"CISA SBOM type"}]},
	  "components":[{"bom-ref":"a","type":"library","name":"a"}]}`
	path := filepath.Join(t.TempDir(), "custom.cdx.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	id := g.Identity()
	if len(id.Custom) != 1 || id.Custom[0] != "Deployed" {
		t.Errorf("Custom = %v, want [Deployed] — .phase-only reading loses this", id.Custom)
	}
	if id.RootDeclared {
		t.Error("RootDeclared = true, want false: this document has no metadata.component")
	}
}

// ADR-0004: the declared root is frequently NOT a node in the graph. Walking from
// it reaches nothing, so roots must be derived and labelled synthetic.
func TestRootsDerivedWhenDeclaredRootIsAbsent(t *testing.T) {
	g := fixture(t, "rootless")
	if !g.Identity().RootDeclared {
		t.Fatal("fixture should declare a root")
	}
	if kids := g.Children(g.Identity().RootRef); len(kids) != 0 {
		t.Fatalf("declared root should reach nothing, got %d", len(kids))
	}
	roots, synthetic := g.Roots()
	if !synthetic {
		t.Error("synthetic = false; a caller would present an invented root as declared")
	}
	if len(roots) == 0 {
		t.Error("no roots derived, so nothing could be rendered")
	}
}

func TestRootsNotSyntheticWhenDeclaredRootIsInGraph(t *testing.T) {
	g := fixture(t, "tree-simple")
	roots, synthetic := g.Roots()
	if synthetic {
		t.Error("synthetic = true, but the declared root IS in the graph")
	}
	if len(roots) != 1 || roots[0].Name != "app" {
		t.Errorf("roots = %v, want [app]", roots)
	}
}

func TestMultipleDerivedRoots(t *testing.T) {
	roots, synthetic := fixture(t, "multi-root").Roots()
	if !synthetic || len(roots) != 3 {
		t.Errorf("roots = %d (synthetic=%v), want 3 synthetic", len(roots), synthetic)
	}
}

// The distinction a tool must not lose: an empty `dependencies` is the document
// asserting there are no relations; an absent one is the generator saying nothing.
func TestDeclaresNoGraphVersusAbsent(t *testing.T) {
	empty := fixture(t, "no-dependencies-empty")
	if !empty.HasDependenciesKey() || !empty.DeclaresNoGraph() {
		t.Error("empty array should be: key present, declares no graph")
	}
	absent := fixture(t, "no-dependencies-absent")
	if absent.HasDependenciesKey() {
		t.Error("absent key reported as present — the two claims are different")
	}
}

func TestCoverage(t *testing.T) {
	c := fixture(t, "partial-coverage").Coverage()
	if c.Components != 10 || c.InGraph != 3 {
		t.Errorf("coverage = %d/%d, want 3/10", c.InGraph, c.Components)
	}
	if c.Complete() {
		t.Error("Complete() = true on a 3-of-10 graph — this is the confidently-wrong case")
	}
}

// Referential integrity was perfect in every real BOM measured. This is the error
// path: report it, never invent a node, never crash.
func TestDanglingRefIsReportedNotInvented(t *testing.T) {
	g := fixture(t, "dangling-ref")
	c := g.Coverage()
	if len(c.Dangling) != 1 {
		t.Fatalf("Dangling = %v, want exactly one", c.Dangling)
	}
	// A missing map key yields a ZERO Node — empty Ref, empty Name — so checking
	// for the dangling ref itself never matches. Mutation testing caught that:
	// materialising dangling targets slipped past the original assertion.
	kids := g.Children("pkg:generic/a@1.0.0")
	if len(kids) != 0 {
		t.Errorf("Children = %d, want 0: the only target is dangling", len(kids))
	}
	for _, n := range kids {
		if n.Ref == "" {
			t.Error("a phantom zero-value Node was emitted for a dangling target")
		}
	}
}

func TestIdentityKeyedOnRefNotName(t *testing.T) {
	g := fixture(t, "duplicate-names")
	names := map[string]int{}
	for _, n := range g.Components() {
		names[n.Name]++
	}
	if names["dup"] != 3 {
		t.Fatalf("expected 3 components named dup, got %d", names["dup"])
	}
	if got := len(g.Components()); got != 4 {
		t.Errorf("Components() = %d, want 4 — name-keyed storage would collapse these", got)
	}
}

func TestPurlLessComponentsSurvive(t *testing.T) {
	n := 0
	for _, c := range fixture(t, "purl-less").Components() {
		if c.PURL == "" {
			n++
		}
	}
	if n != 3 {
		t.Errorf("purl-less components = %d, want 3 — purl-keyed storage drops these", n)
	}
}

func TestCategoriesAreTheOBOMAxis(t *testing.T) {
	cats := fixture(t, "obom-categories").Categories()
	if len(cats) != 4 {
		t.Errorf("categories = %d, want 4", len(cats))
	}
	if len(cats["launchd_services"]) != 2 {
		t.Errorf("launchd_services = %d, want 2", len(cats["launchd_services"]))
	}
}

func TestSpecVersionsAllParse(t *testing.T) {
	for _, f := range []string{"spec-14", "spec-15", "spec-17"} {
		t.Run(f, func(t *testing.T) {
			if got := len(fixture(t, f).Components()); got != 2 {
				t.Errorf("components = %d, want 2", got)
			}
		})
	}
}

// Every committed fixture must load. Guards against a fixture being added that the
// parser cannot handle, which would otherwise go unnoticed until someone used it.
func TestEveryFixtureLoads(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join("..", "..", "testdata", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("manifest is empty; this test would pass vacuously")
	}
	for name, entry := range m {
		if _, err := Load(filepath.Join("..", "..", "testdata", entry.File)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Pointing at the wrong file must be an error, not a plausible empty BOM. Before
// this check, valid-JSON-but-not-a-BOM rendered "0 of 0 components" and exit 0.
func TestNonBOMIsRejectedNotRenderedEmpty(t *testing.T) {
	for name, body := range map[string]string{
		"not a bom":      `{"hello":"world"}`,
		"wrong format":   `{"bomFormat":"SPDX","specVersion":"1.6","version":1}`,
		"no specVersion": `{"bomFormat":"CycloneDX","version":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Error("loaded without error; a wrong file would render as an empty BOM")
			}
		})
	}
}

// XML is a second ENCODING, not a second format: the same model with identity on
// the root element's namespace instead of a bomFormat field. A guard written
// against JSON silently rejects every valid XML BOM, which is what happened until
// syft's corpus surfaced it.
func TestXMLIsParsed(t *testing.T) {
	g, err := Load(filepath.Join("..", "..", "testdata", "xml-simple.cdx.xml"))
	if err != nil {
		t.Fatalf("XML not parsed: %v", err)
	}
	if got := len(g.Components()); got != 2 {
		t.Errorf("components = %d, want 2", got)
	}
	if roots, synthetic := g.Roots(); len(roots) != 1 || synthetic {
		t.Errorf("roots = %d synthetic=%v, want 1 declared", len(roots), synthetic)
	}
}

// CycloneDX 1.1 has no metadata element at all — metadata.component arrived in
// 1.2 — and CycloneDX JSON does not exist below 1.2, so only an XML fixture can
// express this. A reader assuming a root component exists breaks here.
func TestXMLWithoutMetadataElement(t *testing.T) {
	g, err := Load(filepath.Join("..", "..", "testdata", "xml-no-metadata.cdx.xml"))
	if err != nil {
		t.Fatalf("1.1 XML not parsed: %v", err)
	}
	if g.Identity().RootDeclared {
		t.Error("RootDeclared = true for a 1.1 document, which has no metadata element")
	}
	if got := len(g.Components()); got != 2 {
		t.Errorf("components = %d, want 2", got)
	}
}

func TestFormatIsDetectedFromContentNotExtension(t *testing.T) {
	xml, err := os.ReadFile(filepath.Join("..", "..", "testdata", "xml-simple.cdx.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bom.json", "bom.txt", "bom"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(p, xml, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err != nil {
				t.Errorf("XML content named %q was not parsed: %v", name, err)
			}
		})
	}
}

func TestUTF8ByteOrderMarkIsTolerated(t *testing.T) {
	for _, fx := range []string{"diamond.cdx.json", "xml-simple.cdx.xml"} {
		t.Run(fx, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", fx))
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(t.TempDir(), fx)
			if err := os.WriteFile(p, append([]byte{0xEF, 0xBB, 0xBF}, raw...), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err != nil {
				t.Errorf("a UTF-8 BOM broke parsing: %v", err)
			}
		})
	}
}

func TestNonBOMXMLIsRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.xml")
	if err := os.WriteFile(p, []byte(`<?xml version="1.0"?><html><body/></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Error("non-BOM XML loaded without error")
	}
}

func TestUnrecognisedContentIsRejectedByName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(p, []byte("just some text"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("plain text loaded without error")
	}
	if !strings.Contains(err.Error(), "not JSON or XML") {
		t.Errorf("error does not name the cause: %v", err)
	}
}

// The case the XML namespace guard actually exists for.
//
// A <bom> element in a foreign namespace DECODES cleanly — the decoder only
// objects to a different element name, as with <html>. Without the namespace
// check it would render as a CycloneDX BOM, which is the confidently-wrong shape.
// Worth its own test because the obvious mutation (disabling the check) fails to
// COMPILE, and a build error proves nothing.
func TestXMLInAForeignNamespaceIsRejected(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<bom xmlns="http://example.com/not-cyclonedx" version="1">
  <components><component type="library"><name>a</name><version>1.0.0</version></component></components>
</bom>`
	p := filepath.Join(t.TempDir(), "x.xml")
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("a <bom> in a foreign namespace loaded as a CycloneDX document")
	}
	if !strings.Contains(err.Error(), "not a CycloneDX document") {
		t.Errorf("error does not name the cause: %v", err)
	}
}

// A declared root that drives the graph but is absent from `components`.
//
// Legitimate: metadata.component is the SUBJECT of the document, not a member of
// its inventory. Found by POC-8 on a real merged host view, where it left a
// document carrying 33 edges with no roots at all — and fixing only Roots() left
// the tree rooted and EMPTY, which is worse, because it looks like an answer.
func TestDeclaredRootOutsideComponentsStillRoots(t *testing.T) {
	g := fixture(t, "root-outside-components")
	if _, listed := g.Node("host:machine-1"); !listed {
		t.Fatal("the declared root does not resolve to a node")
	}
	roots, synthetic := g.Roots()
	if len(roots) != 1 {
		t.Fatalf("roots = %d, want 1", len(roots))
	}
	if synthetic {
		t.Error("synthetic = true: this root is DECLARED, not derived")
	}
	if roots[0].Name != "the-host" {
		t.Errorf("root name = %q, want the-host", roots[0].Name)
	}

	// The half that fixing Roots() alone would have missed.
	var visits int
	g.Walk(roots[0].Ref, 0, func(Visit) bool { visits++; return true })
	if visits < 4 {
		t.Errorf("walk emitted %d visits from a root with 2 children and a grandchild", visits)
	}
}

// A component with no name must never render as nothing.
//
// Fixed once in the view and still broken in four other places: the column title,
// the path, ls output ("@1.0.0") and tree output (a bare "├──"). Label() is the
// single place that decides, and this asserts it rather than any one caller.
func TestLabelFallsBackToRef(t *testing.T) {
	for _, tc := range []struct {
		n    Node
		want string
	}{
		{Node{Name: "a", Ref: "r"}, "a"},
		{Node{Name: "", Ref: "r"}, "r"},
		{Node{Name: "", Ref: ""}, "(unnamed)"},
	} {
		if got := tc.n.Label(); got != tc.want {
			t.Errorf("Label(%q,%q) = %q, want %q", tc.n.Name, tc.n.Ref, got, tc.want)
		}
	}
}

func TestUnnamedComponentInAFixtureHasALabel(t *testing.T) {
	g := fixture(t, "many-properties")
	found := false
	for _, n := range g.Components() {
		if n.Name == "" {
			found = true
			if n.Label() == "" {
				t.Error("an unnamed component labels as empty")
			}
			if n.Label() != n.Ref {
				t.Errorf("Label = %q, want the ref %q", n.Label(), n.Ref)
			}
		}
	}
	if !found {
		t.Fatal("fixture has no unnamed component; this test would pass vacuously")
	}
}

// The four shapes must be four states. Three of them report 0% coverage and mean
// different things, and two were reported identically before this existed: a BOM
// with no `dependencies` field was labelled the same as one whose declared graph
// misses components. That is the difference between "the document is silent" and
// "the document is deficient", and no surface can restore it once it is lost here.
func TestGraphStateSeparatesTheFourShapes(t *testing.T) {
	want := map[string]GraphState{
		"tree-simple":            GraphComplete,
		"partial-coverage":       GraphPartial,
		"no-dependencies-empty":  GraphDeclaredEmpty,
		"no-dependencies-absent": GraphUndeclared,
	}
	seen := map[string]string{} // explanation -> fixture that produced it
	for name, wantState := range want {
		c := fixture(t, name).Coverage()
		if got := c.State(); got != wantState {
			t.Errorf("%s: state = %v, want %v", name, got, wantState)
		}
		if other, dup := seen[c.Explain()]; dup {
			t.Errorf("%s and %s produce the SAME explanation; a reader cannot tell "+
				"them apart: %s", name, other, c.Explain())
		}
		seen[c.Explain()] = name
	}
}

// The terse label is what the status bar shows, so it must also separate the
// states — a shared word there defeats the distinction above.
func TestGraphStateLabelsAreDistinct(t *testing.T) {
	seen := map[string]GraphState{}
	for _, s := range []GraphState{GraphComplete, GraphPartial, GraphDeclaredEmpty, GraphUndeclared} {
		l := s.Label()
		if other, dup := seen[l]; dup {
			t.Errorf("states %v and %v share the label %q", s, other, l)
		}
		seen[l] = s
	}
	if GraphComplete.Label() != "" {
		t.Errorf("the unremarkable state should have no label, got %q", GraphComplete.Label())
	}
}

// HasCategories must be false for a document whose components carry none, or the
// column view offers a category axis that is one bucket holding everything.
func TestHasCategoriesIsFalseWithoutAny(t *testing.T) {
	if fixture(t, "no-dependencies-empty").HasCategories() {
		t.Error("HasCategories = true on a document with no cdx:osquery:category")
	}
	if !fixture(t, "obom-categories").HasCategories() {
		t.Error("HasCategories = false on the OBOM fixture")
	}
}

// The declared root must be visible to REVERSE edges, not only to Node.
//
// metadata.component is the SUBJECT of a document, not a member of its inventory,
// so it is routinely absent from `components`. Node synthesised it (POC-8) but
// resolve read g.nodes directly, so every direct dependency of such a root reported
// NO dependents while `dependencies` plainly declared one. Measured on a public
// 837-component npm SBOM: 71 components affected, every one of them.
func TestTheDeclaredRootIsVisibleToReverseEdges(t *testing.T) {
	g := fixture(t, "root-outside-components")
	root := g.Identity().RootRef
	if _, inComponents := g.nodes[root]; inComponents {
		t.Fatal("fixture's root IS in components; this test proves nothing")
	}

	child := "pkg:generic/nic-0@1.0.0"
	parents := g.Parents(child)
	if len(parents) != 1 || parents[0].Ref != root {
		t.Errorf("Parents(%s) = %v, want the declared root %s", child, parents, root)
	}
	if !g.HasParents(child) {
		t.Error("HasParents = false where Parents returned the root")
	}
	// A component the root does NOT depend on must be unaffected.
	if ps := g.Parents("pkg:generic/svc-a@1.0.0"); len(ps) != 1 || ps[0].Ref == root {
		t.Errorf("Parents(svc-a) = %v, want only its real parent", ps)
	}
}

// The root is not inventory, so it must not be counted as covered — that would put
// InGraph above Components and make a fully covered document report incomplete.
func TestTheDeclaredRootIsNotCountedAsAComponent(t *testing.T) {
	g := fixture(t, "root-outside-components")
	c := g.Coverage()
	if c.Components != 3 {
		t.Errorf("Components = %d, want 3 — the root is not inventory", c.Components)
	}
	if c.InGraph > c.Components {
		t.Errorf("InGraph %d exceeds Components %d", c.InGraph, c.Components)
	}
	if !c.Complete() {
		t.Errorf("coverage %d/%d is not complete; every component IS in the graph",
			c.InGraph, c.Components)
	}
}

// A declared root that does not drive the graph must NOT be resolvable — otherwise
// any ref equal to it would resolve to a node the document never placed anywhere.
func TestARootThatDrivesNothingDoesNotResolve(t *testing.T) {
	g := fixture(t, "rootless")
	root := g.Identity().RootRef
	if root == "" {
		t.Skip("fixture declares no root")
	}
	if _, drives := g.out[root]; drives {
		t.Fatal("fixture's root DOES drive the graph; this test proves nothing")
	}
	if _, ok := g.Node(root); ok {
		t.Errorf("a root driving nothing resolved to %q", root)
	}
}

// The human-readable surfaces name a few dangling refs, not all of them.
func TestDanglingIsSampledNotDumped(t *testing.T) {
	long := make([]string, DanglingSample+7)
	for i := range long {
		long[i] = "ref"
	}
	shown, omitted := Coverage{Dangling: long}.SampleDangling()
	if len(shown) != DanglingSample || omitted != 7 {
		t.Errorf("shown=%d omitted=%d, want %d and 7", len(shown), omitted, DanglingSample)
	}
	short := []string{"a", "b"}
	shown, omitted = Coverage{Dangling: short}.SampleDangling()
	if len(shown) != 2 || omitted != 0 {
		t.Errorf("a short list was sampled: shown=%d omitted=%d", len(shown), omitted)
	}
}

// A document with vulnerabilities and no components is a VEX, not an SBOM.
//
// It was called an SBOM and drawn as an empty component list — "0 of 0 (0%)" over a
// blank pane, which reads as the tool having failed. 25 of 137 public corpus
// documents are this shape (POC-10).
func TestAStandaloneVEXIsNotCalledAnSBOM(t *testing.T) {
	for name, want := range map[string]Kind{
		"vex-standalone":   KindVEX,
		"attestation-only": KindAttestation,
		"definitions-only": KindDefinitions,
		"sbom-with-vex":    KindSBOM, // components present: embedded, not standalone
		"services-only":    KindSBOM,
		"metadata-only":    KindSBOM,
		"tree-simple":      KindSBOM,
		"obom-categories":  KindOBOM,
	} {
		if got := fixture(t, name).Identity().Kind; got != want {
			t.Errorf("%s: Kind = %q, want %q", name, got, want)
		}
	}
}

// A root type is stronger evidence than the ABSENCE of components, so a document
// that declares itself an operating-system inventory stays an OBOM even when it
// carries vulnerability records and lists nothing. The VEX rule is deliberately
// last in the switch; this pins that order.
func TestRootTypeOutranksTheVEXRule(t *testing.T) {
	os := cdx.ComponentTypeOS
	doc := &cdx.BOM{
		BOMFormat: "CycloneDX",
		Metadata: &cdx.Metadata{
			Component:  &cdx.Component{BOMRef: "host", Type: os, Name: "a-host"},
			Lifecycles: &[]cdx.Lifecycle{{Phase: cdx.LifecyclePhaseOperations}},
		},
		Vulnerabilities: &[]cdx.Vulnerability{{ID: "CVE-2020-0000"}},
	}
	if got := identify(doc).Kind; got != KindOBOM {
		t.Errorf("Kind = %q, want OBOM — the root type must outrank the VEX rule", got)
	}
	// Remove the OBOM evidence and the same document IS a VEX.
	doc.Metadata.Lifecycles = nil
	doc.Metadata.Component.Type = cdx.ComponentTypeApplication
	if got := identify(doc).Kind; got != KindVEX {
		t.Errorf("Kind = %q, want VEX once the OBOM evidence is gone", got)
	}
}

// Every document with no components must say WHY, and the three empty shapes must
// read differently — a VEX, a services-only document and a metadata-only one all
// list nothing, and only one of them means "wrong tool for this file".
func TestEveryEmptyDocumentExplainsItselfDistinctly(t *testing.T) {
	seen := map[string]string{}
	for _, name := range []string{"vex-standalone", "services-only", "metadata-only",
		"attestation-only", "definitions-only"} {
		g := fixture(t, name)
		msg, empty := g.Contents().ExplainEmpty()
		if !empty {
			t.Errorf("%s: reports components when it has none", name)
			continue
		}
		if other, dup := seen[msg]; dup {
			t.Errorf("%s and %s explain themselves identically: %s", name, other, msg)
		}
		seen[msg] = name
	}
	// And a document WITH components must not claim to be empty.
	if _, empty := fixture(t, "tree-simple").Contents().ExplainEmpty(); empty {
		t.Error("a populated document reports itself empty")
	}
}

// Contents counts what it says it counts.
func TestContentsCountsVulnerabilitiesAndServices(t *testing.T) {
	for name, want := range map[string]Contents{
		"vex-standalone":   {Components: 0, Vulnerabilities: 1, Services: 0},
		"services-only":    {Components: 0, Vulnerabilities: 0, Services: 2},
		"metadata-only":    {Components: 0, Vulnerabilities: 0, Services: 0},
		"sbom-with-vex":    {Components: 2, Vulnerabilities: 1, Services: 0},
		"attestation-only": {Declarations: true, Attestations: 1},
		"definitions-only": {Definitions: true, Standards: 1},
	} {
		if got := fixture(t, name).Contents(); got != want {
			t.Errorf("%s: Contents = %+v, want %+v", name, got, want)
		}
	}
}

// The first line of every surface must say a VEX is not a bill of materials — it is
// the fact that explains the empty listing, the 0-of-0 coverage and the blank
// column below it. Unsaid, all three read as the tool having failed.
func TestDescribeSaysWhenADocumentIsNotABOM(t *testing.T) {
	vex := fixture(t, "vex-standalone").Identity()
	if vex.Kind.IsBOM() {
		t.Fatal("VEX reports itself as a bill of materials")
	}
	got := vex.Describe()
	if !strings.Contains(got, "not a bill of materials") {
		t.Errorf("Describe = %q, does not say it is not a BOM", got)
	}

	// And a real BOM must NOT carry the disclaimer.
	for _, name := range []string{"tree-simple", "obom-categories", "sbom-with-vex", "metadata-only"} {
		id := fixture(t, name).Identity()
		if !id.Kind.IsBOM() {
			t.Errorf("%s: reports itself as not a BOM", name)
		}
		if strings.Contains(id.Describe(), "not a bill of materials") {
			t.Errorf("%s: Describe carries the VEX disclaimer: %q", name, id.Describe())
		}
	}
}

// metadata is OPTIONAL, so a document without it must still get a kind. identify
// returned early on a nil metadata before deciding one, and 6 of the 25 standalone
// VEX documents in the public corpora — every one with no metadata — were called
// "SBOM" by the release that introduced the VEX rule.
func TestADocumentWithNoMetadataStillGetsItsKind(t *testing.T) {
	doc := &cdx.BOM{
		BOMFormat:       "CycloneDX",
		Vulnerabilities: &[]cdx.Vulnerability{{ID: "CVE-2020-0000"}},
	}
	if doc.Metadata != nil {
		t.Fatal("the document has metadata; this test proves nothing")
	}
	if got := identify(doc).Kind; got != KindVEX {
		t.Errorf("Kind = %q, want VEX for a document with no metadata", got)
	}
}

// Every non-BOM kind says so on the first line, and the explanation below it names
// the SAME kind — the label and the sentence come from one decision.
func TestEachNonBOMKindIsLabelledAndExplainedConsistently(t *testing.T) {
	for name, want := range map[string]struct {
		kind  Kind
		fixed string // a phrase only this kind's explanation contains
	}{
		"vex-standalone":   {KindVEX, "standalone VEX"},
		"attestation-only": {KindAttestation, "It is an attestation"},
		"definitions-only": {KindDefinitions, "DEFINES 1 standard(s)"},
	} {
		g := fixture(t, name)
		id := g.Identity()
		if id.Kind != want.kind {
			t.Errorf("%s: Kind = %q, want %q", name, id.Kind, want.kind)
		}
		if id.Kind.IsBOM() {
			t.Errorf("%s: %q reports itself as a bill of materials", name, id.Kind)
		}
		if !strings.Contains(id.Describe(), "not a bill of materials") {
			t.Errorf("%s: first line does not say it is not a BOM: %q", name, id.Describe())
		}
		msg, _ := g.Contents().ExplainEmpty()
		if !strings.Contains(msg, want.fixed) {
			t.Errorf("%s: explanation does not name its kind (%q):\n%s", name, want.fixed, msg)
		}
	}
}

// When a document carries several payloads and no components, the best-evidenced
// reading wins: VEX (25 corpus samples) over attestation (one spec sample) over
// definitions (none).
func TestSeveralPayloadsResolveByProvenance(t *testing.T) {
	both := Contents{Vulnerabilities: 1, Declarations: true, Definitions: true}
	if got, _ := both.nonBOMKind(); got != KindVEX {
		t.Errorf("vulnerabilities+declarations+definitions = %q, want VEX", got)
	}
	decl := Contents{Declarations: true, Definitions: true}
	if got, _ := decl.nonBOMKind(); got != KindAttestation {
		t.Errorf("declarations+definitions = %q, want attestation", got)
	}
	// And components present means an inventory, whatever rides along.
	inv := Contents{Components: 1, Vulnerabilities: 1, Declarations: true, Definitions: true}
	if got, ok := inv.nonBOMKind(); ok {
		t.Errorf("a document with components was classified as %q", got)
	}
}
