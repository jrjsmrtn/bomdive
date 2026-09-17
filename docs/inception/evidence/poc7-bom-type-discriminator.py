#!/usr/bin/env python3
"""Which kind of xBOM is this, and can the tool tell without guessing?

bomdive must decide whether `tree` is even meaningful before it renders anything, and
guessing from content is fragile. CycloneDX offers two spec-defined signals that,
together, discriminated every real BOM measured:

    metadata.lifecycles[]  -- .phase (enum) OR .name (custom form)
    metadata.component.type

⚠ THE CUSTOM LIFECYCLE FORM IS REAL AND EASY TO MISS. A lifecycle entry is either
{"phase": "operations"} or {"name": ..., "description": ...}. Reading only `.phase`
returns null for the custom form and silently misclassifies the BOM. Found by
re-measuring: one SBOM in the corpus uses {"name": "Deployed", ...} citing the CISA
SBOM type, and the first version of this check reported it as having no lifecycle.

Emits SHAPE ONLY — no filename, host or path — so it is safe to run over a private
corpus and paste the output.

Usage:
  poc7-bom-type-discriminator.py <bom.json> ...     classify specific files
  poc7-bom-type-discriminator.py --corpus <dir>     summarise a tree, anonymised
"""
import json, pathlib, sys
from collections import Counter


def signals(doc: dict) -> dict:
    md = doc.get("metadata") or {}
    lifecycles = md.get("lifecycles") or []
    phases, names = [], []
    for entry in lifecycles:
        if not isinstance(entry, dict):
            continue
        if entry.get("phase"):
            phases.append(entry["phase"])
        elif entry.get("name"):          # the custom form
            names.append(entry["name"])
    comp = md.get("component") or {}
    deps = doc.get("dependencies")
    return {
        "phases": phases,
        "custom_lifecycles": names,
        "root_type": comp.get("type"),
        "has_root": bool(comp),
        "dependencies": None if deps is None else len(deps),
    }


def classify(s: dict) -> str:
    """The rule, stated so it can be argued with rather than inferred from code."""
    t, ph = s["root_type"], set(s["phases"])
    if t == "operating-system" and "operations" in ph:
        return "OBOM"
    if t == "device" and "operations" in ph:
        return "HBOM"
    if t == "container":
        return "image SBOM"
    if s["custom_lifecycles"]:
        return f"SBOM (custom lifecycle: {', '.join(s['custom_lifecycles'])})"
    if t:
        return f"SBOM (root type: {t})"
    return "SBOM (no lifecycle phase, no root type — undeclared)"


def main():
    args = sys.argv[1:]
    if not args:
        print(__doc__)
        return 2

    if args[0] == "--corpus":
        root = pathlib.Path(args[1])
        rows = Counter()
        for p in sorted(root.rglob("*.json")):
            try:
                s = signals(json.loads(p.read_text()))
            except Exception:
                continue
            rows[(classify(s), tuple(s["phases"]), s["root_type"],
                  "deps" if s["dependencies"] else "no deps")] += 1
        print(f"{'verdict':52} {'phases':30} {'root type':18} {'graph':8} n")
        print("-" * 118)
        for (verdict, phases, rt, graph), n in rows.most_common():
            print(f"{verdict:52} {str(list(phases)):30} {str(rt):18} {graph:8} {n}")
        return 0

    for a in args:
        s = signals(json.loads(pathlib.Path(a).read_text()))
        print(f"{classify(s):52}  phases={s['phases']} root_type={s['root_type']} "
              f"dependencies={s['dependencies']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
