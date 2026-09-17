package columns

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jrjsmrtn/bomdive/internal/bom"
)

// groupKeys returns nil for no groups, so a table can say "nil" and mean "none" —
// reflect.DeepEqual tells an empty slice from a nil one.
func groupKeys(gs []Group) []string {
	var out []string
	for _, g := range gs {
		out = append(out, g.Key)
	}
	return out
}

// ADR-0009's amended rule, on one fixture per outcome. A column holding one group is
// not an axis: state if it splits, else severity if it splits, else no grouping.
func TestGroupingFollowsTheAmendedRule(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		want    Grouping
		keys    []string
	}{
		{"vex-embedded-states", GroupState, []string{"exploitable", "not_affected", "resolved"}},
		{"vex-embedded-untriaged", GroupSeverity, []string{"critical", "high", "medium"}},
		{"vex-uniform", GroupNone, nil},
		// Known states first, then those the schema does not define, as written.
		{"vex-out-of-schema", GroupState, []string{"not_affected", "under_investigation"}},
	} {
		gr, groups := GroupVulnerabilities(load(t, tc.fixture).Vulnerabilities())
		if gr != tc.want {
			t.Errorf("%s: grouping = %v, want %v", tc.fixture, gr, tc.want)
		}
		if got := groupKeys(groups); !reflect.DeepEqual(got, tc.keys) {
			t.Errorf("%s: groups = %v, want %v", tc.fixture, got, tc.keys)
		}
	}
}

// When nothing splits there is no grouping column at all — not a column of one.
func TestASingleGroupIsNeverAColumn(t *testing.T) {
	m := New(load(t, "vex-uniform"))
	if !m.ToggleMode() {
		t.Fatal("ToggleMode refused a document that carries vulnerabilities")
	}
	c := m.Columns()[0]
	if c.Title != "vulnerabilities" || len(c.Entries) != 3 {
		t.Fatalf("entry column = %q with %d entries, want the 3 vulnerabilities", c.Title, len(c.Entries))
	}
	for _, e := range c.Entries {
		if !isEntry(e, typeVuln) {
			t.Errorf("entry %q is a %q, want a vulnerability", e.Name, e.Type)
		}
	}
}

// MEDIUM ranks with medium: a real public source writes severities in capitals.
func TestSeverityIsGroupedRegardlessOfCase(t *testing.T) {
	v := func(i int, sev string) bom.Vulnerability {
		return bom.Vulnerability{Index: i, ID: "V" + itoa(i), Ratings: []bom.Rating{{Severity: sev}}}
	}
	gr, groups := GroupVulnerabilities([]bom.Vulnerability{v(0, "MEDIUM"), v(1, "medium"), v(2, "HIGH")})
	if gr != GroupSeverity {
		t.Fatalf("grouping = %v, want severity", gr)
	}
	if got := groupKeys(groups); !reflect.DeepEqual(got, []string{"high", "medium"}) {
		t.Errorf("groups = %v, want [high medium]", got)
	}
	if len(groups[1].Members) != 2 {
		t.Errorf("medium holds %d, want MEDIUM and medium together", len(groups[1].Members))
	}
}

func titles(m *Model) []string {
	var out []string
	for _, c := range m.Columns() {
		out = append(out, c.Title)
	}
	return out
}

// Looking at a vulnerability must not cost the reader their place.
func TestSwitchingBackRestoresTheComponentChain(t *testing.T) {
	m := New(load(t, "vex-embedded-states"))
	m.Down()
	m.Down()
	before, cursor := titles(m), m.Active().Cursor
	m.ToggleMode()
	if m.Mode() != ModeVulnerabilities || m.Columns()[0].Title != "by analysis state" {
		t.Fatalf("mode %v, entry %q", m.Mode(), m.Columns()[0].Title)
	}
	m.Right()
	m.ToggleMode()
	if m.Mode() != ModeComponents {
		t.Fatal("second toggle did not return to components")
	}
	if got := titles(m); !reflect.DeepEqual(got, before) || m.Active().Cursor != cursor {
		t.Errorf("chain %v cursor %d, want %v cursor %d", got, m.Active().Cursor, before, cursor)
	}
}

func TestSwitchingIsANoOpWithoutVulnerabilities(t *testing.T) {
	m := New(load(t, "tree-simple"))
	if m.ToggleMode() || m.Mode() != ModeComponents {
		t.Error("switched to a vulnerability axis on a document with none")
	}
}

