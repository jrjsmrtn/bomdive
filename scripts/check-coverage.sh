#!/usr/bin/env bash
# Enforce the coverage floor ADR-0002 states.
#
# WHY THIS EXISTS. The ADR promised ">80% for the traversal and rendering
# packages" and nothing checked it. On 2026-09-10 internal/tui sat at 78.5% —
# below the stated floor, for as long as it had existed, and the only reason
# anyone noticed was someone asking whether coverage was gated at all. A
# threshold nothing enforces is a wish.
#
# ⚠ WHAT THIS FLOOR ACTUALLY CATCHES. 80% is loose. Deleting an ENTIRE test file
# from internal/columns took it from 95.2% to 84.1% and this check still passed.
# It catches a package going largely untested, not a package getting worse. Real
# regressions are caught by mutation testing, not by this number — coverage says
# which lines RAN, never whether anything asserted on them.
#
# Usage: check-coverage.sh [--threshold N] [--self-test]
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 2

THRESHOLD=80
SELF_TEST=0
while [ $# -gt 0 ]; do
    case "$1" in
        --threshold) THRESHOLD="$2"; shift 2 ;;
        --self-test) SELF_TEST=1; shift ;;
        *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done

command -v go >/dev/null 2>&1 || { echo "✗ go not found — cannot measure coverage" >&2; exit 2; }

# Gate on the parsed number, not on "go test succeeded": a passing suite says
# nothing about how much of the package it touched.
out="$(go test ./internal/... -cover -count=1 2>&1)"
code=$?
if [ $code -ne 0 ]; then
    echo "$out" | tail -20 >&2
    echo "✗ tests failed; coverage is not meaningful" >&2
    exit 1
fi

measured=$(grep -c 'coverage:' <<<"$out")
if [ "$measured" -eq 0 ]; then
    echo "✗ no coverage figures parsed — the check would pass vacuously" >&2
    exit 2
fi

# EVERY package must be accounted for. An earlier version anchored on "^ok", and
# `go test` prints an UNTESTED package with a leading TAB and 0.0% — so a package
# with no tests at all was silently skipped, which is the worst case this floor
# exists to catch. Compare against `go list` rather than trusting the parse.
expected=$(go list ./internal/... 2>/dev/null | sort)
seen=$(sed -nE 's#.*[[:space:]]([^[:space:]]*/internal/[^[:space:]]+).*coverage:.*#\1#p' <<<"$out" | sort -u)
missing=$(comm -23 <(echo "$expected") <(echo "$seen"))
if [ -n "$missing" ]; then
    echo "✗ no coverage reported for:" >&2
    printf '    %s\n' $missing >&2
    echo "  a package absent from the report is not a package that passed" >&2
    exit 1
fi

fail=0
while read -r pkg pct; do
    if awk -v p="$pct" -v t="$THRESHOLD" 'BEGIN{exit !(p+0 < t+0)}'; then
        printf '  ✗ %-28s %5s%%  below the %s%% floor\n' "$pkg" "$pct" "$THRESHOLD" >&2
        fail=1
    elif [ "$SELF_TEST" -eq 0 ]; then
        printf '  ✓ %-28s %5s%%\n' "$pkg" "$pct"
    fi
done < <(sed -nE 's#.*[[:space:]]([^[:space:]]*/internal/[^[:space:]]+)[[:space:]].*coverage: ([0-9.]+)% of statements#\1 \2#p' <<<"$out")

if [ "$SELF_TEST" -eq 1 ]; then
    # A floor of 100 must fail on any package that is not fully covered; if it
    # passes, the check is not reading the numbers it claims to read.
    if [ $fail -eq 1 ]; then
        echo "SELF-TEST PASS: a 100% floor is correctly rejected"
        exit 0
    fi
    echo "SELF-TEST FAIL: nothing was below a 100% floor, so the parser is broken" >&2
    exit 1
fi

[ $fail -eq 0 ] && echo "coverage OK ($measured packages, all >= ${THRESHOLD}%)"
exit $fail
