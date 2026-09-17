// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package columns

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jrjsmrtn/bomdive/internal/bom"
)

// Mode is which records the whole view navigates (ADR-0009). `v` switches it.
type Mode int

const (
	// ModeComponents is the component axis — the view as it always was.
	ModeComponents Mode = iota
	// ModeVulnerabilities navigates vulnerability records, and the components they affect.
	ModeVulnerabilities
)

// Grouping is how the vulnerability axis's first column groups its records.
type Grouping int

const (
	// GroupNone: neither axis splits the document, so there is no grouping column.
	GroupNone Grouping = iota
	// GroupState groups by analysis state.
	GroupState
	// GroupSeverity groups by the most severe rating each record carries.
	GroupSeverity
)

func (gr Grouping) String() string {
	switch gr {
	case GroupState:
		return "analysis state"
	case GroupSeverity:
		return "severity"
	default:
		return "none"
	}
}

// Group is one first-column entry on the vulnerability axis.
type Group struct {
	Key     string
	Members []bom.Vulnerability
}

const (
	noAnalysis = "(no analysis)"
	noRating   = "(no rating)"
)

// Synthetic entries — rows that are not components — follow the convention the OBOM
// category row set: Ref is EMPTY (a real component always has one: build skips
// components without a bom-ref), Type says which kind it is, and Category carries its
// key. Keying on an empty Ref rather than on Type alone matters, because Type is a
// document's own value and real documents do not keep to the schema.
const (
	typeCategory = "category"
	typeGroup    = "vulnerability group"
	typeVuln     = "vulnerability"
	typeRef      = "reference"
	typeFile     = "file"
)

func isEntry(n bom.Node, kind string) bool { return n.Ref == "" && n.Type == kind }

// isVulnRow reports a row the vulnerability axis owns: a group, a record, or a reference.
func isVulnRow(n bom.Node) bool {
	return isEntry(n, typeVuln) || isEntry(n, typeRef) || isEntry(n, typeGroup)
}

// GroupVulnerabilities applies ADR-0009's amended rule. A column holding one group is
// not an axis, so: analysis state if it gives two or more groups, otherwise severity
// if IT does, otherwise no grouping at all — the principle HasCategories applies to
// OBOM categories. Measured on 169 real public documents: the rule first accepted gave
// one group in 159; this one splits 41 and drops the column for the other 128.
func GroupVulnerabilities(vs []bom.Vulnerability) (Grouping, []Group) {
	if st := groupBy(vs, stateKey, stateRank); len(st) >= 2 {
		return GroupState, st
	}
	if sv := groupBy(vs, severityKey, severityRank); len(sv) >= 2 {
		return GroupSeverity, sv
	}
	return GroupNone, nil
}

// stateKey is the record's state AS WRITTEN — an OpenVEX `under_investigation` is its
// own group, not folded into a CycloneDX value.
func stateKey(v bom.Vulnerability) string {
	if v.State == "" {
		return noAnalysis
	}
	return v.State
}

// stateRank: states the schema defines, then states it does not, then no analysis.
func stateRank(k string) int {
	switch {
	case k == noAnalysis:
		return 2
	case bom.IsKnownState(k):
		return 0
	default:
		return 1
	}
}

// severityKey folds a schema severity to the schema's spelling — MEDIUM is medium — and
// keeps any other value as written.
func severityKey(v bom.Vulnerability) string {
	s := v.TopSeverity()
	if s == "" {
		return noRating
	}
	if l := strings.ToLower(s); bom.SeverityRank(l) < len(bom.SeverityOrder) {
		return l
	}
	return s
}

func severityRank(k string) int {
	if k == noRating {
		return len(bom.SeverityOrder) + 1
	}
	return bom.SeverityRank(k)
}

func groupBy(vs []bom.Vulnerability, key func(bom.Vulnerability) string, rank func(string) int) []Group {
	idx := map[string]int{}
	var out []Group
	for _, v := range vs {
		k := key(v)
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, Group{Key: k})
		}
		out[i].Members = append(out[i].Members, v)
	}
	sort.SliceStable(out, func(a, b int) bool {
		ra, rb := rank(out[a].Key), rank(out[b].Key)
		if ra != rb {
			return ra < rb
		}
		return out[a].Key < out[b].Key
	})
	for i := range out {
		sortVulns(out[i].Members)
	}
	return out
}

