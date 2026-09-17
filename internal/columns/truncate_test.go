// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package columns

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateKeepsTheTail(t *testing.T) {
	const in = "github.com/anchore/clio"
	for _, w := range []int{8, 10, 12, 20} {
		got := Truncate(in, w)
		if n := utf8.RuneCountInString(got); n != w {
			t.Errorf("Truncate(_, %d) = %q (%d runes), want exactly %d", w, got, n, w)
		}
		if !strings.HasPrefix(got, Ellipsis) {
			t.Errorf("Truncate(_, %d) = %q, want a leading ellipsis", w, got)
		}
		// The whole point: what survives is the END of the input.
		tail := strings.TrimPrefix(got, Ellipsis)
		if !strings.HasSuffix(in, tail) {
			t.Errorf("Truncate(_, %d) = %q, whose kept part is not a suffix of the input", w, got)
		}
	}
}

// The property the measurement actually justified: tail-truncation disambiguates a
// shared prefix, head-truncation does not.
func TestTailTruncationDisambiguatesSharedPrefixes(t *testing.T) {
	names := []string{
		"github.com/anchore/clio",
		"github.com/anchore/fangs",
		"github.com/anchore/stereoscope",
	}
	seen := map[string]bool{}
	for _, n := range names {
		got := Truncate(n, 14)
		if seen[got] {
			t.Errorf("collision at width 14: %q", got)
		}
		seen[got] = true
	}
	if len(seen) != len(names) {
		t.Errorf("%d distinct labels for %d names", len(seen), len(names))
	}
}

func TestTruncateEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		in   string
		w    int
		want string
	}{
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"toolong", 0, ""},
		{"toolong", 1, Ellipsis},
		{"", 5, ""},
	} {
		if got := Truncate(tc.in, tc.w); got != tc.want {
			t.Errorf("Truncate(%q,%d) = %q, want %q", tc.in, tc.w, got, tc.want)
		}
	}
}

// Multi-byte input must be cut on rune boundaries, not bytes.
func TestTruncateIsRuneSafe(t *testing.T) {
	got := Truncate("ααααββββγγγγ", 5)
	if r := []rune(got); len(r) != 5 {
		t.Errorf("Truncate returned %d runes, want 5: %q", len(r), got)
	}
}

func TestLabelDropsVersionWhenItDoesNotFit(t *testing.T) {
	if got := Label("pkg", "1.0.0", 20); got != "pkg@1.0.0" {
		t.Errorf("Label = %q, want pkg@1.0.0", got)
	}
	got := Label("github.com/anchore/clio", "v0.1.1", 14)
	if len([]rune(got)) > 14 {
		t.Errorf("Label = %q, longer than the column", got)
	}
	if got == "github.com/anchore/clio@v0.1.1" {
		t.Error("version kept despite not fitting")
	}
}
