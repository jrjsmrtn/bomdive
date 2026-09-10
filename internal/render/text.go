package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/jrjsmrtn/lsxbom/internal/bom"
)

// Text renders for a human.
func Text(w io.Writer, r Result, long bool) error {
	fmt.Fprintf(w, "%s\n", r.Identity)
	if r.From != "" {
		fmt.Fprintf(w, "from: %s\n", r.From)
	}
	fmt.Fprintln(w)

	if len(r.Entries) > 0 {
		if r.Command == "tree" {
			writeTree(w, r.Entries, long)
		} else {
			writeList(w, r.Entries, long)
		}
		fmt.Fprintln(w)
	}

	for _, n := range r.Notes {
		fmt.Fprintf(w, "note: %s\n", n)
	}
	// Coverage is printed unconditionally — it is a correctness guarantee, and a
	// guarantee with an exception is a default.
	fmt.Fprintf(w, "coverage: %d of %d components in the dependency graph (%d%%), %d edges\n",
		r.Coverage.InGraph, r.Coverage.Components, r.Coverage.Percent, r.Coverage.Edges)
	if n := len(r.Coverage.Dangling); n > 0 {
		shown, omitted := bom.Coverage{Dangling: r.Coverage.Dangling}.SampleDangling()
		more := ""
		if omitted > 0 {
			more = fmt.Sprintf(", and %d more (--json lists them all)", omitted)
		}
		fmt.Fprintf(w, "warning: %d dependsOn target(s) match no component: %s%s\n",
			n, strings.Join(shown, ", "), more)
	}
	return nil
}

func writeList(w io.Writer, entries []Entry, long bool) {
	for _, e := range entries {
		indent := strings.Repeat("  ", e.Depth)
		if e.Type == "category" {
			fmt.Fprintf(w, "%s%s/  (%d)\n", indent, e.Name, e.Children)
			continue
		}
		fmt.Fprintf(w, "%s%s\n", indent, label(e, long))
	}
}

// writeTree draws the box-glyph tree. Whether an entry is the last child at its
// depth cannot be known from the entry alone, so it is derived by looking ahead:
// an entry is last when the next entry at a depth <= its own is SHALLOWER.
func writeTree(w io.Writer, entries []Entry, long bool) {
	last := make([]bool, len(entries))
	for i, e := range entries {
		last[i] = true
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Depth < e.Depth {
				break
			}
			if entries[j].Depth == e.Depth {
				last[i] = false
				break
			}
		}
	}

	ancestorLast := []bool{}
	for i, e := range entries {
		if e.Depth == 0 {
			ancestorLast = ancestorLast[:0]
			fmt.Fprintf(w, "%s\n", label(e, long)+marker(e))
			continue
		}
		for len(ancestorLast) < e.Depth-1 {
			ancestorLast = append(ancestorLast, true)
		}
		ancestorLast = ancestorLast[:e.Depth-1]

		var b strings.Builder
		for _, al := range ancestorLast {
			if al {
				b.WriteString("    ")
			} else {
				b.WriteString("│   ")
			}
		}
		if last[i] {
			b.WriteString("└── ")
		} else {
			b.WriteString("├── ")
		}
		fmt.Fprintf(w, "%s%s\n", b.String(), label(e, long)+marker(e))
		ancestorLast = append(ancestorLast, last[i])
	}
}

// marker explains why a branch stopped. A cycle and an ordinary shared node are
// distinguished on purpose: a diamond is normal, a loop is worth seeing.
func marker(e Entry) string {
	switch {
	case e.Cycle:
		return "  ↺ cycle"
	case e.Repeat:
		return "  → already shown"
	default:
		return ""
	}
}

func label(e Entry, long bool) string {
	name := e.Label()
	if e.Version != "" {
		name += "@" + e.Version
	}
	if !long {
		return name
	}
	parts := []string{fmt.Sprintf("%-12s", e.Type), name}
	if e.Category != "" {
		parts = append(parts, "["+e.Category+"]")
	}
	if e.PURL != "" {
		parts = append(parts, e.PURL)
	} else {
		parts = append(parts, e.Ref)
	}
	return strings.Join(parts, "  ")
}
