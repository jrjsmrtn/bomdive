#!/usr/bin/env python3
"""Measure the graph shape of a CycloneDX BOM: is it a tree, a DAG, or flat?"""
import json, sys
from collections import defaultdict, deque

def shape(path):
    d = json.load(open(path))
    comps = d.get("components", []) or []
    deps  = d.get("dependencies", []) or []
    root  = (d.get("metadata", {}).get("component", {}) or {}).get("bom-ref")

    edges = {e.get("ref"): (e.get("dependsOn") or []) for e in deps}
    nonempty = {r: t for r, t in edges.items() if t}
    indeg = defaultdict(int)
    for r, targets in edges.items():
        for t in targets:
            indeg[t] += 1
    shared = [t for t, n in indeg.items() if n > 1]

    # cycle detection + depth over the dependency graph
    WHITE, GREY, BLACK = 0, 1, 2
    color = defaultdict(int)
    cycles = []
    depth = {}
    def dfs(n, stack):
        color[n] = GREY
        best = 0
        for m in edges.get(n, []):
            if color[m] == GREY:
                cycles.append(stack[stack.index(m):] + [m] if m in stack else [n, m])
                continue
            if color[m] == WHITE:
                dfs(m, stack + [m])
            best = max(best, depth.get(m, 0) + 1)
        color[n] = BLACK
        depth[n] = best
    sys.setrecursionlimit(100000)
    starts = [root] if root in edges else list(edges)
    for s in starts:
        if color[s] == WHITE:
            dfs(s, [s])
    for n in edges:
        if color[n] == WHITE:
            dfs(n, [n])

    reachable = set()
    if root in edges:
        q = deque([root]); reachable.add(root)
        while q:
            n = q.popleft()
            for m in edges.get(n, []):
                if m not in reachable:
                    reachable.add(m); q.append(m)

    comp_refs = {c.get("bom-ref") for c in comps}
    ctypes = defaultdict(int)
    for c in comps:
        ctypes[c.get("type", "?")] += 1

    return {
        "file": path.split("/")[-1],
        "specVersion": d.get("specVersion"),
        "components": len(comps),
        "component_types": dict(ctypes),
        "dependency_entries": len(deps),
        "entries_with_dependsOn": len(nonempty),
        "root_declared": bool(root),
        "root_in_graph": root in edges,
        "edges_total": sum(len(v) for v in edges.values()),
        "shared_nodes_(>1 parent)": len(shared),
        "is_tree": len(shared) == 0,
        "cycles_found": len(cycles),
        "max_depth_from_root": depth.get(root, 0) if root in edges else max(depth.values(), default=0),
        "components_reachable_from_root": len(reachable & comp_refs) if reachable else 0,
        "components_orphaned": len(comp_refs - reachable) if reachable else len(comp_refs),
    }

for p in sys.argv[1:]:
    try:
        print(json.dumps(shape(p), indent=2))
    except Exception as e:
        print(json.dumps({"file": p.split("/")[-1], "ERROR": str(e)}))
