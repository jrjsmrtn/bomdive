# Quality Configuration

Single source of truth for quality settings. Where a value is declared in a file, this table says
**where** rather than restating it — a copied value has two sources of truth and only one updates.

## Toolchain Versions

| Role | Value | Declared in | Meaning |
|---|---|---|---|
| **Floor** | `go` directive | `go.mod` | the oldest Go a consumer needs — a compatibility claim |
| **Build** | current toolchain | developer machine / CI | what binaries are actually built with |
| **Inherited** | Go 1.25+ | `cyclonedx-go` v0.12.0+ | a floor the dependency forces, not one chosen |

Recorded at bootstrap: `go.mod` floor `1.27.1`, build `go1.27.1`.

**Build with the current version, never the floor.** The floor states what a consumer needs;
building at it ships the least-patched runtime — and for a supply-chain tool, publishing a
known-vulnerable runtime and then documenting it in a BOM would be a poor advertisement.

⚠ **The inherited floor is the binding one.** `cyclonedx-go` v0.12.0+ requires Go 1.25+, so the
`go.mod` directive cannot drop below that regardless of what this project would otherwise support.

## Formatting Standards

| Setting | Value | Applies to |
|---|---|---|
| Indent style | **tabs** | `*.go` (gofmt is not negotiable) |
| Indent style | spaces, 2 | `*.{yml,yaml,json,md}` |
| Indent style | spaces, 4 | `*.py` |
| End of line | lf | all |
| Final newline | yes | all |
| Trim trailing whitespace | yes | all |

Declared in `.editorconfig`.

## Quality Checks by Stage

| Check | Pre-commit | Pre-push | Notes |
|---|---|---|---|
| `gitleaks protect --staged` | ✅ | — | replaces the `~/.git-template` baseline hook, which lefthook displaced |
| `gofmt -l` | ✅ | — | staged `*.go` only |
| `go vet` | ✅ | — | |
| ADR index (shared checker) | ✅ (on `docs/adr/*`) | ✅ | both directions: listed-but-absent **and** present-but-unlisted |
| doc links (shared checker) | ✅ (on `*.md`) | ✅ | extracts links by parsing, not regex |
| `go build` / `go test` | — | ✅ | |
| `govulncheck` | — | ✅ | |

**Shared checkers are called, not copied**: `../../workspace/scripts/check-{adr-index,doc-links}.py`.
One copy at the portfolio root, so there is nothing to drift.

## Two shapes chosen deliberately

**Pre-push gates live in `scripts:`, not `commands:`.** lefthook derives a pre-push file set from
`@{push}`, which git resolves per *branch*, not per *remote*. The first push advances it, so on a
second remote the diff is empty and every `commands:` entry is **skipped while printing a clean
summary**. This repository has one remote today and the convention adds `github` on publication,
so the shape is chosen before the silent failure can happen. Whole-repo checks never needed a file
set anyway.

**`govulncheck` distinguishes "found something" from "could not run".** A govulncheck built
against an older Go cannot parse a newer stdlib and exits non-zero with a *package loading* error —
which reads exactly like a CVE at a glance. That happened on 2026-09-10 and cost a real
investigation. The gate now names the case and says how to fix it:

```
✗ govulncheck COULD NOT RUN — toolchain mismatch, not a vulnerability.
  Rebuild it: go install golang.org/x/vuln/cmd/govulncheck@latest
```

Both branches still fail the push. A tool that cannot run must not look like a pass *or* like a
finding.

**A missing tool fails the gate.** `.lefthook/pre-push/gates.sh` checks for each binary and sets a
failure rather than skipping. A gate that silently skips is worse than no gate, because it reports
success — the failure mode that let a sibling's scheduled check print OK for months without running.

## What is deliberately absent

| Not adopted | Why |
|---|---|
| `staticcheck` | not installed; adding a gate for an absent tool creates a check that fails for the wrong reason |
| `dprint` | installed, but no config in this repo; a format gate with no ruleset is noise |
| REUSE / `reuse lint` | Private profile — no `LICENSE`, so there is nothing to lint |
| CI `audit` job | no forge remote yet; the pre-push `govulncheck` is the whole enforcement today, and it **is bypassable** with `--no-verify` |
| commit-msg honesty hook | a t2 gate; the rule is stated in `CLAUDE.md` and ADR-0002 and currently rests on discipline |