// Every reference is a row, resolved or not, and says which of the five states it is in.
func TestAVulnerabilityLeadsToEverythingItAffects(t *testing.T) {
	g := load(t, "vex-refs")
	m := New(g)
	m.ToggleMode()
	want := map[string]string{
		"CVE-2024-3001": "component:pkg:generic/a@1.0.0",
		"CVE-2024-3002": "component:pkg:npm/b@2.0.0",
		"CVE-2024-3003": "reference:names a package",
		"CVE-2024-3004": "component:pkg:generic/a@1.0.0",
		"CVE-2024-3005": "reference:linked, version differs",
		"CVE-2024-3006": "reference:linked, not loaded",
		"CVE-2024-3007": "reference:names nothing",
		"CVE-2024-3009": "component:pkg:generic/a@1.0.0", // named twice, shown once
	}
	for _, v := range g.Vulnerabilities() {
		n := vulnNode(v)
		kids := m.vulnChildren(n)
		if v.ID == "CVE-2024-3008" {
			if len(kids) != 0 || m.Descendable(n) {
				t.Errorf("a vulnerability with no affects has %d children / descendable", len(kids))
			}
			continue
		}
		if len(kids) != 1 {
			t.Fatalf("%s: %d children, want 1", v.ID, len(kids))
		}
		got := "component:" + kids[0].Ref
		if isEntry(kids[0], typeRef) {
			got = "reference:" + kids[0].Category
		}
		if got != want[v.ID] {
			t.Errorf("%s -> %s, want %s", v.ID, got, want[v.ID])
		}
	}
}

// A component leads to what affects it, most severe first.
func TestAComponentLeadsToWhatAffectsIt(t *testing.T) {
	g := load(t, "vex-embedded-states")
	m := New(g)
	m.ToggleMode()
	a, _ := g.Node("pkg:generic/a@1.0.0")
	var got []string
	for _, n := range m.vulnChildren(a) {
		got = append(got, n.Name)
	}
	if want := []string{"CVE-2024-0001", "CVE-2024-0002"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a -> %v, want %v (critical before high)", got, want)
	}
}

// The arrow must never promise a descent that → refuses, on the vulnerability axis too.
// Walks every row reachable from the entry column, three levels deep, in both directions.
func TestVulnDescendableAgreesWithItsChildren(t *testing.T) {
	for _, fx := range []string{"vex-embedded-states", "vex-embedded-untriaged", "vex-uniform",
		"vex-refs", "vex-out-of-schema", "vex-standalone", "sbom-with-vex"} {
		for _, flip := range []bool{false, true} {
			m := New(load(t, fx))
			if !m.ToggleMode() {
				t.Fatalf("%s: no vulnerability axis", fx)
			}
			if flip {
				m.ToggleDirection()
			}
			level := m.Columns()[0].Entries
			for depth := 0; depth < 3 && len(level) > 0; depth++ {
				var next []bom.Node
				for _, n := range level {
					kids := m.vulnChildren(n)
					if got, want := m.Descendable(n), len(kids) > 0; got != want {
						t.Errorf("%s flip=%v: Descendable(%s %q) = %v, children say %v",
							fx, flip, n.Type, n.Name, got, want)
					}
					next = append(next, kids...)
				}
				level = next
			}
		}
	}
}

// Flipping lists the affected components, and keeps the cursor on the selection.
func TestReverseListsAffectedComponentsAndKeepsTheCursor(t *testing.T) {
	m := New(load(t, "vex-embedded-states"))
	m.ToggleMode()
	selectByName(t, m, "not_affected (2)")
	m.Right()
	selectByName(t, m, "CVE-2024-0002")
	m.Right()
	selectByName(t, m, "b")
	m.ToggleDirection()
	c := m.Columns()[0]
	if c.Title != "affected components" || len(c.Entries) != 3 {
		t.Fatalf("reverse entry = %q with %d entries, want the 3 affected components", c.Title, len(c.Entries))
	}
	if sel, _ := m.Active().Selected(); sel.Name != "b" {
		t.Errorf("cursor on %q after flipping, want b", sel.Name)
	}
	if m.VulnDirection() != Reverse || !strings.HasPrefix(m.Showing(), "affected") {
		t.Errorf("header does not say which way the view faces: %q", m.Showing())
	}
}

func detailValue(kv []KV, key string) string {
	for _, e := range kv {
		if e.Key == key {
			return e.Value
		}
	}
	return ""
}

// Out-of-schema values are shown as written AND said to be so — neither hidden nor
// silently passed off as CycloneDX.
func TestVulnerabilityDetailFlagsValuesOutsideTheSchema(t *testing.T) {
	g := load(t, "vex-out-of-schema")
	m := New(g)
	byID := map[string]bom.Vulnerability{}
	for _, v := range g.Vulnerabilities() {
		byID[v.ID] = v
	}
	if got := detailValue(m.vulnerabilityDetail(byID["CVE-2024-4001"]), "state"); got != "under_investigation — not a CycloneDX state" {
		t.Errorf("4001 state = %q", got)
	}
	if got := detailValue(m.vulnerabilityDetail(byID["CVE-2024-4001"]), "rating"); !strings.Contains(got, "MEDIUM") ||
		!strings.Contains(got, "the schema spells it medium") {
		t.Errorf("4001 rating = %q, want MEDIUM as written and the schema's spelling", got)
	}
	if got := detailValue(m.vulnerabilityDetail(byID["CVE-2024-4002"]), "justification"); !strings.Contains(got, "not a CycloneDX justification") {
		t.Errorf("4002 justification = %q", got)
	}
	if got := detailValue(m.vulnerabilityDetail(byID["CVE-2024-4003"]), "state"); got != "not_affected" {
		t.Errorf("a schema value is flagged: %q", got)
	}
}

