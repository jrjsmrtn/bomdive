package columns

import "unicode/utf8"

// Ellipsis marks a truncated label. Deliberately the single-rune form: a column is
// tight, and three dots cost three of the characters being fought over.
const Ellipsis = "…"

// Truncate shortens s to width, KEEPING THE TAIL.
//
// Measured, not chosen (ADR-0007). At a 24-wide column, 797 real component names
// need truncating, and the strategies cost:
//
//	head-truncate  (keep first 23)  395 collisions
//	middle-elide   (11 + 11)        100 collisions
//	tail-truncate  (keep last 23)    52 collisions
//
// Package names are hierarchical with the distinguishing part at the END, so
// "github.com/anch…" identifies nothing while "…anchore/clio" does. This reads
// wrong at first glance, which is why the ellipsis has to be unmistakable.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	if width == 1 {
		return Ellipsis
	}
	runes := []rune(s)
	keep := width - 1 // one cell for the ellipsis
	return Ellipsis + string(runes[len(runes)-keep:])
}

// Label renders a node for a column: name, plus @version only when it fits.
//
// purl and bom-ref never appear in a column. Measured: bom-ref does not fit at the
// MEDIAN (54 characters against a ~24-wide pane). They belong in the detail pane.
func Label(name, version string, width int) string {
	if version != "" {
		full := name + "@" + version
		if utf8.RuneCountInString(full) <= width {
			return full
		}
	}
	return Truncate(name, width)
}
