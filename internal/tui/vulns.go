package tui

import (
	"fmt"
	"strings"

	"github.com/jrjsmrtn/bomdive/internal/bom"
	"github.com/jrjsmrtn/bomdive/internal/columns"
)

// groupLabel is the status bar's word for the grouping — terse, because `?` explains it.
func groupLabel(gr columns.Grouping) string {
	switch gr {
	case columns.GroupState:
		return "by state"
	case columns.GroupSeverity:
		return "by severity"
	default:
		return "ungrouped"
	}
}

// vulnStatusText is the status bar on the vulnerability axis. Its equivalent of
// coverage is how many `affects` references resolved: showing only the resolved half
// without saying so would render a partial answer as complete. Yellow rather than red
// when some did not, because "names a package" and "linked, not loaded" are not
// defects in the document — `?` says which states they are in.
func (u *ui) vulnStatusText() string {
	s := u.model.Set()
	counts := s.ReferenceCounts()
	total := 0
	for _, n := range counts {
		total += n
	}
	affects := fmt.Sprintf("%d/%d", counts[bom.RefResolved], total)
	if counts[bom.RefResolved] < total {
		affects = "[yellow]" + affects + "[-]"
	}
	return fmt.Sprintf("[darkgray]vulns[-] %d  [darkgray]affects[-] %s  [darkgray]%s[-]  [darkgray]"+
		"↑↓ →← nav  ⇞⇟ detail  ⇥flip [-][white]v[-][darkgray]components /filter [-]"+
		"[white]?[-][darkgray]why [-][white]H[-][darkgray]keys q quit[-]",
		len(s.Vulnerabilities()), affects, groupLabel(u.model.Grouping()))
}

// vulnExplainText is `?` on the vulnerability axis: why the leftmost column groups the
// way it does, and what every `affects` reference resolved to.
func (u *ui) vulnExplainText() string {
	g := u.model.Graph()
	vs := u.model.Set().Vulnerabilities()
	var b strings.Builder
	fmt.Fprintf(&b, "[white]%s[-]\n[darkgray]%s[-]\n\n", g.Identity().Describe(), u.sourceText())

	counts := u.model.Set().ReferenceCounts()
	total := 0
	for _, n := range counts {
		total += n
	}
	fmt.Fprintf(&b, "[darkgray]the status bar says[-]  vulns %d  affects %d/%d  %s\n\n",
		len(vs), counts[bom.RefResolved], total, groupLabel(u.model.Grouping()))

	fmt.Fprintf(&b, "[darkgray]this view[-]  %s\n\n", u.vulnAxisText())

	fmt.Fprintf(&b, "[darkgray]affects[-]  the %d vulnerabilities name %d target(s):\n", len(vs), total)
	for _, st := range bom.RefStates {
		if n := counts[st]; n > 0 {
			fmt.Fprintf(&b, "  %d %s — %s\n", n, st, st.Explain())
		}
	}
	b.WriteString("\n[darkgray]press v for the components, H for the keys.[-]\n")
	return b.String()
}

// vulnAxisText says what the leftmost column lists and WHY, because the grouping is
// chosen from the document rather than by the reader.
func (u *ui) vulnAxisText() string {
	if u.model.VulnDirection() == columns.Reverse {
		return "the leftmost column lists every component some vulnerability affects. Descend " +
			"to see what affects it; ⇥ goes back to the vulnerabilities."
	}
	n := len(u.model.Set().Vulnerabilities())
	switch u.model.Grouping() {
	case columns.GroupState:
		return fmt.Sprintf("the leftmost column groups the %d vulnerabilities by analysis state, "+
			"because the states divide them. A state the schema does not define is shown as "+
			"the document wrote it, after the others.", n)
	case columns.GroupSeverity:
		return fmt.Sprintf("the leftmost column groups the %d vulnerabilities by severity — the "+
			"most severe rating each carries — because they all share one analysis state, or "+
			"none, and a column of one group is not an axis. Severities are compared "+
			"regardless of case.", n)
	default:
		return fmt.Sprintf("there is no grouping column: the %d vulnerabilities share one "+
			"analysis state and one severity, so any grouping would hold a single group. They "+
			"are listed most severe first.", n)
	}
}
