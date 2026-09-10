#!/usr/bin/env bash
# Pre-push gates. Each check FAILS on a missing tool rather than passing quietly —
# a gate that silently skips is worse than no gate, because it reports success.
set -uo pipefail
fail=0

need() {
    command -v "$1" >/dev/null 2>&1 && return 0
    echo "✗ $1 not found — this gate cannot run, and is not being skipped" >&2
    fail=1
    return 1
}

run() {
    local label="$1"; shift
    echo "→ $label"
    if ! "$@"; then echo "✗ $label failed" >&2; fail=1; else echo "  ok"; fi
}

# No Go files yet is a legitimate state during bootstrap; an absent toolchain is not.
if need go; then
    if compgen -G "**/*.go" >/dev/null 2>&1 || [ -n "$(find . -name '*.go' -not -path './.git/*' -print -quit)" ]; then
        run "go build"   go build ./...
        run "go test"    go test ./...
    else
        echo "→ go build/test (no .go files yet — nothing to build)"
    fi
fi

if need govulncheck; then
    if [ -n "$(find . -name '*.go' -not -path './.git/*' -print -quit)" ]; then
        run "govulncheck" govulncheck ./...
    else
        echo "→ govulncheck (no .go files yet)"
    fi
fi

# ADR index and doc links are cheap and whole-repo; re-run them here because a
# pre-commit glob only fires when those paths happen to be staged.
if need python3; then
    run "adr index" python3 ../../workspace/scripts/check-adr-index.py docs/adr
    run "doc links" python3 ../../workspace/scripts/check-doc-links.py .
    run "fixtures"  python3 testdata/verify.py
fi

exit $fail
