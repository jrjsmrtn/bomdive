package bom

import (
	"encoding/json"
	"os"
	"path/filepath"
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
