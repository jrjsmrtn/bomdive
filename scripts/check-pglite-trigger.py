#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
#
# SPDX-License-Identifier: Apache-2.0

"""Has the PGlite+AGE watch trigger fired?

Condition 1 of docs/roadmap/watched-pglite-age.md: a Go PGlite binding that is
maintained, or an official libpglite / WASI artifact upstream.

Exit codes
  0   still watched  - nothing changed, keep waiting
  10  LOOK          - a signal moved; a human should re-read the roadmap entry
  2   could not tell - forge unreachable AND --require-forge given

Fail-open is the default so a pre-commit hook does not break offline, and is
DISABLED by --require-forge, which the scheduled sweep passes. That split exists
because a scheduled job that silently fail-opens reports OK for months.
"""
import argparse, json, subprocess, sys
from datetime import datetime, timezone, timedelta

ISSUE = "electric-sql/pglite/issues/89"
# Baselines are EVENTS recorded on 2026-09-10, not a rolling window.
# A rolling window fires the day it is written (elliots was 176d old, inside any
# 180d window) and reports noise forever. "Moved since we looked" cannot go stale.
GO_BINDINGS = {
    "elliots/go-pglite": "2026-03-18T03:21:07Z",
    "tw1nk/go-pglite":   "2025-02-27T21:33:22Z",
}


def gh(path, fixture=None):
    if fixture is not None:
        return fixture.get(path)
    try:
        out = subprocess.run(["gh", "api", path], capture_output=True, text=True, timeout=60)
        if out.returncode != 0:
            return None
        return json.loads(out.stdout)
    except Exception:
        return None


def check(fixture=None, require_forge=False):
    signals, unreachable = [], []

    issue = gh(f"repos/{ISSUE}", fixture)
    if issue is None:
        unreachable.append(ISSUE)
    elif issue.get("state") == "closed":
        signals.append(f"pglite#89 is CLOSED (was open since 2024-05-15) - read why")

    for repo, baseline in GO_BINDINGS.items():
        data = gh(f"repos/{repo}", fixture)
        if data is None:
            unreachable.append(repo)
            continue
        pushed = data.get("pushed_at")
        if not pushed:
            continue
        base_dt = datetime.fromisoformat(baseline.replace("Z", "+00:00"))
        push_dt = datetime.fromisoformat(pushed.replace("Z", "+00:00"))
        if push_dt > base_dt:
            signals.append(
                f"{repo} pushed {pushed} - moved since the {baseline} baseline")

    return signals, unreachable


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--require-forge", action="store_true",
                    help="exit 2 if the forge cannot be reached, instead of fail-open")
    ap.add_argument("--self-test", action="store_true",
                    help="prove the checker reports a fired trigger on planted input")
    args = ap.parse_args()

    if args.self_test:
        ok = True
        # planted known-bad: issue closed + a binding pushed today
        fired = {
            f"repos/{ISSUE}": {"state": "closed"},
            "repos/elliots/go-pglite": {"pushed_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")},
            "repos/tw1nk/go-pglite": {"pushed_at": "2025-02-27T21:33:22Z"},  # unchanged: must NOT fire
        }
        sig, _ = check(fixture=fired)
        print(f"  planted FIRED trigger  -> {len(sig)} signal(s) {'PASS' if len(sig) == 2 else 'FAIL'}")
        ok &= len(sig) == 2
        # planted known-good: nothing moved
        quiet = {
            f"repos/{ISSUE}": {"state": "open"},
            "repos/elliots/go-pglite": {"pushed_at": "2026-03-18T03:21:07Z"},  # exactly the baseline
            "repos/tw1nk/go-pglite": {"pushed_at": "2025-02-27T21:33:22Z"},
        }
        sig2, _ = check(fixture=quiet)
        print(f"  planted QUIET state    -> {len(sig2)} signal(s) {'PASS' if not sig2 else 'FAIL'}")
        ok &= not sig2
        print("SELF-TEST", "PASS" if ok else "FAIL")
        return 0 if ok else 1

    signals, unreachable = check(require_forge=args.require_forge)

    if unreachable and args.require_forge:
        print(f"CANNOT TELL: forge unreachable for {', '.join(unreachable)}", file=sys.stderr)
        return 2
    if unreachable:
        print(f"note: could not reach {', '.join(unreachable)} (fail-open; use --require-forge to gate)")

    if signals:
        print("LOOK - PGlite+AGE watch signals moved:")
        for s in signals:
            print(f"  - {s}")
        print("  re-read docs/roadmap/watched-pglite-age.md and re-check conditions 2 and 3")
        return 10
    print("still watched: no PGlite/Go-binding signal moved")
    return 0


if __name__ == "__main__":
    sys.exit(main())