// sortVulns puts the most severe first, then orders by id, then by document position.
func sortVulns(vs []bom.Vulnerability) {
	sort.SliceStable(vs, func(a, b int) bool {
		ra, rb := bom.SeverityRank(vs[a].TopSeverity()), bom.SeverityRank(vs[b].TopSeverity())
		if ra != rb {
			return ra < rb
		}
		if vs[a].ID != vs[b].ID {
			return vs[a].ID < vs[b].ID
		}
		return vs[a].Index < vs[b].Index
	})
}

func vulnNode(v bom.Vulnerability) bom.Node {
	return bom.Node{Type: typeVuln, Name: v.Label(), Category: strconv.Itoa(v.Index), Doc: v.Doc}
}

// vulnNodes turns records into rows, most severe first, and makes every row in the
// column TELLABLE APART.
//
// A vulnerability id is not a key. Across 3,294 real public records, 2,169 share their
// id with another record in the same document — one publisher writes a record per
// affected artifact, 1,958 of its 2,057 (POC-11). Shown by id alone they were identical
// rows. So an id that repeats within the column also names what the record affects —
// with the version, since two records can hit two versions of one package — and a row
// that still repeats adds the record's bom-ref, or its position when it has none. A
// unique id stays bare. Same lesson as component names: bom-ref is the only key.
func (m *Model) vulnNodes(vs []bom.Vulnerability) []bom.Node {
	sorted := append([]bom.Vulnerability(nil), vs...)
	sortVulns(sorted)
	out := make([]bom.Node, len(sorted))
	ids := map[string]int{}
	for _, v := range sorted {
		ids[v.Label()]++
	}
	for i, v := range sorted {
		out[i] = vulnNode(v)
		if ids[v.Label()] > 1 {
			out[i].Name = v.Label() + " · " + m.targetLabel(v)
		}
	}
	names := map[string]int{}
	for _, n := range out {
		names[n.Name]++
	}
	for i, v := range sorted {
		if names[out[i].Name] > 1 {
			tag := v.Ref
			if tag == "" {
				tag = "#" + strconv.Itoa(v.Index)
			}
			out[i].Name += " · " + tag
		}
	}
	return out
}

// targetLabel names what a record affects, for telling rows apart. A resolved target
// is its name and version. An unresolved purl is its package NAME — the part that tells
// two packages apart; the whole reference is in the detail pane. Anything else is the
// reference as written.
func (m *Model) targetLabel(v bom.Vulnerability) string {
	if len(v.Affects) == 0 {
		return "(affects nothing)"
	}
	ref := v.Affects[0].Ref
	label := ref
	if n, st := m.set.Doc(v.Doc).Resolve(ref); st == bom.RefResolved {
		label = n.Label()
		if n.Version != "" {
			label += "@" + n.Version
		}
	} else if strings.HasPrefix(ref, "pkg:") {
		label = purlName(ref)
	}
	if extra := len(v.Affects) - 1; extra > 0 {
		label += fmt.Sprintf(" +%d", extra)
	}
	return label
}

