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

# govulncheck needs its own handling, because "found a vulnerability" and "could
# not run" are different facts that its plain exit code conflates. A govulncheck
# built against an older Go cannot parse a newer stdlib and fails with a package
# error — which reads exactly like a CVE to anyone glancing at the output. That
# happened here, and cost a real "is this a vulnerability?" investigation.
if need govulncheck; then
    if [ -n "$(find . -name '*.go' -not -path './.git/*' -print -quit)" ]; then
        echo "→ govulncheck"
        gv_out="$(govulncheck ./... 2>&1)"; gv_code=$?
        if grep -q 'Loading packages failed\|mismatch between the Go version' <<<"$gv_out"; then
            echo "✗ govulncheck COULD NOT RUN — toolchain mismatch, not a vulnerability." >&2
            echo "  Rebuild it: go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
            fail=1
        elif [ $gv_code -ne 0 ]; then
            echo "$gv_out" | tail -20 >&2
            echo "✗ govulncheck found something" >&2
            fail=1
        else
            echo "  ok"
        fi
    else
        echo "→ govulncheck (no .go files yet)"
    fi
fi

# ADR index and doc links are cheap and whole-repo; re-run them here because a
# pre-commit glob only fires when those paths happen to be staged.
# Coverage is checked here rather than pre-commit: it runs the whole suite, which
# is a push-time cost and not a per-commit one.
if need go; then
    if [ -n "$(find ./internal -name '*_test.go' -print -quit 2>/dev/null)" ]; then
        run "coverage" ./scripts/check-coverage.sh
    fi
fi

if need python3; then
    run "adr index" python3 ../../workspace/scripts/check-adr-index.py docs/adr
    run "doc links" python3 ../../workspace/scripts/check-doc-links.py .
    run "fixtures"  python3 testdata/verify.py
    run "evidence"  python3 scripts/check-evidence-refs.py
fi

exit $fail
