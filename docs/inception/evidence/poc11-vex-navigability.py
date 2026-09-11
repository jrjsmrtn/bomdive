#!/usr/bin/env python3
"""Could a VEX be navigated in the column view, and would its links lead anywhere?

WHY. lsxbom navigates components. Before deciding to browse vulnerability records
(ADR-0009), this measures whether VEX documents have the shape the column view needs —
something to group by, something to descend into — and whether their `affects[].ref`
references resolve.

Two shapes are measured separately, because they turned out to be different worlds:

  STANDALONE VEX  vulnerabilities and NO components. Every sample in the public corpora is
                  this shape, and every one is an illustrative use case.
  EMBEDDED VEX    vulnerabilities AND components in one document. The public corpora hold
                  none; a Dependency-Track 5.1.0 VEX export is this shape.

For each: vulnerabilities per document, analysis states, severities, how references
resolve (a LOCAL ref must name a bom-ref in the same document; a BOM-LINK
`urn:cdx:<serial>/<version>#<bom-ref>` must name a corpus document at that version holding
that bom-ref), and, for embedded VEX, whether the components are an inventory or only the
components some vulnerability affects.

Counts only, never identifiers, so it is safe over a private corpus.

    python3 docs/inception/evidence/poc11-vex-navigability.py <corpus-dir>

<corpus-dir> holds one sub-directory per source, each holding *.json documents.
"""
import collections
import glob
import json
import os
import sys

SEVERITY_ORDER = ["critical", "high", "medium", "low", "info", "none", "unknown"]


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


def top_severity(vuln):
    """The most severe rating a vulnerability carries, or "(no rating)"."""
    sev = [r.get("severity") for r in vuln.get("ratings") or [] if r.get("severity")]
    if not sev:
        return "(no rating)"
    return min(sev, key=lambda s: SEVERITY_ORDER.index(s) if s in SEVERITY_ORDER else 99)


def measure(docs, by_serial):
    """Shape of a set of documents that all carry vulnerabilities."""
    per_doc, per_vuln = [], []
    states, severities, roots = collections.Counter(), collections.Counter(), collections.Counter()
    local, link = collections.Counter(), collections.Counter()
    comps_total = comps_affected = docs_only_affected = with_deps = 0
    max_vulns_per_component = 0
    for d in docs:
        own = refs_in(d)
        vulns = d["vulnerabilities"]
        per_doc.append(len(vulns))
        roots[((d.get("metadata") or {}).get("component") or {}).get("type", "(no root)")] += 1
        with_deps += bool(d.get("dependencies"))
        targets = collections.Counter()
        for v in vulns:
            states[(v.get("analysis") or {}).get("state", "(no analysis)")] += 1
            severities[top_severity(v)] += 1
            affects = v.get("affects") or []
            per_vuln.append(len(affects))
            for a in affects:
                ref = a.get("ref", "")
                targets[ref] += 1
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
                elif fragment in refs_in(exact[0]):
                    link["resolves"] += 1
                else:
                    link["target found, bom-ref missing"] += 1
        comps = {c.get("bom-ref") for c in d.get("components") or []}
        comps_total += len(comps)
        comps_affected += len(comps & set(targets))
        docs_only_affected += bool(comps) and comps <= set(targets)
        if targets:
            max_vulns_per_component = max(max_vulns_per_component, max(targets.values()))
    return {
        "documents": len(docs),
        "vulnerabilities per document": f"{min(per_doc)}–{max(per_doc)}" if per_doc else "-",
        "vulnerabilities, total": sum(per_doc),
        "affects[] per vulnerability": f"{min(per_vuln)}–{max(per_vuln)}" if per_vuln else "-",
        "most vulnerabilities on one target": max_vulns_per_component,
        "analysis.state": dict(states.most_common()),
        "most severe rating": dict(severities.most_common()),
        "root component type": dict(roots.most_common()),
        "documents with a `dependencies` graph": with_deps,
        "components, total": comps_total,
        "components some vulnerability affects": comps_affected,
        "documents whose components are ONLY the affected ones": docs_only_affected,
        "local refs": dict(local),
        "BOM-Link refs": dict(link),
    }


def main(root: str) -> int:
    docs, unreadable = [], 0
    for path in sorted(glob.glob(os.path.join(root, "*", "*.json"))):
        try:
            d = json.load(open(path))
        except Exception:
            unreadable += 1
            continue
        if isinstance(d, dict) and d.get("bomFormat") == "CycloneDX":
            docs.append(d)

    by_serial = collections.defaultdict(list)  # uuid -> [(version, doc)]
    for d in docs:
        s = d.get("serialNumber") or ""
        if s.startswith("urn:uuid:"):
            by_serial[s[len("urn:uuid:"):].lower()].append((str(d.get("version")), d))

    standalone = [d for d in docs if not d.get("components") and d.get("vulnerabilities")]
    embedded = [d for d in docs if d.get("components") and d.get("vulnerabilities")]

    print(f"CycloneDX JSON documents : {len(docs)}   (unreadable files skipped: {unreadable})")
    for title, group in (("STANDALONE VEX", standalone), ("EMBEDDED VEX", embedded)):
        print(f"\n== {title}: {len(group)} document(s)")
        if not group:
            continue
        for key, value in measure(group, by_serial).items():
            if key == "documents":
                continue
            if title == "STANDALONE VEX" and key.startswith(("components", "documents whose")):
                continue
            print(f"  {key:<55s}: {value}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else ".corpora-cache"))
