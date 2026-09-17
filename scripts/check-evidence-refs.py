#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
#
# SPDX-License-Identifier: Apache-2.0

"""Every evidence file the docs name must exist, and every evidence file must be named.

WHY THIS EXISTS. CLAUDE.md promises "every quantitative claim has a re-runnable script in
docs/inception/evidence/". That promise was FALSE for three days: ADR-0005 and CLAUDE.md
quoted DuckDB sizes, zig cross-compilation results and the cycle-semantics split while the
only evidence file on disk covered LadybugDB linking. Nothing objected, because nothing
was checking. A human noticed.

BOTH DIRECTIONS, deliberately:
  - a doc naming a file that does not exist is a broken promise;
  - an evidence file nothing names is orphaned, and will be deleted by someone tidying up
    who cannot tell what depends on it.

Exit 0 clean, 1 on drift.  --self-test proves it catches both directions.
"""
import argparse, pathlib, re, sys

EVIDENCE = pathlib.Path("docs/inception/evidence")
REF = re.compile(r'`(docs/inception/evidence/[A-Za-z0-9._/-]+)`')
# Markdown links, too: [text](path) and [text](../evidence/path)
LINK = re.compile(r'\]\(([^)]*evidence/[A-Za-z0-9._/-]+)\)')


def scan(root: pathlib.Path):
    named, missing = set(), []
    for md in sorted(root.rglob("*.md")):
        if ".schema-cache" in md.parts:
            continue
        text = md.read_text()
        refs = set(REF.findall(text))
        # A doc INSIDE the evidence directory names its siblings by bare filename —
        # `poc6-cycle-semantics.sh`, not the full path. Resolve those relative to the
        # document's own directory, but only count them when they really resolve to a
        # file under the evidence tree, so ordinary backticked prose is not swept in.
        for token in re.findall(r'`([A-Za-z0-9._][A-Za-z0-9._/-]*)`', text):
            cand = (md.parent / token)
            if cand.is_file():
                try:
                    rel_to_root = cand.resolve().relative_to((root / EVIDENCE).resolve())
                    refs.add(str(EVIDENCE / rel_to_root))
                except ValueError:
                    pass
        for rel in LINK.findall(text):
            # resolve a link written relative to the file that contains it
            cand = (md.parent / rel).resolve()
            try:
                refs.add(str(cand.relative_to(root.resolve())))
            except ValueError:
                pass
        for ref in refs:
            p = root / ref
            if p.exists():
                named.add(str(pathlib.Path(ref)))
            else:
                missing.append(f"{md.relative_to(root)} names {ref}, which does not exist")

    on_disk = set()
    ev = root / EVIDENCE
    if ev.is_dir():
        # The directory's own index is its map; requiring another doc to name it
        # would be circular. It is reached from the SPARK analysis instead.
        on_disk = {str(p.relative_to(root)) for p in ev.rglob("*")
                   if p.is_file() and p.name != "README.md"}
    orphaned = sorted(on_disk - named)
    return missing, orphaned, len(on_disk)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("root", nargs="?", default=".")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args()

    if args.self_test:
        import tempfile, os
        ok = True
        with tempfile.TemporaryDirectory() as td:
            r = pathlib.Path(td)
            (r / EVIDENCE).mkdir(parents=True)
            (r / EVIDENCE / "real.md").write_text("x")
            # planted: a doc naming a file that is not there
            (r / "doc.md").write_text(
                "see `docs/inception/evidence/real.md` and `docs/inception/evidence/ghost.md`")
            miss, orph, _ = scan(r)
            print(f"  planted MISSING ref  -> {len(miss)} {'PASS' if len(miss) == 1 else 'FAIL'}")
            ok &= len(miss) == 1
            # planted: an evidence file nothing names
            (r / EVIDENCE / "orphan.md").write_text("y")
            (r / "doc.md").write_text("see `docs/inception/evidence/real.md`")
            miss2, orph2, _ = scan(r)
            print(f"  planted ORPHAN file  -> {len(orph2)} {'PASS' if orph2 == ['docs/inception/evidence/orphan.md'] else 'FAIL'}")
            ok &= orph2 == ["docs/inception/evidence/orphan.md"]
            # clean case must be clean
            (r / EVIDENCE / "orphan.md").unlink()
            miss3, orph3, _ = scan(r)
            print(f"  clean state          -> {len(miss3)} missing, {len(orph3)} orphaned "
                  f"{'PASS' if not miss3 and not orph3 else 'FAIL'}")
            ok &= not miss3 and not orph3
        print("SELF-TEST", "PASS" if ok else "FAIL")
        return 0 if ok else 1

    missing, orphaned, total = scan(pathlib.Path(args.root))
    if missing or orphaned:
        print(f"evidence reference drift ({len(missing) + len(orphaned)}):")
        for m in missing:
            print(f"  - {m}")
        for o in orphaned:
            print(f"  - {o} exists but no document names it")
        return 1
    print(f"evidence refs OK ({total} file(s), all named, all present)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
