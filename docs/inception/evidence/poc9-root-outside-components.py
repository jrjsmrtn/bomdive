#!/usr/bin/env python3
"""How many BOMs declare a root that drives the graph but is absent from `components`?

WHY. metadata.component is the SUBJECT of a document, not a member of its inventory, so it
is routinely absent from `components` while still driving `dependencies`. lsxbom's Node()
synthesised such a root (POC-8) but resolve() read the component map directly, so every
direct dependency of that root reported NO dependents. This measures the reach.

Counts, per document:
  - a declared root NOT in `components`, that DOES drive the graph;
  - how many of its dependsOn targets ARE components — each one a component whose sole
    declared dependent was invisible.

Run against the corpora cache populated by scripts/check-corpora.sh:
    python3 docs/inception/evidence/poc9-root-outside-components.py .corpora-cache
"""
import collections
import glob
import json
import os
import sys


def main(root: str) -> int:
    docs = sorted(glob.glob(os.path.join(root, "*", "*.json")))
    scanned = skipped = affected_docs = affected_comps = 0
    by_corpus = collections.Counter()

    for path in docs:
        try:
            doc = json.load(open(path))
        except Exception:
            skipped += 1
            continue
        # Only CycloneDX JSON: the corpora carry schemas, fixtures and tool inputs too.
        if not isinstance(doc, dict) or doc.get("bomFormat") != "CycloneDX":
            skipped += 1
            continue
        scanned += 1

        meta = doc.get("metadata")
        meta = meta if isinstance(meta, dict) else {}
        comp = meta.get("component")
        declared = comp.get("bom-ref") if isinstance(comp, dict) else None
        if not declared:
            continue

        inventory = {
            c.get("bom-ref") for c in (doc.get("components") or []) if isinstance(c, dict)
        }
        if declared in inventory:
            continue  # the root IS inventory: resolve found it either way

        deps = doc.get("dependencies") or []
        drives = next(
            (d for d in deps if isinstance(d, dict) and d.get("ref") == declared), None
        )
        if not drives or not drives.get("dependsOn"):
            continue  # declared but drives nothing: correctly unresolvable

        # Only targets that are real components: a dangling target was invisible anyway.
        hidden = sum(1 for t in drives["dependsOn"] if t in inventory)
        if hidden:
            affected_docs += 1
            affected_comps += hidden
            by_corpus[os.path.basename(os.path.dirname(path))] += 1

    print(f"CycloneDX JSON documents scanned      : {scanned}")
    print(f"non-BOM files skipped                 : {skipped}")
    print(f"documents with the affected root shape: {affected_docs}"
          f" ({affected_docs * 100 // max(scanned, 1)}%)")
    print(f"components whose only declared dependent was invisible: {affected_comps}")
    for corpus, n in sorted(by_corpus.items()):
        print(f"  {corpus}: {n} document(s)")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else ".corpora-cache"))
