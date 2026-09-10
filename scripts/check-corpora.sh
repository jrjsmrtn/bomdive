#!/usr/bin/env bash
# Run lsxbom over BOMs written by other people, by other tools, for other reasons.
#
# WHY, SEPARATELY FROM check-conformance.sh. That script asks whether we honour the
# SPECIFICATION. This one asks whether we survive REALITY: documents shaped by
# generators we do not control, carrying sections we never read. The two fail
# differently and are worth failing separately.
#
# Every committed fixture in testdata/ is synthetic by design, so each isolates one
# shape. The blind spot that leaves is everything nobody thought to synthesise.
#
#   CycloneDX/sbom-examples   CC0-1.0      83 BOMs — the only PUBLIC OBOM/HBOM/CBOM
#                                          samples found; POC-7's were all private
#   CycloneDX/cdxgen          Apache-2.0   277 test files from the generator whose
#                                          output this workspace actually holds
#   anchore/syft              Apache-2.0   version-identify fixtures, JSON 1.2-1.7 and
#                                          XML 1.0-1.7 — the only XML coverage we have
#
# Assertion: every document must PARSE, or be a named known-failure with a cause.
# Not a git hook — it needs the network and several hundred files.
#
# Usage: check-corpora.sh [--corpus examples|cdxgen|syft|all] [--refresh]
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 2

WHICH=all
REFRESH=0
while [ $# -gt 0 ]; do
    case "$1" in
        --corpus) WHICH="$2"; shift 2 ;;
        --refresh) REFRESH=1; shift ;;
        *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done

command -v gh >/dev/null 2>&1 || { echo "✗ gh not found — cannot fetch corpora" >&2; exit 2; }
BIN="$(mktemp -d)/lsxbom"; trap 'rm -rf "$(dirname "$BIN")"' EXIT
go build -o "$BIN" ./cmd/lsxbom || { echo "✗ build failed" >&2; exit 1; }

# Known failures MUST name a cause, so the list can never quietly absorb our own bugs.
declare -A KNOWN=(
  ["valid-attestation-1.6.json"]="cyclonedx-go v0.12.0 cannot decode declarations.evidence[].data[].classification; CycloneDX/cyclonedx-go#275"
)

fetch() { # repo ref pattern destdir
    local repo="$1" ref="$2" pat="$3" dest="$4"
    [ "$REFRESH" -eq 0 ] && [ -d "$dest" ] && [ -n "$(ls -A "$dest" 2>/dev/null)" ] && return 0
    mkdir -p "$dest"
    gh api "repos/$repo/git/trees/$ref?recursive=1" \
      --jq ".tree[]|select(.type==\"blob\")|select(.path|test(\"$pat\"))|.path" 2>/dev/null \
    | while read -r p; do
        out="$dest/$(echo "$p" | tr '/' '_')"
        [ -s "$out" ] && continue
        # RAW media type, not the base64 .content field: the contents API returns
        # EMPTY content above 1 MB, which silently produced zero-byte files that
        # then failed as "unexpected end of JSON input" — a download failure
        # reported as a parse failure.
        gh api "repos/$repo/contents/$p?ref=$ref" \
            --header 'Accept: application/vnd.github.raw' > "$out" 2>/dev/null
      done
}

total_pass=0 total_fail=0 total_known=0
run_corpus() { # label dir
    local label="$1" dir="$2"
    shopt -s nullglob
    local files=("$dir"/*.json "$dir"/*.xml)
    if [ ${#files[@]} -eq 0 ]; then
        echo "  ✗ $label: no documents fetched — this corpus would pass vacuously" >&2
        total_fail=$((total_fail+1)); return
    fi
    local pass=0 fail=0 known=0 notbom=0
    for f in "${files[@]}"; do
        # XML is claimed as of 2026-09-10 and is no longer skipped. syft's corpus
        # spans 1.0-1.7, which reaches TWO versions below anything JSON can express.
        if [ ! -s "$f" ]; then
            printf '  ✗ %-52s EMPTY FILE — a fetch failure, not a parse failure\n' "$(basename "$f")" >&2
            fail=$((fail+1)); continue
        fi
        # A corpus directory is not necessarily a corpus of BOMs. cdxgen's test/data
        # holds generator INPUTS — package.json, lockfiles, vcpkg manifests — beside
        # its outputs, and counting those as failures said more about this script's
        # assumptions than about lsxbom. Identify a BOM rather than assuming one.
        # A BOM identifies itself by bomFormat in JSON and by its namespace in XML.
        if ! grep -qE '"bomFormat"|cyclonedx\.org/schema/bom' "$f" 2>/dev/null; then
            notbom=$((notbom+1)); continue
        fi
        if "$BIN" ls "$f" >/dev/null 2>&1; then
            pass=$((pass+1))
        elif [ -n "${KNOWN[$(basename "$f")]:-}" ] || [ -n "${KNOWN[${f##*_}]:-}" ]; then
            known=$((known+1))
        else
            fail=$((fail+1))
            printf '  ✗ %-52s %s\n' "$(basename "$f")" "$("$BIN" ls "$f" 2>&1 | head -1 | cut -c1-90)" >&2
        fi
    done
    printf '  %-14s %3d parsed, %d known, %d unexplained' "$label" "$pass" "$known" "$fail"
    [ $notbom -gt 0 ] && printf '  (%d non-BOM files skipped)' "$notbom"
    printf '\n'
    total_pass=$((total_pass+pass)); total_fail=$((total_fail+fail)); total_known=$((total_known+known))
}

CACHE=.corpora-cache
if [ "$WHICH" = all ] || [ "$WHICH" = examples ]; then
    fetch CycloneDX/sbom-examples master '\\.json$' "$CACHE/examples"
    run_corpus "sbom-examples" "$CACHE/examples"
fi
if [ "$WHICH" = all ] || [ "$WHICH" = cdxgen ]; then
    fetch CycloneDX/cdxgen master '^test/data/.*\\.json$' "$CACHE/cdxgen"
    run_corpus "cdxgen" "$CACHE/cdxgen"
fi
if [ "$WHICH" = all ] || [ "$WHICH" = syft ]; then
    fetch anchore/syft main 'format/cyclonedx(json|xml)/testdata/.*\\.(json|xml)$' "$CACHE/syft"
    run_corpus "syft" "$CACHE/syft"
fi

echo "corpora: $total_pass parsed, $total_known known-upstream, $total_fail unexplained"
[ $total_fail -eq 0 ]
