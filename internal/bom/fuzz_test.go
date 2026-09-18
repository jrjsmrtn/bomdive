// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package bom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fuzzing targets the one thing this tool does with input it did not write: parse a document
// somebody else produced. SECURITY.md names parser and traversal defects on hostile input as the
// most valuable class of report, so this is that claim under test rather than merely asserted.
//
// The bar is "never panics, and never lies": a malformed document may fail to load, but it may not
// crash, hang, or load into a graph whose own counts contradict each other.

func seedFromFixtures(f *testing.F) {
	f.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "*.cdx.*"))
	if err != nil || len(paths) == 0 {
		f.Fatalf("no fixtures to seed from: %v", err)
	}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw)
	}
	// Shapes a corpus of valid documents would never reach on its own.
	f.Add([]byte(""))
	f.Add([]byte("{"))
	f.Add([]byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","components":[{"bom-ref":"a"}],` +
		`"dependencies":[{"ref":"a","dependsOn":["a"]}]}`)) // a self-cycle
	f.Add([]byte(`<bom xmlns="http://cyclonedx.org/schema/bom/1.6"><components>`))
}

// FuzzLoad: arbitrary bytes through the real entry point, then the invariants the renderers rely
// on. A document that loads must not report more components in its graph than it contains.
func FuzzLoad(f *testing.F) {
	seedFromFixtures(f)
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		p := filepath.Join(dir, "fuzz.cdx.json")
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Skip()
		}
		g, err := Load(p)
		if err != nil {
			return // refusing is a correct outcome; crashing is not
		}
		c := g.Coverage()
		if c.InGraph > c.Components {
			t.Fatalf("coverage claims %d of %d components in the graph", c.InGraph, c.Components)
		}
		if got := len(g.Components()); got != c.Components {
			t.Fatalf("Components() lists %d, coverage counts %d", got, c.Components)
		}
		if c.Percent() > 100 {
			t.Fatalf("coverage of %d%%", c.Percent())
		}
		// Traversal must terminate on whatever shape arrived — cycles included (ADR-0004).
		roots, _ := g.Roots()
		for _, root := range roots {
			_ = g.Reachable(root.Ref)
			g.Children(root.Ref)
			g.Parents(root.Ref)
		}
	})
}

// FuzzResolve: the affects-reference resolver, which parses BOM-Link URNs
// (urn:cdx:<serial>/<version>#<bom-ref>) out of a document this tool did not write.
func FuzzResolve(f *testing.F) {
	for _, s := range []string{
		"a", "", "pkg:npm/left-pad@1.0.0",
		"urn:cdx:3e671687-395b-41f5-a30f-a58921a69b79/1#a",
		"urn:cdx:", "urn:cdx:///#", "urn:cdx:x/999999999999999999999#a",
		"urn:cdx:" + strings.Repeat("a", 4096) + "/1#b",
	} {
		f.Add(s)
	}
	g, err := Load(filepath.Join("..", "..", "testdata", "link-vex.cdx.json"))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, ref string) {
		_, state := g.Resolve(ref)
		if state.String() == "" {
			t.Fatalf("ref %q resolved to a state with no name", ref)
		}
		_ = g.LinkTargets(ref)
	})
}
