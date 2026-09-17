// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package columns

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrjsmrtn/bomdive/internal/bom"
)

func openSet(t *testing.T, names ...string) *Model {
	t.Helper()
	paths := make([]string, 0, len(names))
	for _, n := range names {
		paths = append(paths, filepath.Join("..", "..", "testdata", n+".cdx.json"))
	}
	s, err := bom.LoadSet(paths)
	if err != nil {
		t.Fatal(err)
	}
	return New(s.Doc(0))
}

// Several documents: the leftmost column lists them. One document: nothing changes —
// a column of one file is a column of one entry, which is not an axis.
func TestAFilesColumnOnlyWithSeveralDocuments(t *testing.T) {
	if one := New(load(t, "link-sbom")); one.Axis() == AxisFiles || one.Columns()[0].Title == "files" {
		t.Errorf("a single document opened on a files column")
	}
	m := openSet(t, "link-sbom", "link-vex")
	c := m.Columns()[0]
	if m.Axis() != AxisFiles || c.Title != "files" {
		t.Fatalf("entry column %q, want files", c.Title)
	}
	if got := strings.Join(rowNames(c.Entries), " | "); got != "link-sbom.cdx.json — SBOM | link-vex.cdx.json — VEX" {
		t.Errorf("files = %s", got)
	}
}

// Descending into a file opens its usual first column; a VEX inventories nothing, so
// its file opens on its vulnerability records instead.
func TestDescendingAFileOpensItsFirstColumn(t *testing.T) {
	m := openSet(t, "link-sbom", "link-vex")
	if !m.Right() {
		t.Fatal("could not descend into the SBOM")
	}
	if got := strings.Join(rowNames(m.Active().Entries), " "); got != "a b" {
		t.Errorf("the SBOM opened on %q, want its components a b", got)
	}
	m.Left()
	m.Down()
	// The VEX lists no components, so its file opens on its vulnerability records.
	if !m.Right() || len(m.Active().Entries) != 5 || !isEntry(m.Active().Entries[0], typeVuln) {
		t.Fatalf("the VEX file opened on %v, want its 5 records", rowNames(m.Active().Entries))
	}
	m.Left()
	if !strings.HasSuffix(m.Graph().Path(), "link-vex.cdx.json") {
		t.Errorf("the current document is %q, want the VEX under the cursor", m.Graph().Path())
	}
}

// The SBOM's components show the VEX's vulnerabilities — the reason for loading both.
func TestAComponentShowsVulnerabilitiesFromAnotherDocument(t *testing.T) {
	m := openSet(t, "link-sbom", "link-vex")
	m.Right()
	selectByName(t, m, "a")
	kv := m.Detail()
	if got := detailValue(kv, "vulnerabilities"); got != "1 — most severe: critical" {
		t.Errorf("vulnerabilities = %q, want the VEX's one", got)
	}
	if got := detailValue(kv, "document"); got != "link-sbom.cdx.json" {
		t.Errorf("document = %q", got)
	}
}

// The vulnerability axis spans every document, whichever order they are named in.
func TestTheVulnerabilityAxisSpansEveryDocument(t *testing.T) {
	m := openSet(t, "link-sbom", "link-vex")
	if !m.ToggleMode() {
		t.Fatal("v found no vulnerabilities, though the second document has four")
	}
	var names []string
	var walk func(rows []bom.Node, depth int)
	walk = func(rows []bom.Node, depth int) {
		for _, n := range rows {
			names = append(names, n.Name)
			if depth < 2 {
				walk(m.vulnChildren(n), depth+1)
			}
		}
	}
	walk(m.Columns()[0].Entries, 0)
	if all := strings.Join(names, " "); !strings.Contains(all, "CVE-2024-6001") {
		t.Errorf("the VEX's records are missing from the axis: %v", names)
	}
	g := m.Set().Doc(1)
	for _, v := range g.Vulnerabilities() {
		if v.ID != "CVE-2024-6001" {
			continue
		}
		kids := m.vulnChildren(vulnNode(v))
		if len(kids) != 1 || kids[0].Doc != 0 || kids[0].Name != "a" {
			t.Errorf("CVE-2024-6001 -> %+v, want component a of the SBOM (document 0)", kids)
		}
	}
}

