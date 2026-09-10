package bom

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// synthBOM writes a BOM of n components with a realistic shape: a wide root, a
// long spine, shared nodes and a cycle.
//
// Generated rather than committed, and generated rather than borrowed from a real
// corpus: the 1.0 budget is stated against ~10^4 components, and a benchmark that
// depends on a file only one machine has is not a benchmark anyone else can run.
func synthBOM(t testing.TB, n int) string {
	t.Helper()
	type comp struct {
		Ref     string `json:"bom-ref"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Version string `json:"version"`
		PURL    string `json:"purl"`
	}
	type dep struct {
		Ref       string   `json:"ref"`
		DependsOn []string `json:"dependsOn"`
	}
	ref := func(i int) string { return fmt.Sprintf("pkg:generic/github.com/example/mod%06d@1.0.0", i) }

	comps := make([]comp, 0, n)
	for i := 0; i < n; i++ {
		comps = append(comps, comp{
			Ref: ref(i), Type: "library",
			Name:    fmt.Sprintf("github.com/example/group%03d/mod%06d", i%200, i),
			Version: "1.0.0", PURL: ref(i),
		})
	}

	var deps []dep
	// a wide root
	var wide []string
	for i := 1; i < 200 && i < n; i++ {
		wide = append(wide, ref(i))
	}
	deps = append(deps, dep{Ref: ref(0), DependsOn: wide})
	// a long spine with shared tails, plus one cycle back to the root region
	for i := 1; i < n-2; i++ {
		on := []string{ref(i + 1)}
		if i%7 == 0 {
			on = append(on, ref(n-1)) // a shared node many parents point at
		}
		if i%1000 == 0 {
			on = append(on, ref(1)) // a cycle
		}
		deps = append(deps, dep{Ref: ref(i), DependsOn: on})
	}

	doc := map[string]any{
		"bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1,
		"metadata":   map[string]any{"component": map[string]any{"bom-ref": ref(0), "type": "application", "name": "root"}},
		"components": comps, "dependencies": deps,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "synth.cdx.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func BenchmarkLoad10k(b *testing.B) {
	path := synthBOM(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Load(path); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWalk10k(b *testing.B) {
	g, err := Load(synthBOM(b, 10000))
	if err != nil {
		b.Fatal(err)
	}
	roots, _ := g.Roots()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := 0
		g.Walk(roots[0].Ref, 0, func(Visit) bool { n++; return true })
		if n == 0 {
			b.Fatal("walk emitted nothing; the benchmark is measuring nothing")
		}
	}
}

// The 1.0 acceptance criterion, as a test rather than a note in a roadmap.
//
// Deliberately generous — 2s is the budget and the measured figure is far under —
// because a tight timing assertion on shared CI is flaky, and a flaky gate gets
// disabled. It catches a catastrophic regression, which is what it is for.
func TestLoadAndWalkWithinTheBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	path := synthBOM(t, 10000)
	start := time.Now()
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	roots, _ := g.Roots()
	if len(roots) == 0 {
		t.Fatal("no roots; the walk below would measure nothing")
	}
	visits := 0
	g.Walk(roots[0].Ref, 0, func(Visit) bool { visits++; return true })
	elapsed := time.Since(start)

	// Gate on BOTH: fast enough, and the work actually happened.
	if visits < 1000 {
		t.Fatalf("only %d visits over a 10k-component BOM; timing is meaningless", visits)
	}
	if elapsed.Seconds() > 2.0 {
		t.Errorf("load+walk took %v over the 2s budget (%d visits)", elapsed, visits)
	}
	t.Logf("load+walk of 10k components: %v, %d visits", elapsed, visits)
}
