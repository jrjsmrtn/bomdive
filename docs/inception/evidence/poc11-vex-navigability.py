#!/usr/bin/env python3
"""Could a standalone VEX be navigated in the column view, and would its links lead anywhere?

WHY. lsxbom navigates components. A standalone VEX has none, so it shows nothing. Before
deciding to browse VEX documents (ADR-0009), this measures whether they have the shape the
column view needs — something to group by, something to descend into — and whether their
`affects[].ref` references resolve to anything:

  - a LOCAL ref must name a bom-ref in the same document;
  - a BOM-LINK ref (`urn:cdx:<serial>/<version>#<bom-ref>`) must name a document present in
    the corpus, at that version, containing that bom-ref.

Counts only, never identifiers, so it is safe over a private corpus.

    python3 docs/inception/evidence/poc11-vex-navigability.py .corpora-cache
"""
import collections
import glob
import json
import os
import sys


def refs_in(doc):
    """Every bom-ref a document defines: its root, components and services, nested."""
    out = set()
    root = (doc.get("metadata") or {}).get("component") or {}
    if root.get("bom-ref"):
        out.add(root["bom-ref"])
    stack = list(doc.get("components") or []) + list(doc.get("services") or [])
    while stack:
        x = stack.pop()
        if x.get("bom-ref"):
            out.add(x["bom-ref"])
        stack += (x.get("components") or []) + (x.get("services") or [])
    return out


def main(root: str) -> int:
    docs = {}
    for path in sorted(glob.glob(os.path.join(root, "*", "*.json"))):
        try:
            d = json.load(open(path))
        except Exception:
            continue
        if isinstance(d, dict) and d.get("bomFormat") == "CycloneDX":
            docs[path] = d

    by_serial = collections.defaultdict(list)  # uuid -> [(version, path)]
    for path, d in docs.items():
        s = d.get("serialNumber") or ""
        if s.startswith("urn:uuid:"):
            by_serial[s[len("urn:uuid:"):].lower()].append((str(d.get("version")), path))

    vex = {p: d for p, d in docs.items() if not d.get("components") and d.get("vulnerabilities")}
    embedded = sum(1 for d in docs.values() if d.get("components") and d.get("vulnerabilities"))

    per_doc, per_vuln = [], []
    states = collections.Counter()
    local, link = collections.Counter(), collections.Counter()
    for d in vex.values():
        own = refs_in(d)
        per_doc.append(len(d["vulnerabilities"]))
        for v in d["vulnerabilities"]:
            states[(v.get("analysis") or {}).get("state", "(no analysis)")] += 1
            affects = v.get("affects") or []
            per_vuln.append(len(affects))
            for a in affects:
                ref = a.get("ref", "")
                if not ref.startswith("urn:cdx:"):
                    local["resolves" if ref in own else "names nothing"] += 1
                    continue
                body, _, fragment = ref[len("urn:cdx:"):].partition("#")
                serial, _, version = body.partition("/")
                candidates = by_serial.get(serial.lower(), [])
                if not candidates:
                    link["target document not in corpus"] += 1
                    continue
                exact = [p for v_, p in candidates if v_ == version]
                if not exact:
                    link["serial found, version differs"] += 1
                elif fragment in refs_in(docs[exact[0]]):
                    link["resolves"] += 1
                else:
                    link["target found, bom-ref missing"] += 1

    print(f"CycloneDX JSON documents          : {len(docs)}")
    print(f"standalone VEX documents          : {len(vex)}")
    print(f"SBOMs with embedded VEX           : {embedded}")
    if not vex:
        return 0
    print(f"vulnerabilities per VEX document  : min {min(per_doc)}, max {max(per_doc)}")
    print(f"affects[] per vulnerability       : min {min(per_vuln)}, max {max(per_vuln)}")
    print(f"analysis.state                    : {dict(states.most_common())}")
    print(f"local refs    ({sum(local.values()):3d})               : {dict(local)}")
    print(f"BOM-Link refs ({sum(link.values()):3d})               : {dict(link)}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else ".corpora-cache"))