func TestComponentDetailSummarisesItsVulnerabilities(t *testing.T) {
	m := New(load(t, "vex-embedded-states"))
	selectByName(t, m, "a")
	kv := m.Detail()
	if got := detailValue(kv, "vulnerabilities"); got != "2 — most severe: critical" {
		t.Errorf("vulnerabilities = %q", got)
	}
	if got := detailValue(kv, "analysis"); got != "exploitable 1, not_affected 1" {
		t.Errorf("analysis = %q", got)
	}
	plain := New(load(t, "tree-simple"))
	if got := detailValue(plain.Detail(), "vulnerabilities"); got != "" {
		t.Errorf("a document with no vulnerabilities shows a count: %q", got)
	}
}

func rowNames(rows []bom.Node) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

// Rows must be tellable apart. Across 3,294 real public records, 2,169 share their id
// with another record in the same document (POC-11); shown by id alone they were
// identical rows — four identical CVE-2015-2080 rows in one column of a real feed.
func TestRowsSharingAnIDAreTellableApart(t *testing.T) {
	m := New(load(t, "vex-shared-ids"))
	if !m.ToggleMode() {
		t.Fatal("no vulnerability axis")
	}
	unique := func(where string, rows []bom.Node) {
		seen := map[string]bool{}
		for _, r := range rows {
			if seen[r.Name] {
				t.Errorf("%s: two rows read %q", where, r.Name)
			}
			seen[r.Name] = true
		}
	}
	entry := m.Columns()[0].Entries
	unique("entry column", entry)
	for _, n := range entry {
		kids := m.vulnChildren(n)
		unique(n.Name, kids)
		for _, k := range kids {
			unique(n.Name+" > "+k.Name, m.vulnChildren(k))
		}
	}
	got := rowNames(entry)
	for _, want := range []string{
		"CVE-2024-5001 · jetty@1.0.0", // same id, told apart by the version it affects
		"CVE-2024-5001 · jetty@2.0.0",
		"CVE-2024-5002",                         // a unique id stays bare
		"CVE-2024-5003 · c@1.0.0 · vuln-5003-a", // same id AND target: the bom-ref decides
		"CVE-2024-5004 · lib-one",               // an unresolved purl is named by its package
		"CVE-2024-5004 · lib-two",
	} {
		found := false
		for _, g := range got {
			found = found || g == want
		}
		if !found {
			t.Errorf("no row reads %q; rows are %v", want, got)
		}
	}
}

// A shortened vulnerability row keeps its ID whole. Tail truncation — right for a
// package name — cut the ID off a live row and left rows differing only in a version.
func TestAShortenedVulnerabilityRowKeepsItsID(t *testing.T) {
	m := New(load(t, "vex-shared-ids"))
	m.ToggleMode()
	for _, w := range []int{16, 20, 24, 30, 60} {
		for _, n := range m.Columns()[0].Entries {
			v, _ := m.vulnOf(n)
			got := m.RowLabel(n, w)
			if utf8.RuneCountInString(got) > w {
				t.Errorf("width %d: %q is %d wide", w, got, utf8.RuneCountInString(got))
			}
			if !strings.HasPrefix(got, v.Label()) {
				t.Errorf("width %d: %q does not start with its id %q", w, got, v.Label())
			}
		}
	}
	// An ordinary component keeps the tail rule ADR-0007 measured for names.
	plain := New(load(t, "tree-simple"))
	for _, n := range plain.Columns()[0].Entries {
		if got, want := plain.RowLabel(n, 8), Label(n.Label(), n.Version, 8); got != want {
			t.Errorf("component row %q, want the tail rule's %q", got, want)
		}
	}
}

func TestAnUnresolvedPurlIsNamedByItsPackage(t *testing.T) {
	for in, want := range map[string]string{
		"pkg:maven/com.fasterxml.jackson.core/jackson-databind": "jackson-databind",
		"pkg:maven/org.example/lib@1.0?type=jar":                "lib",
		"pkg:npm/%40angular/core@17.0.0":                        "core",
		"pkg:golang/github.com/example/mod@v1.2.3#sub/dir":      "mod",
		"pkg:deb/ubuntu/gnupg":                                  "gnupg",
	} {
		if got := purlName(in); got != want {
			t.Errorf("purlName(%q) = %q, want %q", in, got, want)
		}
	}
}
