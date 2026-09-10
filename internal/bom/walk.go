package bom

// Visit is one node encountered during a walk.
type Visit struct {
	Node   Node
	Depth  int
	Parent string

	// Repeat marks a node already emitted earlier in this walk. Its children are
	// NOT descended into again. This is what keeps a cyclic BOM terminating, and
	// what keeps a diamond from duplicating its whole subtree.
	Repeat bool

	// Cycle marks the stronger case: the node is on the current path, so this edge
	// closes a loop. Every Cycle is also a Repeat. Rendering them differently is
	// worth it — a diamond is normal, a cycle is worth seeing.
	Cycle bool
}

// Walk performs a depth-first traversal from ref, calling fn for each node reached.
//
// TRAVERSAL SEMANTICS ARE A DECISION, NOT A DEFAULT (ADR-0004). Two defensible
// readings of "reachable" disagree on real BOMs:
//
//   - relationship-uniqueness (Cypher's): an edge may not repeat, a node may. Over
//     x -> y -> x, x IS reachable from itself.
//   - node-uniqueness: a node may not repeat. x is NOT reachable from itself.
//
// lsxbom uses NODE-UNIQUENESS, because for "what does this pull in?" reporting a
// component as its own dependency is confusing rather than correct. Both were
// reproduced against real engines — see docs/inception/evidence/poc6-cycle-semantics.sh.
//
// maxDepth <= 0 means unlimited. Returning false from fn stops the walk.
func (g *Graph) Walk(ref string, maxDepth int, fn func(Visit) bool) {
	emitted := map[string]bool{}
	// onPath is a SET, not a slice scan. The obvious implementation walks the path
	// slice looking for cur, which is O(depth) per visit and therefore O(n^2) on a
	// deep chain — measured at 115ms for a 10k-deep spine. A first guess blamed the
	// sort in Children and memoising it made things slightly WORSE, which is how
	// the real cost was found.
	onPathSet := map[string]bool{}
	var path []string

	var rec func(cur, parent string, depth int) bool
	rec = func(cur, parent string, depth int) bool {
		n, known := g.nodes[cur]
		if !known {
			return true // dangling: reported via Coverage, never invented here
		}

		onPath := onPathSet[cur]
		repeat := onPath || emitted[cur]

		if !fn(Visit{Node: n, Depth: depth, Parent: parent, Repeat: repeat, Cycle: onPath}) {
			return false
		}
		if repeat {
			return true // the guard: never descend twice, so cycles terminate
		}
		emitted[cur] = true
		if maxDepth > 0 && depth+1 > maxDepth {
			return true
		}

		path = append(path, cur)
		onPathSet[cur] = true
		defer func() {
			path = path[:len(path)-1]
			delete(onPathSet, cur)
		}()
		for _, child := range g.Children(cur) {
			if !rec(child.Ref, cur, depth+1) {
				return false
			}
		}
		return true
	}
	rec(ref, "", 0)
}

// Reachable returns every node reachable from ref under node-uniqueness,
// excluding ref itself. Order is deterministic (by name, then ref).
func (g *Graph) Reachable(ref string) []Node {
	seen := map[string]bool{}
	g.Walk(ref, 0, func(v Visit) bool {
		if v.Node.Ref != ref {
			seen[v.Node.Ref] = true
		}
		return true
	})
	refs := make([]string, 0, len(seen))
	for r := range seen {
		refs = append(refs, r)
	}
	return g.resolve(refs)
}