// purlName is a purl's name segment: pkg:maven/org.example/lib@1.0?x=y is "lib".
func purlName(p string) string {
	p = strings.TrimPrefix(p, "pkg:")
	for _, sep := range []string{"#", "?", "@"} {
		if i := strings.Index(p, sep); i >= 0 {
			p = p[:i]
		}
	}
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// RowLabel is what a column row shows at a given width.
//
// A vulnerability row that must be shortened keeps its ID WHOLE and trims what follows.
// Every other label keeps its tail (ADR-0007), because a package name is told apart by
// its end — but applied to "CVE-2015-2080 · jetty-http@9.0.7…" that rule cut the ID off
// and left rows differing only in a version. Found dogfooding a 2,057-record feed. The
// ID is the row's identity; what follows only tells it from a neighbour.
func (m *Model) RowLabel(n bom.Node, width int) string {
	v, ok := m.vulnOf(n)
	if !ok {
		return Label(n.Label(), n.Version, width)
	}
	if utf8.RuneCountInString(n.Name) <= width {
		return n.Name
	}
	id := v.Label()
	room := width - utf8.RuneCountInString(id)
	if room < 4 {
		return Truncate(id, width)
	}
	rest := []rune(strings.TrimPrefix(n.Name, id))
	return id + string(rest[:room-1]) + Ellipsis
}

func (m *Model) vulnOf(n bom.Node) (bom.Vulnerability, bool) {
	if !isEntry(n, typeVuln) {
		return bom.Vulnerability{}, false
	}
	i, err := strconv.Atoi(n.Category)
	if err != nil {
		return bom.Vulnerability{}, false
	}
	return m.set.VulnerabilityAt(n.Doc, i)
}

// Mode reports which records the view navigates.
func (m *Model) Mode() Mode { return m.mode }

// HasVulnerabilities reports whether the document carries any vulnerability record.
func (m *Model) HasVulnerabilities() bool { return len(m.set.Vulnerabilities()) > 0 }

// Grouping reports how the vulnerability axis groups THIS document.
func (m *Model) Grouping() Grouping {
	gr, _ := GroupVulnerabilities(m.set.Vulnerabilities())
	return gr
}

// Showing names what the columns list, for the header. The direction is stated because
// the two ways are visually identical, and a view that silently swapped them would lie.
func (m *Model) Showing() string {
	if m.mode == ModeComponents {
		return m.dir.String()
	}
	// Short on purpose: the header gives it the first line, beside the identity, and
	// at 80 columns a longer form wrapped and pushed the path line off the screen.
	if m.vdir == Reverse {
		return "affected → vulnerabilities"
	}
	return "vulnerabilities → affected"
}

// VulnDirection reports which way the vulnerability axis faces. A caller asks this
// rather than comparing Showing's text, which is wording and will change.
func (m *Model) VulnDirection() Direction { return m.vdir }

// ToggleMode switches between the component axis and the vulnerability axis. Coming
// back restores the component chain exactly as it was left, so looking at a
// vulnerability does not cost the reader their place. A no-op, reporting false, on a
// document with no vulnerability records.
func (m *Model) ToggleMode() bool {
	if m.mode == ModeVulnerabilities {
		m.mode = ModeComponents
		m.cols, m.saved = m.saved, nil
		if len(m.cols) == 0 {
			m.cols = []Column{m.entryColumn()}
		}
		return true
	}
	if !m.HasVulnerabilities() {
		return false
	}
	m.saved = m.cols
	m.mode = ModeVulnerabilities
	m.cols = []Column{m.vulnEntryColumn()}
	return true
}

func (m *Model) vulnEntryColumn() Column {
	if m.vdir == Reverse {
		return Column{Title: "affected components", Entries: m.set.AffectedComponents()}
	}
	gr, groups := GroupVulnerabilities(m.set.Vulnerabilities())
	if gr == GroupNone {
		return Column{Title: "vulnerabilities", Entries: m.vulnNodes(m.set.Vulnerabilities())}
	}
	entries := make([]bom.Node, 0, len(groups))
	for _, g := range groups {
		entries = append(entries, bom.Node{Type: typeGroup, Category: g.Key,
			Name: fmt.Sprintf("%s (%d)", g.Key, len(g.Members))})
	}
	return Column{Title: "by " + gr.String(), Entries: entries}
}

// vulnDescendable mirrors vulnChildren without building the level.
func (m *Model) vulnDescendable(n bom.Node) bool {
	switch {
	case isEntry(n, typeGroup):
		return true // a group exists only because it has members
	case isEntry(n, typeVuln):
		v, ok := m.vulnOf(n)
		return ok && len(v.Affects) > 0
	case n.Ref == "":
		return false // a reference that did not resolve, or any other synthetic row
	default:
		return m.set.HasVulnerabilities(bom.NodeKey{Doc: n.Doc, Ref: n.Ref})
	}
}

// vulnChildren is one level down the vulnerability axis. The axis ALTERNATES: a
// vulnerability leads to what it affects, and a component to what affects it — so
// "what else does this vulnerability touch" and "what else is wrong with this
// component" are each one keypress away.
func (m *Model) vulnChildren(n bom.Node) []bom.Node {
	switch {
	case isEntry(n, typeGroup):
		_, groups := GroupVulnerabilities(m.set.Vulnerabilities())
		for _, g := range groups {
			if g.Key == n.Category {
				return m.vulnNodes(g.Members)
			}
		}
		return nil
	case isEntry(n, typeVuln):
		v, ok := m.vulnOf(n)
		if !ok {
			return nil
		}
		var out []bom.Node
		seen := map[bom.NodeKey]bool{}
		for _, a := range v.Affects {
			// Every reference is shown, resolved or not: dropping the unresolved ones
			// would render a partial answer as complete.
			if node, st := m.set.Doc(v.Doc).Resolve(a.Ref); st == bom.RefResolved {
				if k := (bom.NodeKey{Doc: node.Doc, Ref: node.Ref}); !seen[k] {
					seen[k] = true
					out = append(out, node)
				}
			} else {
				out = append(out, bom.Node{Type: typeRef, Name: a.Ref, Category: st.String(), Doc: v.Doc})
			}
		}
		return out
	case n.Ref == "":
		return nil
	default:
		return m.vulnNodes(m.set.VulnerabilitiesAffecting(bom.NodeKey{Doc: n.Doc, Ref: n.Ref}))
	}
}

// toggleVulnDirection flips the vulnerability axis between "vulnerabilities → what they
// affect" and "affected components → their vulnerabilities", keeping the cursor on the
// selection when the other entry column holds it.
func (m *Model) toggleVulnDirection() {
	sel, ok := m.active().Selected()
	if m.vdir == Forward {
		m.vdir = Reverse
	} else {
		m.vdir = Forward
	}
	m.cols = []Column{m.vulnEntryColumn()}
	if !ok {
		return
	}
	for i, e := range m.cols[0].Entries {
		if e.Ref == sel.Ref && e.Doc == sel.Doc && e.Type == sel.Type && e.Category == sel.Category {
			m.cols[0].Cursor = i
			return
		}
	}
}

// vulnDetail fills the detail pane for the vulnerability axis's own rows. It reports
// false for a component, which gets the ordinary component detail.
func (m *Model) vulnDetail(sel bom.Node) ([]KV, bool) {
	switch {
	case isEntry(sel, typeGroup):
		gr, groups := GroupVulnerabilities(m.set.Vulnerabilities())
		n := 0
		for _, g := range groups {
			if g.Key == sel.Category {
				n = len(g.Members)
			}
		}
		kv := []KV{{"group", sel.Category}, {"grouped by", gr.String()}, {"vulnerabilities", itoa(n)}}
		if gr == GroupState && sel.Category != noAnalysis && !bom.IsKnownState(sel.Category) {
			kv = append(kv, KV{"note", "not a CycloneDX analysis state — shown as the document wrote it"})
		}
		return kv, true
	case isEntry(sel, typeRef):
		st := refStateNamed(sel.Category)
		kv := []KV{{"reference", sel.Name}, {"resolves", st.String()}, {"meaning", st.Explain()}}
		if st == bom.RefLinkedAmbiguous {
			var names []string
			for _, d := range m.set.Doc(sel.Doc).LinkTargets(sel.Name) {
				names = append(names, filepath.Base(d.Path()))
			}
			kv = append(kv, KV{"candidates", strings.Join(names, ", ")})
		}
		return kv, true
	case isEntry(sel, typeVuln):
		v, ok := m.vulnOf(sel)
		if !ok {
			return nil, true
		}
		return m.vulnerabilityDetail(v), true
	}
	return nil, false
}

func refStateNamed(name string) bom.RefState {
	for _, st := range bom.RefStates {
		if st.String() == name {
			return st
		}
	}
	return bom.RefNamesNothing
}

// vulnerabilityDetail shows a record AS WRITTEN, and says so where a value is not one
// the schema defines — neither hiding it nor correcting it.
func (m *Model) vulnerabilityDetail(v bom.Vulnerability) []KV {
	kv := []KV{{"id", v.Label()}}
	if m.multi() {
		kv = append(kv, KV{"document", filepath.Base(m.set.Doc(v.Doc).Path())})
	}
	if v.Ref != "" {
		kv = append(kv, KV{"bom-ref", v.Ref})
	}
	if v.Source != "" {
		kv = append(kv, KV{"source", v.Source})
	}
	for _, r := range v.Ratings {
		val := r.Severity
		if r.Score != nil {
			val += " " + strconv.FormatFloat(*r.Score, 'f', -1, 64)
		}
		if r.Method != "" {
			val += " " + r.Method
		}
		if r.Source != "" {
			val += " (" + r.Source + ")"
		}
		// Shown as written, and said so when it is not the schema's spelling: MEDIUM
		// ranks with medium, but the schema only allows the lower-case form.
		if l := strings.ToLower(r.Severity); r.Severity != "" && bom.SeverityRank(l) >= len(bom.SeverityOrder) {
			val += " — not a CycloneDX severity"
		} else if r.Severity != l {
			val += " — the schema spells it " + l
		}
		kv = append(kv, KV{"rating", strings.TrimSpace(val)})
	}
	if len(v.CWEs) > 0 {
		cwes := make([]string, 0, len(v.CWEs))
		for _, c := range v.CWEs {
			cwes = append(cwes, "CWE-"+strconv.Itoa(c))
		}
		kv = append(kv, KV{"cwes", strings.Join(cwes, ", ")})
	}
	if v.State == "" {
		kv = append(kv, KV{"analysis", "none — the document makes no claim about this vulnerability"})
	} else {
		st := v.State
		if !bom.IsKnownState(st) {
			st += " — not a CycloneDX state"
		}
		kv = append(kv, KV{"state", st})
	}
	if v.Justification != "" {
		j := v.Justification
		if !bom.IsKnownJustification(j) {
			j += " — not a CycloneDX justification"
		}
		kv = append(kv, KV{"justification", j})
	}
	if len(v.Responses) > 0 {
		kv = append(kv, KV{"response", strings.Join(v.Responses, ", ")})
	}
	for _, f := range []KV{{"analysis detail", v.AnalysisDetail}, {"description", v.Description},
		{"detail", v.Detail}, {"recommendation", v.Recommendation}} {
		if f.Value != "" {
			kv = append(kv, f)
		}
	}
	resolved := 0
	for _, a := range v.Affects {
		if _, st := m.set.Doc(v.Doc).Resolve(a.Ref); st == bom.RefResolved {
			resolved++
		}
	}
	kv = append(kv, KV{"affects", fmt.Sprintf("%d (%d resolved)", len(v.Affects), resolved)})
	for _, a := range v.Affects {
		_, st := m.set.Doc(v.Doc).Resolve(a.Ref)
		val := a.Ref + " — " + st.String()
		for _, ver := range a.Versions {
			val += fmt.Sprintf("; %s%s %s", ver.Version, ver.Range, ver.Status)
		}
		kv = append(kv, KV{"  target", val})
	}
	return kv
}

// vulnSummary adds a component's vulnerabilities to its detail, on either axis, so the
// component axis answers "is this affected?" without switching.
func (m *Model) vulnSummary(n bom.Node) []KV {
	vs := m.set.VulnerabilitiesAffecting(bom.NodeKey{Doc: n.Doc, Ref: n.Ref})
	if len(vs) == 0 {
		if m.HasVulnerabilities() {
			return []KV{{"vulnerabilities", "0"}}
		}
		return nil
	}
	top := ""
	states := map[string]int{}
	for _, v := range vs {
		if s := v.TopSeverity(); s != "" && (top == "" || bom.SeverityRank(s) < bom.SeverityRank(top)) {
			top = s
		}
		states[stateKey(v)]++
	}
	val := itoa(len(vs))
	if top != "" {
		val += " — most severe: " + top
	}
	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if ra, rb := stateRank(keys[a]), stateRank(keys[b]); ra != rb {
			return ra < rb
		}
		return keys[a] < keys[b]
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, states[k]))
	}
	return []KV{{"vulnerabilities", val}, {"analysis", strings.Join(parts, ", ")}}
}
