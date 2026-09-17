// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

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
	KindSBOM      Kind = "SBOM"

	// The kinds below are CycloneDX documents that are NOT bills of materials:
	// they inventory nothing. See nonBOMKind for how each is recognised.
	KindVEX         Kind = "VEX"
	KindAttestation Kind = "attestation"
	KindDefinitions Kind = "definitions"
)

// IsBOM reports whether this kind of document is a bill of materials — that is,
// whether it inventories anything.
func (k Kind) IsBOM() bool {
	switch k {
	case KindVEX, KindAttestation, KindDefinitions:
		return false
	}
	return true
}

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
	// The payload rule runs BEFORE the metadata check, not inside the switch below.
	// metadata is optional, and a document without it returned early here, so its
	// kind was never decided: 6 of the 25 standalone VEX documents in the public
	// corpora carry no metadata and were still called "SBOM". The root-type rules
	// below can override this, and can only fire when metadata exists — so a
	// declared root type still outranks an absent component list.
	if kind, ok := payloadOf(doc).nonBOMKind(); ok {
		id.Kind = kind
	}
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
	}
	return id
}

// nonBOMKind recognises a CycloneDX document that inventories nothing, from what
// it carries INSTEAD of components. It is the one decision behind both the kind
// on the first line and the sentence ExplainEmpty prints, so the label and the
// explanation cannot disagree.
//
// Components present means a bill of materials, whatever else rides along: VEX
// embedded in an SBOM is still an SBOM, and so is an SBOM carrying attestations.
// Keying on the ABSENCE of components, not on the presence of a payload, is the
// whole rule.
//
// Provenance differs per kind, and the weakest is stated rather than hidden:
//   - VEX: 25 of 137 public corpus documents (POC-10).
//   - attestation: `declarations` only. One sample, from the specification's own
//     conformance suite (valid-attestation-1.6.json), which names the shape.
//   - definitions: `definitions` only. NO sample anywhere, the spec's suite
//     included; recognised from the schema alone.
//
// ORDER matters when a document carries more than one payload and no components.
// It follows provenance, strongest first, so the best-evidenced reading wins. The
// root-type rules in identify are checked before any of this, because a declared
// root type is stronger evidence than an absent component list.
func (c Contents) nonBOMKind() (Kind, bool) {
	if c.Components > 0 {
		return "", false
	}
	switch {
	case c.Vulnerabilities > 0:
		return KindVEX, true
	case c.Declarations:
		return KindAttestation, true
	case c.Definitions:
		return KindDefinitions, true
	}
	return "", false
}
