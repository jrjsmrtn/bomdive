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
# VEX sources, cached SEPARATELY in .corpora-cache-vex so the POC-9 and POC-10 re-runs
# over .corpora-cache keep reproducing what they recorded (ADR-0009, POC-11):
#
#   liquibase/vex-repo        Apache-2.0   108 standalone VEX, all triaged not_affected,
#                                          purl references without a version
#   apache/camel (+3 sibs)    Apache-2.0   standalone VEX from Dependency-Track 4.10.1
#   WyattAu/EvergreenImageRegistry  Apache-2.0  out-of-schema values: OpenVEX states and
#                                          justifications, uppercase severities
#   DependencyTrack/hyades-e2e Apache-2.0  a Dependency-Track 4.8.0 export
#   Jahia/saml-authentication-valve  MIT   one hand-maintained project VEX
#   SoftingIndustrial/FOSS    NO LICENCE   standalone VEX from Dependency-Track 4.13/4.14,
#                                          triaged
#   vex.backpatch.moderne.io  NO TERMS     one embedded VEX, 2,057 vulnerabilities
#
# The last two state no reuse terms. They are fetched for local testing only — never
# committed, never redistributed — by the maintainer's decision of 2026-09-11.
#
# Assertion: every document must PARSE, or be a named known-failure with a cause.
# Not a git hook — it needs the network and several hundred files.
#
# Usage: check-corpora.sh [--corpus examples|cdxgen|syft|vex|all] [--refresh]
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

urlpath() { python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe="/"))' "$1"; }

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
        # A PATH IS NOT A URL. liquibase/vex-repo holds paths containing "?" and
        # "%", which gh sent as a query string; GitHub answered 404, the error body was
        # saved as the document, and run_corpus skipped it as "not a BOM" — a download
        # failure reported as a property of the corpus. Encode the path, and on ANY
        # failure leave the file EMPTY so run_corpus reports it as a fetch failure.
        gh api "repos/$repo/contents/$(urlpath "$p")?ref=$ref" \
            --header 'Accept: application/vnd.github.raw' > "$out" 2>/dev/null || : > "$out"
      done
}

branch() { gh api "repos/$1" --jq .default_branch 2>/dev/null; }

# One known file, fetched directly. The tree walk above is unsafe for a very large
# repository: GitHub TRUNCATES a recursive tree, and a file past the cut is silently
# never listed — apache/camel is that size. Cached per FILE, not per directory,
# because several repositories share one destination here.
fetch_file() { # repo path destdir
    local repo="$1" path="$2" dest="$3"
    local out="$dest/$(echo "$repo/$path" | tr '/' '_')"
    [ "$REFRESH" -eq 0 ] && [ -s "$out" ] && return 0
    mkdir -p "$dest"
    gh api "repos/$repo/contents/$(urlpath "$path")" \
        --header 'Accept: application/vnd.github.raw' > "$out" 2>/dev/null || : > "$out"
}

# Python, not curl, and deliberately so. On the machine this was written on, MacPorts
# curl resolved the Moderne feed's CDN name to addresses that never answered, and timed
# out after 34 seconds; macOS's own resolver — which Python uses — returned different
# addresses that answered at once. Which resolver a curl build uses is not something this
# script should depend on. On failure the file is left EMPTY, so run_corpus reports it.
fetch_url() { # url destfile
    [ "$REFRESH" -eq 0 ] && [ -s "$2" ] && return 0
    mkdir -p "$(dirname "$2")"
    python3 - "$1" "$2" <<'PYFETCH' || : > "$2"
import sys, urllib.request
with urllib.request.urlopen(sys.argv[1], timeout=120) as r, open(sys.argv[2], "wb") as f:
    f.write(r.read())
PYFETCH
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

if [ "$WHICH" = all ] || [ "$WHICH" = vex ]; then
    V=.corpora-cache-vex
    fetch liquibase/vex-repo "$(branch liquibase/vex-repo)" '\\.cdx\\.vex\\.json$' "$V/liquibase"
    for r in camel camel-quarkus camel-spring-boot camel-kamelets; do
        fetch_file "apache/$r" "$r-sbom/$r-sbom.vex.json" "$V/camel"
    done
    fetch WyattAu/EvergreenImageRegistry "$(branch WyattAu/EvergreenImageRegistry)" \
        '^compliance/vex/documents/.*\\.json$' "$V/evergreen"
    fetch_file DependencyTrack/hyades-e2e playwright-tests/resources/dtrack-4.8.0.vex.cdx.json "$V/hyades"
    fetch_file Jahia/saml-authentication-valve .security/vex.cdx.json "$V/jahia"
    fetch SoftingIndustrial/FOSS "$(branch SoftingIndustrial/FOSS)" 'vex\\.cdx\\.json$' "$V/softing"
    fetch_url https://vex.backpatch.moderne.io/cyclonedx/backpatch-vex.cdx.json "$V/moderne/backpatch-vex.cdx.json"
    for d in liquibase camel evergreen hyades jahia softing moderne; do
        run_corpus "vex/$d" "$V/$d"
    done
fi

echo "corpora: $total_pass parsed, $total_known known-upstream, $total_fail unexplained"
[ $total_fail -eq 0 ]
