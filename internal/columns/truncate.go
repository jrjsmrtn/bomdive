// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package columns

import (
	"strings"
	"unicode/utf8"
)

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

// Descend marks an entry you can descend into — the column view's equivalent of
// the chevron macOS Finder puts on a folder. Without it, a leaf and a component
// with fifty dependencies look identical until you press → and nothing happens.
const Descend = "→"

// Row lays out one column row: the label, and Descend in the LAST cell when the
// entry can be descended into.
//
// width is the row's available cells. The label is truncated to leave room rather
// than allowed to push the arrow off the edge, because an indicator that silently
// disappears at narrow widths is worse than one that was never there.
func Row(label string, descendable bool, width int) string {
	if !descendable || width < 2 {
		return Truncate(label, width)
	}
	label = Truncate(label, width-2)
	pad := width - 1 - utf8.RuneCountInString(label)
	if pad < 1 {
		pad = 1
	}
	return label + strings.Repeat(" ", pad) + Descend
}
