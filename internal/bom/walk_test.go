package bom

import (
	"testing"
	"time"
)

// The whole point of the corpus. A naive recursive walk does not terminate on any
// of these, and all four shapes occur in real BOMs.
func TestWalkTerminatesOnEveryCyclicFixture(t *testing.T) {
	for _, f := range []string{"cycle-direct", "cycle-self", "cycle-deep", "cycle-semantics"} {
		t.Run(f, func(t *testing.T) {
			g := fixture(t, f)
			roots, _ := g.Roots()
			if len(roots) == 0 {
				t.Fatal("no roots to walk from")
			}
			done := make(chan int, 1)
			go func() {
				n := 0
				g.Walk(roots[0].Ref, 0, func(Visit) bool { n++; return true })
				done <- n
			}()
			select {
			case n := <-done:
				if n == 0 {
					t.Error("walk emitted nothing")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("walk did not terminate — the failure this fixture exists to catch")
			}
		})
	}
}

// A cycle must be VISIBLE, not silently truncated: the reader has to be able to see
// why the branch stopped.
func TestCycleIsMarkedNotTruncated(t *testing.T) {
	g := fixture(t, "cycle-direct")
	roots, _ := g.Roots()
	var cycles, repeats int
	g.Walk(roots[0].Ref, 0, func(v Visit) bool {
		if v.Cycle {
			cycles++
		}
		if v.Repeat {
			repeats++
		}
		return true
	})
	if cycles == 0 {
		t.Error("no visit marked Cycle; the loop closed invisibly")
	}
	if repeats < cycles {
		t.Error("every Cycle must also be a Repeat")
	}
}

// The timeout test above catches a genuine hang, but an unguarded recursion dies of
// stack overflow instead, which kills the process and names no test. A depth-capped
// walk terminates either way, so the visit COUNT is what distinguishes guarded from
// unguarded — and it fails as an assertion rather than as a crash.
func TestCycleDoesNotRepeatWorkUnderADepthCap(t *testing.T) {
	g := fixture(t, "cycle-direct")
	roots, _ := g.Roots()
	n := 0
	g.Walk(roots[0].Ref, 10, func(Visit) bool { n++; return true })
	// Guarded: app, x, y, then x again as a back-reference. Unguarded with a cap of
	// 10 this climbs into double figures.
	if n > 6 {
		t.Errorf("%d visits over a 3-node cycle with depth cap 10; the guard is not holding", n)
	}
}

func TestSelfLoopDoesNotHang(t *testing.T) {
	g := fixture(t, "cycle-self")
	roots, _ := g.Roots()
	seen := map[string]int{}
	g.Walk(roots[0].Ref, 0, func(v Visit) bool { seen[v.Node.Ref]++; return true })
	for ref, n := range seen {
		if n > 2 {
			t.Errorf("%s emitted %d times; the visited-set marks after recursing", ref, n)
		}
	}
}

// A diamond must not duplicate its subtree: 288 shared nodes were measured in one
// real BOM, and repeating each subtree is how a renderer explodes.
func TestDiamondEmitsSharedNodeOnceWithSubtree(t *testing.T) {
	g := fixture(t, "diamond")
	roots, _ := g.Roots()
	var full, repeated int
	g.Walk(roots[0].Ref, 0, func(v Visit) bool {
		if v.Node.Name == "shared" {
			if v.Repeat {
				repeated++
			} else {
				full++
			}
		}
		return true
	})
	if full != 1 {
		t.Errorf("shared expanded %d times, want exactly 1", full)
	}
	if repeated != 1 {
		t.Errorf("shared back-referenced %d times, want 1 (via the second parent)", repeated)
	}
}

// A diamond is NOT a cycle, and the two are rendered differently on purpose.
//
// This exists because mutation testing showed the earlier tests could not tell
// them apart: leaving a node in the on-path set after unwinding marks every later
// re-encounter as a cycle, and every assertion still passed.
func TestSharedNodeIsARepeatNotACycle(t *testing.T) {
	g := fixture(t, "diamond")
	roots, _ := g.Roots()
	var sawRepeat bool
	g.Walk(roots[0].Ref, 0, func(v Visit) bool {
		if v.Node.Name == "shared" && v.Repeat {
			sawRepeat = true
			if v.Cycle {
				t.Error("a diamond's shared node was marked as a cycle; the on-path set is not being unwound")
			}
		}
		return true
	})
	if !sawRepeat {
		t.Fatal("shared was never re-encountered; the fixture or the walk changed")
	}
}

// Two sibling branches that touch the same subtree must not make each other look
// cyclic — the on-path set has to shrink as the walk unwinds.
func TestOnPathSetIsUnwound(t *testing.T) {
	g := fixture(t, "tree-simple")
	roots, _ := g.Roots()
	g.Walk(roots[0].Ref, 0, func(v Visit) bool {
		if v.Cycle {
			t.Errorf("%s marked as a cycle in an acyclic BOM", v.Node.Name)
		}
		return true
	})
}

// ADR-0004 picks node-uniqueness. Under relationship-uniqueness (Cypher) x IS
// reachable from itself via x->y->x; under node-uniqueness it is not. This pins
// the choice, and poc6-cycle-semantics.sh shows the two really differ.
func TestReachableUsesNodeUniqueness(t *testing.T) {
	g := fixture(t, "cycle-semantics")
	var x string
	for _, n := range g.Components() {
		if n.Name == "x" {
			x = n.Ref
		}
	}
	if x == "" {
		t.Fatal("fixture has no component named x")
	}
	for _, n := range g.Reachable(x) {
		if n.Name == "x" {
			t.Error("x reachable from itself: that is relationship-uniqueness, not the ADR-0004 choice")
		}
		if n.Name == "y" {
			return
		}
	}
	t.Error("y not reachable from x")
}

func TestMaxDepthIsHonoured(t *testing.T) {
	g := fixture(t, "cycle-deep")
	roots, _ := g.Roots()
	max := 0
	g.Walk(roots[0].Ref, 2, func(v Visit) bool {
		if v.Depth > max {
			max = v.Depth
		}
		return true
	})
	if max > 2 {
		t.Errorf("max depth reached %d with cap 2", max)
	}
}

func TestWalkOnGraphlessBOMEmitsOnlyTheStart(t *testing.T) {
	g := fixture(t, "no-dependencies-empty")
	if !g.DeclaresNoGraph() {
		t.Fatal("fixture should declare no graph")
	}
	n := 0
	for _, c := range g.Components() {
		g.Walk(c.Ref, 0, func(Visit) bool { n++; return true })
	}
	if n != len(g.Components()) {
		t.Errorf("emitted %d visits for %d components; there are no edges to follow", n, len(g.Components()))
	}
}
