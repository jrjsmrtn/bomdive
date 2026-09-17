#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
#
# SPDX-License-Identifier: Apache-2.0

# ADR-0004 rests on this result, so it is a script that RUNS rather than a transcript.
#
# Two defensible readings of "what is reachable from x" over the cycle x -> y -> x:
#   edge-uniqueness (Cypher's relationship isomorphism): an edge may not repeat, a NODE may.
#                   x -> y -> x is two distinct edges, so x IS reachable from itself.
#   node-uniqueness: a node may not repeat. x is NOT reachable from itself.
# Neither is wrong. bomdive picks node-uniqueness; this proves the two really differ.
#
# Pure Go, CGO_ENABLED=0. Needs network on first run to fetch modernc.org/sqlite.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
cp "$here/poc6-src/cycle-semantics.go.txt" "$work/main.go"
cd "$work"
go mod init poc6 >/dev/null 2>&1
go get modernc.org/sqlite >/dev/null 2>&1
echo "expected:  edge-uniqueness -> 'x y'   node-uniqueness -> 'y'"
CGO_ENABLED=0 go run . | tee "$work/out"

grep -q "edge-uniqueness from x: *x y" "$work/out" \
  || { echo "FAIL: edge-uniqueness did not return 'x y'" >&2; exit 1; }
grep -qE "node-uniqueness from x: *y *$" "$work/out" \
  || { echo "FAIL: node-uniqueness did not return 'y' alone" >&2; exit 1; }
echo "OK: the two semantics differ, as ADR-0004 records"
