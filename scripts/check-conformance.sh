#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
#
# SPDX-License-Identifier: Apache-2.0

# Run bomdive against the CycloneDX specification's own conformance corpus.
#
# The spec repository carries tools/src/test/resources/<version>/ with 359
# valid-*.json and 141 invalid-*.json documents — a POSITIVE and NEGATIVE corpus,
# which is worth more than examples because it exercises the rejection path too.
# Apache-2.0.
#
# WHAT IS ASSERTED, AND WHAT DELIBERATELY IS NOT:
#
#   valid-*    MUST parse. A viewer that cannot open a conformant BOM is broken.
#   invalid-*  are NOT required to be rejected. bomdive is a viewer, not a
#              validator — cdx-validate and sbom-utility own that, and refusing to
#              show a slightly-malformed document would be unhelpful, the way `ls`
#              still lists a directory with a corrupt entry. We never CLAIM a
#              document is valid, which is the part that would be dishonest.
#
# Not a git hook: it needs the network and ~500 documents. Run it when the parser
# or the cyclonedx-go version changes.
#
# Usage: check-conformance.sh [--version 1.6] [--refresh]
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 2

VERSION=1.6
REFRESH=0
while [ $# -gt 0 ]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        --refresh) REFRESH=1; shift ;;
        *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done

CACHE=".conformance-cache/$VERSION"
BIN="$(mktemp -d)/bomdive"; trap 'rm -rf "$(dirname "$BIN")"' EXIT

command -v gh >/dev/null 2>&1 || { echo "✗ gh not found — cannot fetch the corpus" >&2; exit 2; }
go build -o "$BIN" ./cmd/bomdive || { echo "✗ build failed" >&2; exit 1; }

if [ "$REFRESH" -eq 1 ] || [ ! -d "$CACHE" ]; then
    mkdir -p "$CACHE"
    echo "fetching the $VERSION corpus (cached in $CACHE)…"
    gh api "repos/CycloneDX/specification/git/trees/master?recursive=1" \
      --jq ".tree[]|select(.path|test(\"tools/src/test/resources/$VERSION/(valid|invalid)-.*\\\\.json\$\"))|.path" \
      2>/dev/null | while read -r p; do
        out="$CACHE/$(basename "$p")"
        [ -f "$out" ] || gh api "repos/CycloneDX/specification/contents/$p?ref=master" \
            --jq '.content' 2>/dev/null | base64 -d > "$out" 2>/dev/null
    done
fi

shopt -s nullglob
valid=("$CACHE"/valid-*.json)
invalid=("$CACHE"/invalid-*.json)
if [ ${#valid[@]} -eq 0 ]; then
    echo "✗ no valid-*.json in $CACHE — the check would pass vacuously" >&2
    exit 2
fi

# Documents that fail for a reason upstream of us. Each MUST name the cause, so an
# allowlist can never quietly absorb our own regressions.
declare -A KNOWN=(
  ["valid-attestation-1.6.json"]="cyclonedx-go v0.12.0 cannot decode declarations.evidence[].data[].classification; see CycloneDX/cyclonedx-go#275"
)

pass=0; fail=0; known=0
for f in "${valid[@]}"; do
    if "$BIN" ls "$f" >/dev/null 2>&1; then
        pass=$((pass+1))
    elif [ -n "${KNOWN[$(basename "$f")]:-}" ]; then
        known=$((known+1))
        printf '  ~ %-38s known upstream: %s\n' "$(basename "$f")" "${KNOWN[$(basename "$f")]}"
    else
        fail=$((fail+1))
        printf '  ✗ %-38s a conformant document we cannot open\n' "$(basename "$f")" >&2
        "$BIN" ls "$f" 2>&1 | head -1 | sed 's/^/      /' >&2
    fi
done

# Reported, never gated: see the header for why a viewer does not validate.
rejected=0
for f in "${invalid[@]}"; do
    "$BIN" ls "$f" >/dev/null 2>&1 || rejected=$((rejected+1))
done

echo "conformance $VERSION: ${pass}/${#valid[@]} valid parsed, $known known-upstream, $fail unexplained"
echo "  (of ${#invalid[@]} invalid documents, $rejected rejected — reported, not gated: bomdive is a viewer, not a validator)"
[ $fail -eq 0 ]
