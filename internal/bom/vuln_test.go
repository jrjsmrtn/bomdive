package bom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// schemaEnum reads an enum from the cached CycloneDX schema — the specification these
// lists claim to follow. Skipped, loudly, when the cache has not been fetched.
func schemaEnum(t *testing.T, def string) []string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", ".schema-cache", "bom-1.6.schema.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("schema cache absent (%v) — run testdata/validate-schema.py to fetch it", err)
	}
	var s struct {
		Definitions map[string]struct {
			Enum []string `json:"enum"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s.Definitions[def].Enum
}

func TestSeverityOrderIsTheSchemas(t *testing.T) {
	if want := schemaEnum(t, "severity"); !reflect.DeepEqual(SeverityOrder, want) {
		t.Errorf("SeverityOrder = %v, schema says %v", SeverityOrder, want)
	}
}

func TestKnownStatesAreTheSchemas(t *testing.T) {
	if want := schemaEnum(t, "impactAnalysisState"); !reflect.DeepEqual(KnownStates, want) {
		t.Errorf("KnownStates = %v, schema says %v", KnownStates, want)
	}
}

func TestKnownJustificationsAreTheSchemas(t *testing.T) {
	if want := schemaEnum(t, "impactAnalysisJustification"); !reflect.DeepEqual(KnownJustifications, want) {
		t.Errorf("KnownJustifications = %v, schema says %v", KnownJustifications, want)
	}
}

// Every way an `affects` reference can resolve, one vulnerability each, so a mutation
// that merges two states fails here rather than surviving as "roughly right".
func TestEveryReferenceLandsInItsState(t *testing.T) {
	g := fixture(t, "vex-refs")
	want := map[string]RefState{
		"CVE-2024-3001": RefResolved,             // local bom-ref
		"CVE-2024-3002": RefResolved,             // a purl that IS a component's bom-ref
		"CVE-2024-3003": RefNamesPackage,         // a versionless purl naming nothing
		"CVE-2024-3004": RefResolved,             // a BOM-Link back into this document
		"CVE-2024-3005": RefLinkedVersionDiffers, // the same link at another version
		"CVE-2024-3006": RefLinkedNotLoaded,      // a link to a document not supplied
		"CVE-2024-3007": RefNamesNothing,         // dangling
	}
	seen := 0
	for _, v := range g.Vulnerabilities() {
		st, ok := want[v.ID]
		if !ok {
			continue
		}
		seen++
		if len(v.Affects) != 1 {
			t.Fatalf("%s: %d affects, fixture has one", v.ID, len(v.Affects))
		}
		if _, got := g.Resolve(v.Affects[0].Ref); got != st {
			t.Errorf("%s: %q resolved as %q, want %q", v.ID, v.Affects[0].Ref, got, st)
		}
	}
	if seen != len(want) {
		t.Fatalf("checked %d of %d vulnerabilities; the fixture changed under this test", seen, len(want))
	}
}

// A vulnerability that affects nothing is still a record the document makes. 49 real
// public documents carry no `affects` at all (POC-11); dropping them would hide claims.
func TestAVulnerabilityWithNoAffectsIsKept(t *testing.T) {
	g := fixture(t, "vex-refs")
	for _, v := range g.Vulnerabilities() {
		if v.ID == "CVE-2024-3008" {
			if len(v.Affects) != 0 {
				t.Errorf("CVE-2024-3008 has %d affects, want 0", len(v.Affects))
			}
			return
		}
	}
	t.Error("the vulnerability with no affects was dropped")
}

// Values the schema forbids come through AS WRITTEN — neither rejected nor corrected.
func TestOutOfSchemaValuesAreKeptAsWritten(t *testing.T) {
	g := fixture(t, "vex-out-of-schema")
	got := map[string][3]string{}
	for _, v := range g.Vulnerabilities() {
		got[v.ID] = [3]string{v.State, v.Justification, v.TopSeverity()}
	}
	want := map[string][3]string{
		"CVE-2024-4001": {"under_investigation", "", "MEDIUM"},
		"CVE-2024-4002": {"not_affected", "vulnerable_code_not_present", "HIGH"},
		"CVE-2024-4003": {"not_affected", "code_not_reachable", "medium"},
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s = %v, want %v as written", id, got[id], w)
		}
	}
	if IsKnownState("under_investigation") || IsKnownJustification("vulnerable_code_not_present") {
		t.Error("an OpenVEX value is reported as a CycloneDX one")
	}
}

func TestSeverityRankIgnoresCaseAndPutsTheUnknownLast(t *testing.T) {
	if SeverityRank("MEDIUM") != SeverityRank("medium") {
		t.Error("MEDIUM and medium rank differently")
	}
	if !(SeverityRank("critical") < SeverityRank("HIGH") && SeverityRank("HIGH") < SeverityRank("low")) {
		t.Error("severities are not in the schema's order")
	}
	if !(SeverityRank("unknown") < SeverityRank("catastrophic") && SeverityRank("catastrophic") < SeverityRank("")) {
		t.Error("want schema values, then values outside the schema, then no value at all")
	}
}

// The index holds RESOLVED targets only, each vulnerability once per component.
func TestVulnerabilitiesAffectingIndexesResolvedTargets(t *testing.T) {
	g := fixture(t, "vex-embedded-states")
	ids := func(ref string) []string {
		var out []string
		for _, v := range g.VulnerabilitiesAffecting(ref) {
			out = append(out, v.ID)
		}
		return out
	}
	for ref, want := range map[string][]string{
		"pkg:generic/a@1.0.0": {"CVE-2024-0001", "CVE-2024-0002"},
		"pkg:generic/b@1.0.0": {"CVE-2024-0002", "CVE-2024-0004"},
		"pkg:generic/c@1.0.0": {"CVE-2024-0003"},
	} {
		if got := ids(ref); !reflect.DeepEqual(got, want) {
			t.Errorf("%s affected by %v, want %v", ref, got, want)
		}
		if !g.HasVulnerabilities(ref) {
			t.Errorf("HasVulnerabilities(%s) = false", ref)
		}
	}
	if g.HasVulnerabilities("pkg:generic/app@1.0.0") {
		t.Error("the root is affected by nothing, yet HasVulnerabilities says it is")
	}
	if got := len(g.AffectedComponents()); got != 3 {
		t.Errorf("AffectedComponents = %d, want 3", got)
	}
}

// Every reference is counted in exactly one state — the vulnerability axis's coverage.
func TestReferenceCountsCoverEveryReference(t *testing.T) {
	g := fixture(t, "vex-refs")
	got := g.ReferenceCounts()
	want := map[RefState]int{RefResolved: 5, RefNamesPackage: 1, RefLinkedNotLoaded: 1,
		RefLinkedVersionDiffers: 1, RefNamesNothing: 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReferenceCounts = %v, want %v", got, want)
	}
}

// One vulnerability naming the same component twice — by bom-ref and by a BOM-Link back
// into this document — affects it ONCE. Listing it twice would double-count a finding.
func TestAComponentNamedTwiceIsIndexedOnce(t *testing.T) {
	g := fixture(t, "vex-refs")
	n := 0
	for _, v := range g.VulnerabilitiesAffecting("pkg:generic/a@1.0.0") {
		if v.ID == "CVE-2024-3009" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("CVE-2024-3009 is listed %d times against component a, want once", n)
	}
}
