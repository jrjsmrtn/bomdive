// Package bom loads a CycloneDX document and exposes it as a navigable graph.
//
// The traversal API is deliberately ONE LEVEL AT A TIME from an arbitrary node
// (ADR-0006). A column view asks for exactly that; a tree is then a complete walk
// of the same model rather than a second implementation.
//
// Note what "lazy" does and does not mean here. The JSON is parsed in full, because
// that is unavoidable for a JSON document. What is lazy is the TRAVERSAL: no caller
// is ever handed a materialised subtree.
package bom

import cdx "github.com/CycloneDX/cyclonedx-go"

// Kind is what a CycloneDX DOCUMENT is — which is not the same as what kind of BOM
// it is, because not every CycloneDX document is a bill of materials.
//
// `bomFormat: "CycloneDX"` is a format marker, not a claim of BOM-ness: CycloneDX
// also carries VEX, and a standalone VEX inventories nothing. It asserts which
// vulnerabilities affect a product described in another document. This workspace's
// glossary already draws that line — the xBOM family table lists SBOM, HBOM, OBOM,
// CBOM, ML-BOM, SaaSBOM and MBOM, while VEX sits under *adjacent concerns*.
//
// Derived from spec-defined signals rather than sniffed from content — see
// docs/inception/evidence/poc7-bom-identity-2026-09-10.md, where these discriminated
// all 24 real BOMs measured, and poc10-empty-documents-2026-09-11.md for VEX.
type Kind string

const (
	KindOBOM      Kind = "OBOM"
	KindHBOM      Kind = "HBOM"
	KindImageSBOM Kind = "image SBOM"
	// KindVEX is the one value here that is NOT a bill of materials.
	KindVEX  Kind = "VEX"
	KindSBOM Kind = "SBOM"
)

// IsBOM reports whether this kind of document is a bill of materials — that is,
// whether it inventories anything. Everything but VEX does.
func (k Kind) IsBOM() bool { return k != KindVEX }

// Identity records the verdict AND the evidence for it, so a caller can explain
// itself rather than asserting a classification the user cannot check.
type Identity struct {
	Kind Kind

	// Phases holds lifecycle entries using the enum form.
	Phases []string
	// Custom holds lifecycle entries using the {name, description} form.
	//
	// READING ONLY .phase SILENTLY MISCLASSIFIES A BOM. A lifecycle entry is
	// either {"phase": ...} or {"name": ..., "description": ...}, and a real
	// generator in this portfolio uses the latter to record the CISA SBOM type.
	// POC-7's first version had this bug.
	Custom []string

	// RootType is metadata.component.type, empty when there is no root component.
	RootType string
	// RootName is metadata.component.name.
	RootName string
	// RootRef is metadata.component.bom-ref. Frequently NOT a node in the
	// dependency graph — see ADR-0004; do not use it as a traversal entry point.
	RootRef string
	// RootDeclared is false when the document has no metadata.component at all.
	// That is schema-valid: metadata has no required fields. The document simply
	// never says what it is about.
	RootDeclared bool
}

// Describe explains the verdict in one line, naming the evidence.
//
// A document that is not a bill of materials says so HERE, on the first line of
// every surface, because that is the fact which explains everything below it: an
// empty component list, a coverage of 0 of 0, a blank column. Left unsaid, all
// three read as the tool having failed.
func (i Identity) Describe() string {
	if !i.Kind.IsBOM() {
		return string(i.Kind) + " (a CycloneDX document, not a bill of materials: " +
			"it inventories nothing)"
	}
	switch {
	case !i.RootDeclared && len(i.Custom) > 0:
		return string(i.Kind) + " (no root component; custom lifecycle: " + join(i.Custom) + ")"
	case !i.RootDeclared:
		return string(i.Kind) + " (no root component, no lifecycle phase — undeclared)"
	case len(i.Custom) > 0:
		return string(i.Kind) + " (root " + i.RootType + "; custom lifecycle: " + join(i.Custom) + ")"
	default:
		return string(i.Kind) + " (root " + i.RootType + "; phases: " + join(i.Phases) + ")"
	}
}

func join(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	out := s[0]
	for _, x := range s[1:] {
		out += ", " + x
	}
	return out
}

func identify(doc *cdx.BOM) Identity {
	id := Identity{Kind: KindSBOM}
	if doc.Metadata == nil {
		return id
	}
	if doc.Metadata.Lifecycles != nil {
		for _, lc := range *doc.Metadata.Lifecycles {
			if lc.Phase != "" {
				id.Phases = append(id.Phases, string(lc.Phase))
			} else if lc.Name != "" {
				id.Custom = append(id.Custom, lc.Name)
			}
		}
	}
	if c := doc.Metadata.Component; c != nil {
		id.RootDeclared = true
		id.RootType = string(c.Type)
		id.RootRef = c.BOMRef
		id.RootName = c.Name
	}

	ops := false
	for _, p := range id.Phases {
		if p == "operations" {
			ops = true
		}
	}
	switch {
	case id.RootType == "operating-system" && ops:
		id.Kind = KindOBOM
	case id.RootType == "device" && ops:
		id.Kind = KindHBOM
	case id.RootType == "container":
		id.Kind = KindImageSBOM
	case isStandaloneVEX(doc):
		id.Kind = KindVEX
	}
	return id
}

// isStandaloneVEX: vulnerability records and NOTHING to inventory.
//
// CycloneDX carries VEX either embedded in a BOM — `vulnerabilities` alongside
// `components` — or standalone, where the document describes a product held
// elsewhere and asserts only which vulnerabilities affect it. Components present
// therefore means SBOM-with-embedded-VEX and NOT this.
//
// A standalone VEX is a valid CycloneDX document and NOT a bill of materials. It is
// not a degenerate SBOM, an empty one, or a failure to inventory — it is a
// different kind of statement that happens to share the envelope.
//
// Measured over the public corpora (POC-10): the split is clean. 25 of 137
// documents carry vulnerabilities with no components, 97 carry components, and NO
// document carries both. Before this, all 25 were called "SBOM" and drawn as an
// empty component list — the confidently-wrong shape checkIsBOM exists to prevent,
// arrived at one step later.
//
// It is placed LAST in the switch on purpose: a root type is a stronger signal than
// the absence of components, so an OBOM that also carried vulnerabilities stays an
// OBOM.
func isStandaloneVEX(doc *cdx.BOM) bool {
	if doc.Components != nil && len(*doc.Components) > 0 {
		return false
	}
	return doc.Vulnerabilities != nil && len(*doc.Vulnerabilities) > 0
}
