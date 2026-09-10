#!/usr/bin/env python3
"""Assert every fixture actually contains the shape its manifest claims.

WHY THIS EXISTS. A fixture named `cycle-direct` that contains no cycle tests nothing,
and nothing would say so — the test suite would pass, the traversal core would be
unproven, and the failure would surface against a real BOM instead. The manifest is a
claim; this file is the check on it.

Exit 0 all fixtures match their claims, 1 on any mismatch.
Usage: verify.py [dir]        --self-test proves it catches a planted mismatch
"""
import json, pathlib, sys
from collections import defaultdict


def properties(doc: dict) -> dict:
    comps = doc.get("components") or []
    has_deps_key = "dependencies" in doc
    deps = doc.get("dependencies") or []
    root = (doc.get("metadata", {}).get("component") or {}).get("bom-ref")

    edges = {e.get("ref"): list(e.get("dependsOn") or []) for e in deps}
    comp_refs = {c.get("bom-ref") for c in comps}

    indeg = defaultdict(int)
    for src, targets in edges.items():
        for t in targets:
            indeg[t] += 1
    shared = sum(1 for n, k in indeg.items() if k > 1)

    # cycle count: number of distinct back-edges found by DFS over the edge map
    WHITE, GREY, BLACK = 0, 1, 2
    color = defaultdict(int)
    back_edges = set()
    sys.setrecursionlimit(10000)

    def dfs(n):
        color[n] = GREY
        for m in edges.get(n, []):
            if color[m] == GREY:
                back_edges.add((n, m))
            elif color[m] == WHITE:
                dfs(m)
        color[n] = BLACK

    for n in list(edges):
        if color[n] == WHITE:
            dfs(n)

    # depth / reachability from the DECLARED root only
    depth_from_root, reachable = 0, set()
    if root in edges:
        stack = [(root, 0)]
        seen = {root}
        while stack:
            n, d = stack.pop()
            depth_from_root = max(depth_from_root, d)
            for m in edges.get(n, []):
                if m not in seen:
                    seen.add(m)
                    stack.append((m, d + 1))
        reachable = seen - {root}

    all_targets = {t for ts in edges.values() for t in ts}
    in_graph = (set(edges) | all_targets) & comp_refs
    cats = {p.get("value") for c in comps for p in (c.get("properties") or [])
            if p.get("name") == "cdx:osquery:category"}

    return {
        "specVersion": doc.get("specVersion"),
        "components": len(comps),
        # A document can carry these INSTEAD of components — a standalone VEX, a
        # services-only document. All of them list zero components, and telling them
        # apart is what stops the tool drawing an unexplained blank.
        "vulnerabilities": len(doc.get("vulnerabilities") or []),
        "services": len(doc.get("services") or []),
        "dependency_entries": len(deps),
        "has_dependencies_key": has_deps_key,
        "root_declared": root is not None,
        "root_in_graph": root in edges if root else False,
        "components_reachable_from_root": len(reachable & comp_refs),
        "max_depth_from_root": depth_from_root,
        "shared_nodes": shared,
        "cycles_found": len(back_edges),
        "is_tree": shared == 0 and not back_edges,
        "indegree_zero": len([n for n in edges if indeg[n] == 0]),
        "in_graph_components": len(in_graph),
        "osquery_categories": len(cats),
        "purl_less_components": sum(1 for c in comps if not c.get("purl")),
        "dangling_targets": len(all_targets - comp_refs),
        "unique_names_lt_components": len({c.get("name") for c in comps}) < len(comps),
        "max_properties": max((len(c.get("properties") or []) for c in comps), default=0),
        "unnamed_components": sum(1 for c in comps if not c.get("name")),
        "max_value_length": max((len(p.get("value") or "")
                                 for c in comps for p in (c.get("properties") or [])), default=0),
        # True when some component's properties are NOT in alphabetical order, so a
        # test can tell "document order preserved" from "sorted" at all.
        "has_unsorted_properties": any(
            (names := [p.get("name") for p in (c.get("properties") or [])]) != sorted(names)
            for c in comps
        ),
    }


def check(dirpath: pathlib.Path):
    manifest = json.loads((dirpath / "manifest.json").read_text())
    failures, checked = [], 0
    for name, entry in sorted(manifest.items()):
        path = dirpath / entry["file"]
        # XML fixtures are checked for ENCODING and version only. Re-implementing
        # the graph measurement against a second syntax would be a second thing to
        # keep correct, and what these fixtures exist to prove is that the encoding
        # is handled at all.
        if path.suffix == ".xml":
            text = path.read_text()
            actual = {
                "encoding": "xml",
                "specVersion": next((v for v in ("1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7")
                                     if f"schema/bom/{v}" in text), None),
            }
        else:
            actual = properties(json.loads(path.read_text()))
        for key, want in entry["asserts"].items():
            checked += 1
            got = actual.get(key)
            if got != want:
                failures.append(f"{name}: {key} claims {want!r}, file has {got!r}")
    return manifest, checked, failures


def main():
    args = [a for a in sys.argv[1:] if a != "--self-test"]
    here = pathlib.Path(args[0]) if args else pathlib.Path(__file__).parent

    if "--self-test" in sys.argv:
        ok = True
        # planted good: a real cycle must be counted
        good = {"bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1,
                "components": [{"bom-ref": "a", "type": "library", "name": "a"},
                               {"bom-ref": "b", "type": "library", "name": "b"}],
                "dependencies": [{"ref": "a", "dependsOn": ["b"]},
                                 {"ref": "b", "dependsOn": ["a"]}]}
        p = properties(good)
        print(f"  planted CYCLE      -> cycles_found={p['cycles_found']} "
              f"{'PASS' if p['cycles_found'] == 1 else 'FAIL'}")
        ok &= p["cycles_found"] == 1
        # planted bad: same doc but acyclic — the checker must NOT report a cycle
        acyclic = json.loads(json.dumps(good))
        acyclic["dependencies"][1]["dependsOn"] = []
        p2 = properties(acyclic)
        print(f"  planted ACYCLIC    -> cycles_found={p2['cycles_found']} "
              f"{'PASS' if p2['cycles_found'] == 0 else 'FAIL'}")
        ok &= p2["cycles_found"] == 0
        # planted mismatch: a manifest claim that contradicts the file must FAIL
        import tempfile, os
        with tempfile.TemporaryDirectory() as td:
            t = pathlib.Path(td)
            (t / "x.cdx.json").write_text(json.dumps(acyclic))
            (t / "manifest.json").write_text(json.dumps(
                {"x": {"file": "x.cdx.json", "why": "planted", "asserts": {"cycles_found": 1}}}))
            _, _, fails = check(t)
        print(f"  planted MISMATCH   -> {len(fails)} failure(s) "
              f"{'PASS' if len(fails) == 1 else 'FAIL'}")
        ok &= len(fails) == 1
        print("SELF-TEST", "PASS" if ok else "FAIL")
        return 0 if ok else 1

    manifest, checked, failures = check(here)
    if failures:
        print(f"FIXTURE CLAIMS UNMET ({len(failures)}):")
        for f in failures:
            print(f"  - {f}")
        return 1
    print(f"fixtures OK ({len(manifest)} fixtures, {checked} claims verified)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