// An ambiguous link says so, and names every file that claims its serial and version.
func TestAnAmbiguousReferenceNamesItsCandidates(t *testing.T) {
	m := openSet(t, "link-vex", "link-sbom", "link-sbom-other")
	m.ToggleMode()
	for _, v := range m.Set().Doc(0).Vulnerabilities() {
		if v.ID != "CVE-2024-6001" {
			continue
		}
		kids := m.vulnChildren(vulnNode(v))
		if len(kids) != 1 || kids[0].Category != "linked, ambiguous" {
			t.Fatalf("CVE-2024-6001 -> %+v, want one ambiguous reference", kids)
		}
		kv, _ := m.vulnDetail(kids[0])
		if got := detailValue(kv, "candidates"); got != "link-sbom.cdx.json, link-sbom-other.cdx.json" {
			t.Errorf("candidates = %q", got)
		}
		return
	}
	t.Fatal("CVE-2024-6001 not found")
}

// The arrow never promises what → refuses, across documents, on both axes.
func TestDescendableAgreesAcrossDocuments(t *testing.T) {
	for _, docs := range [][]string{
		{"link-sbom", "link-vex"}, {"link-vex", "link-sbom", "link-sbom-other"}, {"vex-embedded-states", "link-vex"},
	} {
		for _, vulns := range []bool{false, true} {
			m := openSet(t, docs...)
			if vulns && !m.ToggleMode() {
				t.Fatalf("%v: no vulnerability axis", docs)
			}
			level := m.Columns()[0].Entries
			for depth := 0; depth < 3 && len(level) > 0; depth++ {
				var next []bom.Node
				for _, n := range level {
					kids := m.childrenOf(n)
					if got, want := m.Descendable(n), len(kids) > 0; got != want {
						t.Errorf("%v vulns=%v: Descendable(%s %q) = %v, children say %v",
							docs, vulns, n.Type, n.Name, got, want)
					}
					next = append(next, kids...)
				}
				level = next
			}
		}
	}
}

// A file with no components opens on what it DOES carry — records, else its subject,
// else nothing. Opening on the empty component list made every file of CISA case 8 a
// dead end: its VEX and both product BOMs list no components.
func TestAFileWithNoComponentsOpensOnWhatItCarries(t *testing.T) {
	m := openSet(t, "vex-standalone", "metadata-only", "attestation-only")
	want := []struct {
		title string
		rows  string
	}{
		{"vulnerabilities", "CVE-2020-25649"}, // a standalone VEX: its records
		{"subject", "just-a-name"},            // a product BOM naming only its product
		{"", ""},                              // neither: no arrow, nothing to open
	}
	files := m.Columns()[0].Entries
	for i, w := range want {
		f := files[i]
		if got := m.Descendable(f); got != (w.title != "") {
			t.Errorf("%s: descendable = %v", f.Name, got)
			continue
		}
		if w.title == "" {
			continue
		}
		c := m.entryColumnOf(m.graphOf(f))
		if c.Title != w.title || strings.Join(rowNames(c.Entries), " ") != w.rows {
			t.Errorf("%s opens on %q %v, want %q %s", f.Name, c.Title, rowNames(c.Entries), w.title, w.rows)
		}
	}
}

// From a product BOM's subject, the detail shows what other files' links attribute to it.
func TestASubjectShowsTheVulnerabilitiesLinkedToIt(t *testing.T) {
	m := openSet(t, "link-vex", "link-sbom")
	subject, ok := m.Set().Doc(1).Subject()
	if !ok {
		t.Fatal("link-sbom declares no subject")
	}
	if got := detailValue(m.vulnSummary(subject), "vulnerabilities"); got != "1 — most severe: high" {
		t.Errorf("subject's vulnerabilities = %q, want the one link-vex attributes to it", got)
	}
}

// In the component view, a record row descends to what it affects — the same as on
// the vulnerability axis — and its detail is the record's.
func TestARecordRowWorksInTheComponentView(t *testing.T) {
	m := openSet(t, "link-vex", "link-sbom")
	m.Right() // into the VEX file: its records
	selectByName(t, m, "CVE-2024-6001")
	if kv := m.Detail(); detailValue(kv, "id") != "CVE-2024-6001" {
		t.Errorf("detail of a record row in the component view = %v", kv)
	}
	if !m.Right() || strings.Join(rowNames(m.Active().Entries), " ") != "a" {
		t.Errorf("CVE-2024-6001 -> %v, want component a", rowNames(m.Active().Entries))
	}
}
