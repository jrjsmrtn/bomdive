package bom

import (
	"strconv"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// Vulnerability is one record from `vulnerabilities`, kept AS WRITTEN.
//
// Real VEX does not keep to the schema (POC-11): OpenVEX states and justifications,
// severities in capitals. cyclonedx-go decodes them unchanged, and so does this.
// Normalising them here would put words in the document's mouth; rejecting them would
// hide its claim. Interpretation — ranking a severity, choosing a group — happens where
// it is needed, on top of the raw value.
type Vulnerability struct {
	// Index is the record's position in the document. It is the stable key, because
	// `bom-ref` is optional on a vulnerability.
	Index int

	Ref, ID, Source                     string
	Ratings                             []Rating
	CWEs                                []int
	Description, Detail, Recommendation string

	// The analysis, as written. State is "" when the record makes no claim.
	State, Justification, AnalysisDetail string
	Responses                            []string

	Affects []Affect
}

// Rating is one severity rating, as written.
type Rating struct {
	Severity, Method, Source, Vector string
	Score                            *float64
}

// Affect is one `affects` entry: a reference, and the versions it names.
type Affect struct {
	Ref      string
	Versions []AffectedVersion
}

// AffectedVersion is one `affects[].versions` entry.
type AffectedVersion struct{ Version, Range, Status string }

// SeverityOrder is the schema's own severity enum, most severe first.
// TestSeverityOrderIsTheSchemas pins it to the cached schema file, so it cannot drift
// from the specification it claims to follow.
var SeverityOrder = []string{"critical", "high", "medium", "low", "info", "none", "unknown"}

// KnownStates is the schema's analysis-state enum. TestKnownStatesAreTheSchemas pins it.
var KnownStates = []string{"resolved", "resolved_with_pedigree", "exploitable", "in_triage",
	"false_positive", "not_affected"}

// KnownJustifications is the schema's analysis-justification enum.
// TestKnownJustificationsAreTheSchemas pins it.
var KnownJustifications = []string{"code_not_present", "code_not_reachable", "requires_configuration",
	"requires_dependency", "requires_environment", "protected_by_compiler", "protected_at_runtime",
	"protected_at_perimeter", "protected_by_mitigating_control"}

// IsKnownJustification reports whether a justification is one the schema defines.
func IsKnownJustification(s string) bool {
	for _, k := range KnownJustifications {
		if k == s {
			return true
		}
	}
	return false
}

// SeverityRank orders a severity: schema values in the schema's order, compared
// REGARDLESS OF CASE because real documents write MEDIUM and HIGH; any other value
// after them; no value at all last.
func SeverityRank(s string) int {
	if s == "" {
		return len(SeverityOrder) + 1
	}
	l := strings.ToLower(s)
	for i, k := range SeverityOrder {
		if k == l {
			return i
		}
	}
	return len(SeverityOrder)
}

// IsKnownState reports whether a state is one the schema defines.
func IsKnownState(s string) bool {
	for _, k := range KnownStates {
		if k == s {
			return true
		}
	}
	return false
}

// TopSeverity is the most severe rating a vulnerability carries, as written, or "".
func (v Vulnerability) TopSeverity() string {
	best := ""
	for _, r := range v.Ratings {
		if r.Severity != "" && (best == "" || SeverityRank(r.Severity) < SeverityRank(best)) {
			best = r.Severity
		}
	}
	return best
}

// Label is what a column shows: the id, or "(no id)".
func (v Vulnerability) Label() string {
	if v.ID != "" {
		return v.ID
	}
	return "(no id)"
}

// RefState is what an `affects` reference names (ADR-0009 section 2). Every reference
// is in exactly one state, and none is dropped: a view that showed only the resolved
// ones would render a partial answer as complete.
type RefState int

const (
	// RefResolved names a component in this document.
	RefResolved RefState = iota
	// RefLinkedNotLoaded is a BOM-Link into a document that was not supplied. Not an
	// error: the VEX correctly points elsewhere.
	RefLinkedNotLoaded
	// RefLinkedVersionDiffers is a BOM-Link to this document's serial number at a
	// different version. Not resolved — the view does not guess.
	RefLinkedVersionDiffers
	// RefNamesNothing is a reference that matches nothing: dangling.
	RefNamesNothing
	// RefNamesPackage is a purl that names no component here. It identifies a PACKAGE,
	// not a missing component, so it is not dangling. All 686 references in a measured
	// public VEX feed are of this form, none with a version.
	RefNamesPackage
)

// RefStates lists every state, in the order a report should give them.
var RefStates = []RefState{RefResolved, RefNamesPackage, RefLinkedNotLoaded,
	RefLinkedVersionDiffers, RefNamesNothing}

func (s RefState) String() string {
	switch s {
	case RefResolved:
		return "resolved"
	case RefLinkedNotLoaded:
		return "linked, not loaded"
	case RefLinkedVersionDiffers:
		return "linked, version differs"
	case RefNamesPackage:
		return "names a package"
	default:
		return "names nothing"
	}
}

// Explain is the sentence the detail pane gives for a reference that did not resolve.
func (s RefState) Explain() string {
	switch s {
	case RefResolved:
		return "names a component in this document"
	case RefLinkedNotLoaded:
		return "a BOM-Link into another document, which was not supplied — the reference is " +
			"correct, lsxbom was just not given the document it points to"
	case RefLinkedVersionDiffers:
		return "a BOM-Link to this document's serial number at a different version — not " +
			"resolved, because a different version is a different document"
	case RefNamesPackage:
		return "a purl that names no component here — it identifies a package, not a " +
			"missing component, so it is not dangling"
	default:
		return "matches nothing in this document — dangling"
	}
}

// Resolve says what an `affects` reference names.
//
// A purl can resolve: a document may use a component's purl as its bom-ref, which a
// measured public feed does for all 2,057 of its references. Only a purl that names
// NO component is RefNamesPackage.
func (g *Graph) Resolve(ref string) (Node, RefState) {
	if n, ok := g.Node(ref); ok {
		return n, RefResolved
	}
	if rest, isLink := strings.CutPrefix(ref, "urn:cdx:"); isLink {
		body, fragment, _ := strings.Cut(rest, "#")
		serial, version, _ := strings.Cut(body, "/")
		if g.serial == "" || !strings.EqualFold(serial, g.serial) {
			return Node{}, RefLinkedNotLoaded
		}
		// A BOM-Link back into THIS document resolves here, at the version it names.
		if version != strconv.Itoa(g.version) {
			return Node{}, RefLinkedVersionDiffers
		}
		if n, ok := g.Node(fragment); ok {
			return n, RefResolved
		}
		return Node{}, RefNamesNothing
	}
	if strings.HasPrefix(ref, "pkg:") {
		return Node{}, RefNamesPackage
	}
	return Node{}, RefNamesNothing
}

// Vulnerabilities returns every vulnerability record, in document order.
func (g *Graph) Vulnerabilities() []Vulnerability { return g.vulns }

// VulnerabilityAt returns the record at a document index.
func (g *Graph) VulnerabilityAt(i int) (Vulnerability, bool) {
	if i < 0 || i >= len(g.vulns) {
		return Vulnerability{}, false
	}
	return g.vulns[i], true
}

// VulnerabilitiesAffecting returns the records whose `affects` resolves to a
// component, in document order.
func (g *Graph) VulnerabilitiesAffecting(ref string) []Vulnerability {
	idx := g.affectedBy[ref]
	out := make([]Vulnerability, 0, len(idx))
	for _, i := range idx {
		out = append(out, g.vulns[i])
	}
	return out
}

// HasVulnerabilities reports whether any record affects a component — without
// building the list, because the view asks once per visible row.
func (g *Graph) HasVulnerabilities(ref string) bool { return len(g.affectedBy[ref]) > 0 }

// AffectedComponents returns every component some vulnerability affects, in document
// order.
func (g *Graph) AffectedComponents() []Node {
	var out []Node
	for _, ref := range g.order {
		if len(g.affectedBy[ref]) > 0 {
			out = append(out, g.nodes[ref])
		}
	}
	// The declared root is not in g.order; a vulnerability may still affect it.
	if root := g.id.RootRef; root != "" && len(g.affectedBy[root]) > 0 {
		if _, inInventory := g.nodes[root]; !inInventory {
			if n, ok := g.Node(root); ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// ReferenceCounts tallies every `affects` reference by state — the vulnerability
// axis's equivalent of coverage, reported so a partial answer is never shown as a
// complete one.
func (g *Graph) ReferenceCounts() map[RefState]int {
	out := map[RefState]int{}
	for _, v := range g.vulns {
		for _, a := range v.Affects {
			_, st := g.Resolve(a.Ref)
			out[st]++
		}
	}
	return out
}

// loadVulnerabilities reads the records and indexes which components each affects.
// It runs LAST in build: Resolve needs the node set, the edges (a declared root only
// resolves once it is known to drive the graph) and the document's serial number.
func (g *Graph) loadVulnerabilities(doc *cdx.BOM) {
	if doc.Vulnerabilities == nil {
		return
	}
	g.affectedBy = map[string][]int{}
	for i, v := range *doc.Vulnerabilities {
		x := Vulnerability{Index: i, Ref: v.BOMRef, ID: v.ID, Description: v.Description,
			Detail: v.Detail, Recommendation: v.Recommendation}
		if v.Source != nil {
			x.Source = v.Source.Name
		}
		if v.Ratings != nil {
			for _, r := range *v.Ratings {
				rt := Rating{Severity: string(r.Severity), Method: string(r.Method), Vector: r.Vector, Score: r.Score}
				if r.Source != nil {
					rt.Source = r.Source.Name
				}
				x.Ratings = append(x.Ratings, rt)
			}
		}
		if v.CWEs != nil {
			x.CWEs = append(x.CWEs, *v.CWEs...)
		}
		if a := v.Analysis; a != nil {
			x.State, x.Justification, x.AnalysisDetail = string(a.State), string(a.Justification), a.Detail
			if a.Response != nil {
				for _, r := range *a.Response {
					x.Responses = append(x.Responses, string(r))
				}
			}
		}
		if v.Affects != nil {
			indexed := map[string]bool{}
			for _, af := range *v.Affects {
				e := Affect{Ref: af.Ref}
				if af.Range != nil {
					for _, rv := range *af.Range {
						e.Versions = append(e.Versions, AffectedVersion{rv.Version, rv.Range, string(rv.Status)})
					}
				}
				x.Affects = append(x.Affects, e)
				if n, st := g.Resolve(af.Ref); st == RefResolved && !indexed[n.Ref] {
					indexed[n.Ref] = true
					g.affectedBy[n.Ref] = append(g.affectedBy[n.Ref], i)
				}
			}
		}
		g.vulns = append(g.vulns, x)
	}
}
