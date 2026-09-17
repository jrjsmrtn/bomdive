#!/usr/bin/env python3
"""How many BOMs carry no `components` at all, and what do they carry instead?

WHY. bomdive identified every such document as an "SBOM" and drew an empty component
list — "0 of 0 (0%)" over a blank pane, which reads as the tool having failed rather than
as the document having no inventory. checkIsBOM's own comment calls that shape "the
confidently-wrong shape this tool exists to avoid"; it was being reached one step later.

Cross-tabulates `components` against `vulnerabilities`, and reports what the
component-less documents hold instead. Counts documents, never identifiers, so it is safe
to run over a private corpus.

    python3 docs/inception/evidence/poc10-documents-without-components.py .corpora-cache

With --check-labels BINARY it also runs `BINARY ls` over every VEX-shaped document and
counts the ones whose first line is not "VEX", exiting 1 if there are any. That is the
check which caught the first VEX fix reaching only 19 of 25 documents.

    python3 ... .corpora-cache --check-labels ~/.local/bin/bomdive
"""
import collections
import glob
import json
import os
import subprocess
import sys


def bucket(n, present):
    if not present:
        return "absent"
    return "0" if n == 0 else ">0"


def main(root: str, binary: str | None = None) -> int:
    vex_paths = []
    grid = collections.Counter()
    payloads = collections.Counter()
    scanned = skipped = 0

    for path in sorted(glob.glob(os.path.join(root, "*", "*.json"))):
        try:
            doc = json.load(open(path))
        except Exception:
            skipped += 1
            continue
        if not isinstance(doc, dict) or doc.get("bomFormat") != "CycloneDX":
            skipped += 1
            continue
        scanned += 1

        comps = doc.get("components") or []
        vulns = doc.get("vulnerabilities") or []
        grid[(bucket(len(comps), "components" in doc),
              bucket(len(vulns), "vulnerabilities" in doc))] += 1

        if comps:
            continue
        # What does a document with no inventory actually hold?
        if vulns:
            payloads["vulnerabilities (standalone VEX)"] += 1
            vex_paths.append(path)
        elif doc.get("services"):
            payloads["services (SaaSBOM shape)"] += 1
        else:
            payloads["metadata only"] += 1

    print(f"CycloneDX JSON documents scanned : {scanned}   (skipped: {skipped})")
    print()
    print("components x vulnerabilities:")
    for (c, v), n in sorted(grid.items()):
        print(f"  components={c:<6} vulnerabilities={v:<6} : {n}")
    print()
    print("documents with NO components carry:")
    for what, n in payloads.most_common():
        print(f"  {what:<36}: {n}")

    if not binary:
        return 0
    # Measure the TOOL, not the corpus: what the built binary actually prints first.
    wrong = []
    for path in vex_paths:
        out = subprocess.run([binary, "ls", path], capture_output=True, text=True)
        first = out.stdout.splitlines()[0] if out.stdout else f"(exit {out.returncode})"
        if not first.startswith("VEX"):
            wrong.append((os.path.basename(path), first.split(" ")[0]))
    print()
    print(f"{binary}: {len(vex_paths) - len(wrong)} of {len(vex_paths)} "
          f"VEX-shaped documents labelled VEX")
    for name, label in wrong:
        print(f"  labelled {label}: {name}")
    return 1 if wrong else 0


if __name__ == "__main__":
    args = sys.argv[1:]
    binary = None
    if "--check-labels" in args:
        i = args.index("--check-labels")
        binary = args[i + 1]
        del args[i:i + 2]
    sys.exit(main(args[0] if args else ".corpora-cache", binary))
