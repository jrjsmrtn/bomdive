#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
#
# SPDX-License-Identifier: Apache-2.0

"""POC-2: measure OBOM/HBOM/image-SBOM graph shape.

Usage: poc2-corpus-shape.py <out.json> <corpus-dir>
The corpus dir is passed in, never hardcoded: BOMs of a private estate carry host
identifiers, and only the anonymised shape is written out.
"""
import json, sys, glob, os, importlib.util, collections
spec = importlib.util.spec_from_file_location("shape", os.path.join(os.path.dirname(__file__), "bom-graph-shape.py"))
m = importlib.util.module_from_spec(spec)
src = open(os.path.join(os.path.dirname(__file__), "bom-graph-shape.py")).read().split('for p in sys.argv')[0]
exec(compile(src, "shape", "exec"), m.__dict__)

files = sorted(glob.glob(os.path.join(sys.argv[2], "*", "*.json")))
counters = collections.Counter()
rows = []
for p in files:
    base = os.path.basename(p)
    kind = "OBOM" if base.startswith("obom") else "HBOM" if base.startswith("hbom") else "IMG"
    counters[kind] += 1
    label = f"{kind}-{counters[kind]:02d}"          # host name deliberately discarded
    try:
        r = m.shape(p)
        r["file"] = label
        r["kind"] = kind
        rows.append(r)
    except Exception as e:
        rows.append({"file": label, "kind": kind, "ERROR": str(e)})
json.dump(rows, open(sys.argv[1], "w"), indent=2)

# terminal summary
hdr = f"{'label':8} {'spec':5} {'comps':>7} {'depEnt':>7} {'rootIn':>6} {'shared':>7} {'cycles':>7} {'depth':>6} {'reach%':>7}"
print(hdr); print("-"*len(hdr))
for r in rows:
    if "ERROR" in r:
        print(f"{r['file']:8} ERROR {r['ERROR'][:60]}"); continue
    reach = (100*r["components_reachable_from_root"]/r["components"]) if r["components"] else 0
    print(f"{r['file']:8} {str(r['specVersion']):5} {r['components']:7} {r['dependency_entries']:7} "
          f"{str(r['root_in_graph']):>6} {r['shared_nodes_(>1 parent)']:7} {r['cycles_found']:7} "
          f"{r['max_depth_from_root']:6} {reach:6.0f}%")
print()
for kind in ("OBOM","HBOM","IMG"):
    sub=[r for r in rows if r.get("kind")==kind and "ERROR" not in r]
    if not sub: continue
    withgraph=[r for r in sub if r["dependency_entries"]>0]
    print(f"{kind}: n={len(sub)}  with a dependency graph: {len(withgraph)}  "
          f"any cycles: {sum(1 for r in sub if r['cycles_found']>0)}  "
          f"root in graph: {sum(1 for r in sub if r['root_in_graph'])}")
    types=collections.Counter()
    for r in sub: types.update(r.get("component_types",{}))
    print(f"      component types seen: {dict(types)}")
