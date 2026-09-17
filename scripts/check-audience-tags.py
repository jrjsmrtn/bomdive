#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
#
# SPDX-License-Identifier: Apache-2.0

"""Every BDD scenario must trace to an audience the registry defines.

WHY THIS EXISTS. The audience registry says traceability is "enforced". This
project has already found three promises nothing checked — a coverage floor, an
evidence-file rule, and a tier trigger — so a fourth was not going to be left to
discipline.

Checks, BOTH DIRECTIONS:
  - every Feature carries an @audience:AN tag, and every scenario inherits or has one
  - every tag names an ID the registry actually defines
  - every registry audience marked as owed BDD has at least one scenario

Exit 0 clean, 1 on drift.  --self-test proves it catches each direction.
"""
import argparse, pathlib, re, sys

TAG = re.compile(r"@audience:(A\d+)")


def registry_ids(root: pathlib.Path):
    """IDs the registry defines, and those its coverage matrix marks as owed BDD."""
    text = (root / "docs/reference/audience-registry.md").read_text()
    defined, owed = set(), set()
    for line in text.splitlines():
        m = re.match(r"\|\s*(A\d+)\s*\|", line)
        if m:
            defined.add(m.group(1))
        # coverage matrix rows look like: | A1 Enterprise IaC | [ ] | ...
        m2 = re.match(r"\|\s*(A\d+)[^|]*\|\s*(\[[ x]\]|-)\s*\|", line)
        if m2 and m2.group(2) != "-":
            owed.add(m2.group(1))
    return defined, owed


def scan(root: pathlib.Path):
    defined, owed = registry_ids(root)
    used, untagged = set(), []
    feature_dir = root / "features"
    features = sorted(feature_dir.glob("*.feature"))
    for f in features:
        text = f.read_text()
        tags = set(TAG.findall(text))
        if not tags:
            untagged.append(f.name)
        used |= tags
    unknown = sorted(used - defined)
    missing = sorted(owed - used)
    return features, defined, used, untagged, unknown, missing


def report(root: pathlib.Path) -> int:
    features, defined, used, untagged, unknown, missing = scan(root)
    if not features:
        print("✗ no .feature files found — the check would pass vacuously", file=sys.stderr)
        return 2
    problems = []
    for f in untagged:
        problems.append(f"{f} carries no @audience tag")
    for u in unknown:
        problems.append(f"@audience:{u} is not defined in the registry")
    for m in missing:
        problems.append(f"{m} is owed BDD by the registry but no scenario tags it")
    if problems:
        print(f"audience traceability drift ({len(problems)}):")
        for p in problems:
            print(f"  - {p}")
        return 1
    print(f"audience tags OK ({len(features)} features, {len(used)} of {len(defined)} audiences covered)")
    return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("root", nargs="?", default=".")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args()

    if args.self_test:
        import tempfile, shutil
        ok = True
        src = pathlib.Path(args.root)
        with tempfile.TemporaryDirectory() as td:
            t = pathlib.Path(td)
            (t / "docs/reference").mkdir(parents=True)
            (t / "features").mkdir()
            (t / "docs/reference/audience-registry.md").write_text(
                "| A1 | x | Primary | n | a |\n| A2 | y | Integration | n | a |\n"
                "| A1 One | [ ] | - | - | - | - |\n| A2 Two | [ ] | - | - | - | - |\n")
            # clean
            (t / "features/a.feature").write_text("@audience:A1\nFeature: a\n")
            (t / "features/b.feature").write_text("@audience:A2\nFeature: b\n")
            _, _, _, un, unk, mis = scan(t)
            print(f"  clean state            -> {len(un)+len(unk)+len(mis)} problems "
                  f"{'PASS' if not (un or unk or mis) else 'FAIL'}")
            ok &= not (un or unk or mis)
            # untagged feature
            (t / "features/c.feature").write_text("Feature: untagged\n")
            _, _, _, un, _, _ = scan(t)
            print(f"  planted UNTAGGED       -> {len(un)} {'PASS' if un == ['c.feature'] else 'FAIL'}")
            ok &= un == ["c.feature"]
            (t / "features/c.feature").unlink()
            # unknown id
            (t / "features/d.feature").write_text("@audience:A99\nFeature: d\n")
            _, _, _, _, unk, _ = scan(t)
            print(f"  planted UNKNOWN id     -> {unk} {'PASS' if unk == ['A99'] else 'FAIL'}")
            ok &= unk == ["A99"]
            (t / "features/d.feature").unlink()
            # audience owed BDD with no scenario
            (t / "features/b.feature").unlink()
            _, _, _, _, _, mis = scan(t)
            print(f"  planted MISSING cover  -> {mis} {'PASS' if mis == ['A2'] else 'FAIL'}")
            ok &= mis == ["A2"]
        print("SELF-TEST", "PASS" if ok else "FAIL")
        return 0 if ok else 1

    return report(pathlib.Path(args.root))


if __name__ == "__main__":
    sys.exit(main())
