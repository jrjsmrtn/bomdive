package bom

import (
	"strconv"
	"strings"
)

// Set is every document named in one session. BOM-Links resolve among them
// (ADR-0009 [B]): a VEX names components in an SBOM by serial number, version and
// bom-ref, and only when that SBOM is loaded can the link lead anywhere.
//
// A document loaded alone is a set of one, so the single-document path is the same
// code, not a special case beside it.
type Set struct {
	docs []*Graph
	// affected indexes which records affect each component, ACROSS documents. It is
	// built here rather than per graph because a link can point into a document
	// loaded after the one holding it.
	affected map[NodeKey][]VulnKey
}

// NodeKey identifies a component across documents: a bom-ref is unique only within
// its own document.
type NodeKey struct {
	Doc int
	Ref string
}

// VulnKey identifies a vulnerability record across documents.
type VulnKey struct{ Doc, Index int }

// NewSet gathers graphs into one session, in the order given.
func NewSet(gs ...*Graph) *Set {
	s := &Set{docs: gs}
	for i, g := range gs {
		g.doc, g.set = i, s
		for ref, n := range g.nodes {
			n.Doc = i
			g.nodes[ref] = n
		}
		for k := range g.vulns {
			g.vulns[k].Doc = i
		}
	}
	s.reindex()
	return s
}

// LoadSet loads every path, in order, into one session.
func LoadSet(paths []string) (*Set, error) {
	gs := make([]*Graph, 0, len(paths))
	for _, p := range paths {
		g, err := Load(p)
		if err != nil {
			return nil, err
		}
		gs = append(gs, g)
	}
	return NewSet(gs...), nil
}

// Docs returns the loaded documents, in the order they were named.
func (s *Set) Docs() []*Graph { return s.docs }

// Doc returns one loaded document.
func (s *Set) Doc(i int) *Graph { return s.docs[i] }

// Len is how many documents are loaded.
func (s *Set) Len() int { return len(s.docs) }

func (s *Set) reindex() {
	s.affected = map[NodeKey][]VulnKey{}
	for _, g := range s.docs {
		for _, v := range g.vulns {
			indexed := map[NodeKey]bool{}
			for _, a := range v.Affects {
				n, st := g.Resolve(a.Ref)
				if st != RefResolved {
					continue
				}
				// One record naming one component twice — by bom-ref and by a BOM-Link
				// — affects it ONCE.
				k := NodeKey{n.Doc, n.Ref}
				if !indexed[k] {
					indexed[k] = true
					s.affected[k] = append(s.affected[k], VulnKey{g.doc, v.Index})
				}
			}
		}
	}
}

// linkTargets is every loaded document a BOM-Link's serial and version could mean,
// with byte-identical copies counted once — the CISA use cases reuse one product BOM
// across cases, and two files holding one document are not a conflict. loaded reports
// whether the serial is loaded at all, at any version.
func (s *Set) linkTargets(serial, version string) (targets []*Graph, loaded bool) {
	for _, d := range s.docs {
		if d.serial == "" || !strings.EqualFold(d.serial, serial) {
			continue
		}
		loaded = true
		if strconv.Itoa(d.version) != version {
			continue
		}
		copyOfOne := false
		for _, t := range targets {
			copyOfOne = copyOfOne || t.digest == d.digest
		}
		if !copyOfOne {
			targets = append(targets, d)
		}
	}
	return targets, loaded
}

// Vulnerabilities returns every record in every document, in the order the documents
// were named and then in document order.
func (s *Set) Vulnerabilities() []Vulnerability {
	var out []Vulnerability
	for _, g := range s.docs {
		out = append(out, g.vulns...)
	}
	return out
}

// VulnerabilityAt returns one record by document and index.
func (s *Set) VulnerabilityAt(doc, i int) (Vulnerability, bool) {
	if doc < 0 || doc >= len(s.docs) {
		return Vulnerability{}, false
	}
	return s.docs[doc].VulnerabilityAt(i)
}

// VulnerabilitiesAffecting returns the records, from any document, affecting a component.
func (s *Set) VulnerabilitiesAffecting(k NodeKey) []Vulnerability {
	keys := s.affected[k]
	out := make([]Vulnerability, 0, len(keys))
	for _, vk := range keys {
		out = append(out, s.docs[vk.Doc].vulns[vk.Index])
	}
	return out
}

// HasVulnerabilities reports whether any record affects a component.
func (s *Set) HasVulnerabilities(k NodeKey) bool { return len(s.affected[k]) > 0 }

// AffectedComponents returns every component, in any document, that some record affects.
func (s *Set) AffectedComponents() []Node {
	var out []Node
	for _, g := range s.docs {
		out = append(out, g.AffectedComponents()...)
	}
	return out
}

// ReferenceCounts tallies every `affects` reference in every document by state.
func (s *Set) ReferenceCounts() map[RefState]int {
	out := map[RefState]int{}
	for _, g := range s.docs {
		for st, n := range g.ReferenceCounts() {
			out[st] += n
		}
	}
	return out
}

// Path is the file a document was loaded from; empty when it was built in memory.
func (g *Graph) Path() string { return g.path }

// Set is the session a document belongs to.
func (g *Graph) Set() *Set { return g.session() }

// Doc is the document's position in its session.
func (g *Graph) Doc() int { return g.doc }

// session never returns nil: a graph built without Load still belongs to a set of one.
func (g *Graph) session() *Set {
	if g.set == nil {
		NewSet(g)
	}
	return g.set
}

// Serial is the document's serial number without its urn:uuid: prefix; empty when it has none.
func (g *Graph) Serial() string { return g.serial }

// Version is the document's version.
func (g *Graph) Version() int { return g.version }