## Not a hook: the PGlite watch

`scripts/check-pglite-trigger.py` is **not wired into any hook, deliberately.** Its trigger is
external state that moves on upstream's timeline, not on this repository's diffs — the same reason a
`stale_after` expiry can never be checked by a commit hook. It needs a **scheduled** run to be
worth anything, and until one exists it is run by hand.

```bash
python3 scripts/check-pglite-trigger.py --self-test   # prove it can report a fired trigger
python3 scripts/check-pglite-trigger.py               # 0 watched, 10 look
python3 scripts/check-pglite-trigger.py --require-forge   # 2 if the forge is unreachable
```

## Two checking traps hit in this project, both silent

Recorded because each produced a **passing-looking result that was wrong**, and both recurred
within a single session.

**`$?` after a pipeline is the last command's status.** `cdx-validate ... | tail` reported `EXIT=0`
when the tool's own exit code was **3**; `cdx-convert ... | tail` said 0 where the real code was 1.
Capture the command's status directly, or the check silently reports on `tail`.

**jq's `//` treats `false` as empty.** A schema check reading `.schemaValid // "?"` printed `?` for
every fixture where `schemaValid` was genuinely `false` — so a real failure rendered as "unknown",
and would have been read as a tooling quirk rather than a defect. Use `if has("x") then .x else …`,
or test the value explicitly.

Both are instances of the same rule: **state the success criterion in terms of the result** — how
many came back, does this exact string appear, did the file change — rather than checking that a
command "ran".

## Mutation testing is how a suite earns trust here

A test suite that has only ever passed is unproven. Every package is checked by planting a defect
and confirming the suite fails — sixteen so far, across `bom`, `render` and `cli`.

It has paid for itself three times, and none of the three would have been found by reading:

- **A test measured the wrong signal.** `TestDanglingRefIsReportedNotInvented` looked for a dangling
  ref appearing in output; the real defect emits a Go *zero value* with an **empty** Ref, so the
  assertion never matched. A test written to catch exactly that class of bug had it.
- **A guard was only caught by a crash.** Removing the cycle guard blew the stack, which kills the
  process, names no test, and is indistinguishable from an unrelated failure. A depth-capped walk
  turned it into an assertion with a diagnostic.
- **It stopped a false claim reaching a commit message.** A comment asserted that package-global
  cobra flags would leak between invocations. Planting them showed they would not — cobra
  re-registers each flag with its default on every run. The comment and the test name were corrected
  to what is actually true.

### The failure that keeps recurring: an unscoped `Contains`

Four escaped mutations across this project share one shape — asserting that a word appears
*somewhere* in a whole output, when the word also appears somewhere else:

| Assertion | Why it passed anyway |
|---|---|
| dangling ref appears in output | a missing map key yields a **zero value**, so the ref never appeared at all |
| `schemaValid // "?"` in jq | `//` treats **`false`** as empty, so a real failure read as "unknown" |
| two property keys in document order | those two keys were **already alphabetical**, so sorting was undetectable |
| "dependencies" on the TUI screen | it is also a **detail-pane key**, so the header could say nothing |

**State the success criterion against the region that is supposed to carry it** — the header, that
column, this field — not against the whole buffer. And when a fixture cannot distinguish two
behaviours, fix the FIXTURE: the corpus now contains a component whose properties are deliberately
not in alphabetical order, for exactly that reason.

⚠ **A mutation that fails to COMPILE proves nothing.** Two did, and were redone as valid code; one
of those was the false claim above. A build error is not a caught defect.

## Validation

```bash
# Both stages run, and both can fail:
lefthook run pre-commit
lefthook run pre-push

# Prove the ADR gate fails rather than only ever passing:
printf '# 6. Planted\n\nDate: 2026-09-10\n' > docs/adr/0006-planted.md
lefthook run pre-push; echo "expect non-zero"
rm docs/adr/0006-planted.md

# Hooks are actually installed (a config listing gates git never calls looks identical to one that works):
ls .git/hooks/pre-commit .git/hooks/pre-push
```

- [ ] `.editorconfig` matches Formatting Standards above
- [ ] `.lefthook.yml` pre-push uses `scripts:`, not `commands:`
