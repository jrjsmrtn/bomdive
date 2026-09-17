// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package bom

import (
	"path/filepath"
	"strings"
	"testing"
)

func loadSet(t *testing.T, names ...string) *Set {
	t.Helper()
	paths := make([]string, 0, len(names))
	for _, n := range names {
		paths = append(paths, filepath.Join("..", "..", "testdata", n+".cdx.json"))
	}
	s, err := LoadSet(paths)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// linkStates resolves the first reference of each record in the first document.
func linkStates(s *Set) map[string]RefState {
	out := map[string]RefState{}
	vex := s.Doc(0)
	for _, v := range vex.Vulnerabilities() {
		_, st := vex.Resolve(v.Affects[0].Ref)
		out[v.ID] = st
	}
	return out
}

// What a link resolves to depends on which documents are named with it — the whole
// point of [B]. One table per session.
func TestALinkResolvesAgainstTheDocumentsNamed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		docs  []string
		first RefState // CVE-2024-6001, a link to a component that exists in link-sbom
	}{
		{"alone", []string{"link-vex"}, RefLinkedNotLoaded},
		{"with its SBOM", []string{"link-vex", "link-sbom"}, RefResolved},
		{"with a byte-identical copy too", []string{"link-vex", "link-sbom", "link-sbom-copy"}, RefResolved},
		{"with a different document claiming the serial", []string{"link-vex", "link-sbom", "link-sbom-other"}, RefLinkedAmbiguous},
		{"with the SBOM at another version", []string{"link-vex", "link-sbom-v2"}, RefLinkedVersionDiffers},
		{"with the SBOM named FIRST", []string{"link-sbom", "link-vex"}, RefResolved},
	} {
		s := loadSet(t, tc.docs...)
		vex := s.Doc(0)
		if !strings.HasSuffix(vex.Path(), "link-vex.cdx.json") {
			vex = s.Doc(1)
		}
		v := vex.Vulnerabilities()[0]
		if _, st := vex.Resolve(v.Affects[0].Ref); st != tc.first {
			t.Errorf("%s: %s resolved as %q, want %q", tc.name, v.ID, st, tc.first)
		}
	}
}

// With the SBOM loaded, each link lands where it should: in the OTHER document.
func TestEveryLinkLandsInItsState(t *testing.T) {
	s := loadSet(t, "link-vex", "link-sbom")
	want := map[string]RefState{
		"CVE-2024-6001": RefResolved,
		"CVE-2024-6002": RefResolved,
		"CVE-2024-6003": RefNamesNothing,    // the document is loaded; the component is not in it
		"CVE-2024-6004": RefLinkedNotLoaded, // a document never supplied
		"CVE-2024-6005": RefResolved,        // the SBOM's own subject, which drives no graph
	}
	for id, st := range linkStates(s) {
		if st != want[id] {
			t.Errorf("%s = %q, want %q", id, st, want[id])
		}
	}
	n, _ := s.Doc(0).Resolve(s.Doc(0).Vulnerabilities()[0].Affects[0].Ref)
	if n.Doc != 1 || n.Ref != "pkg:generic/a@1.0.0" {
		t.Errorf("resolved to %+v, want component a of document 1", n)
	}
}

// An ambiguous link names every candidate, so the reader can see why it did not resolve.
func TestAnAmbiguousLinkNamesItsCandidates(t *testing.T) {
	s := loadSet(t, "link-vex", "link-sbom", "link-sbom-other")
	targets := s.Doc(0).LinkTargets(s.Doc(0).Vulnerabilities()[0].Affects[0].Ref)
	if len(targets) != 2 {
		t.Fatalf("%d candidates, want 2", len(targets))
	}
	var names []string
	for _, g := range targets {
		names = append(names, filepath.Base(g.Path()))
	}
	if got := strings.Join(names, " "); got != "link-sbom.cdx.json link-sbom-other.cdx.json" {
		t.Errorf("candidates = %s", got)
	}
}

// Vulnerabilities reach across documents: the VEX's records affect the SBOM's components.
func TestVulnerabilitiesReachAcrossDocuments(t *testing.T) {
	s := loadSet(t, "link-sbom", "link-vex")
	sbom := s.Doc(0)
	var ids []string
	for _, v := range sbom.VulnerabilitiesAffecting("pkg:generic/a@1.0.0") {
		ids = append(ids, v.ID)
		if v.Doc != 1 {
			t.Errorf("%s is recorded as document %d, want 1", v.ID, v.Doc)
		}
	}
	if strings.Join(ids, ",") != "CVE-2024-6001" {
		t.Errorf("a is affected by %v, want CVE-2024-6001 from the VEX", ids)
	}
	if got := len(s.AffectedComponents()); got != 3 {
		t.Errorf("AffectedComponents = %d, want a, b and the SBOM's subject", got)
	}
	if got := s.ReferenceCounts(); got[RefResolved] != 3 || got[RefNamesNothing] != 1 || got[RefLinkedNotLoaded] != 1 {
		t.Errorf("ReferenceCounts = %v", got)
	}
}

// One bom-ref in two documents is two components.
func TestOneRefInTwoDocumentsIsTwoComponents(t *testing.T) {
	s := loadSet(t, "link-sbom", "link-sbom-other")
	a0, _ := s.Doc(0).Node("pkg:generic/a@1.0.0")
	a1, _ := s.Doc(1).Node("pkg:generic/a@1.0.0")
	if a0.Doc != 0 || a1.Doc != 1 {
		t.Errorf("both documents' a are document %d and %d, want 0 and 1", a0.Doc, a1.Doc)
	}
}

// A document that cannot be loaded fails the whole set: a session missing one named
// document would resolve its links as "not loaded" and say nothing about why.
func TestASetFailsWhenOneDocumentCannotBeLoaded(t *testing.T) {
	_, err := LoadSet([]string{
		filepath.Join("..", "..", "testdata", "link-vex.cdx.json"),
		filepath.Join("..", "..", "testdata", "no-such-document.cdx.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "no-such-document") {
		t.Errorf("err = %v, want one naming the missing file", err)
	}
}

// A document's SUBJECT is a target, whether or not it drives a dependency graph. Every
// CISA use-case link points at a product BOM's subject; read through Node's stricter,
// tree-walking rule, all 11 of case 8's links said "names nothing" with both BOMs loaded.
func TestALinkToADocumentsSubjectResolves(t *testing.T) {
	s := loadSet(t, "link-vex", "link-sbom")
	sbom := s.Doc(1)
	if _, isNode := sbom.Node("pkg:generic/app@1.0.0"); isNode {
		t.Fatal("the SBOM's subject drives a graph, so this test proves nothing")
	}
	if n, st := s.Doc(0).Resolve(s.Doc(0).Vulnerabilities()[4].Affects[0].Ref); st != RefResolved || n.Ref != "pkg:generic/app@1.0.0" || n.Doc != 1 {
		t.Errorf("link to the subject = %q %+v, want resolved to document 1's subject", st, n)
	}
}

// And within one document: a standalone VEX naming its own product is not dangling.
func TestAVEXsReferenceToItsOwnProductResolves(t *testing.T) {
	g := fixture(t, "vex-standalone")
	if got := g.ReferenceCounts(); got[RefResolved] != 1 || got[RefNamesNothing] != 0 {
		t.Errorf("ReferenceCounts = %v, want its one reference, to its own product, resolved", got)
	}
}
