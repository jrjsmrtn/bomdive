# 2. Adopt Development Best Practices

Date: 2026-09-10

## Status

Accepted. **Amended 2026-09-17**: section 3 now says what moves each branch — see
[ADR-0010](0010-rename-the-tool-to-bomdive.md) for the rename that produced the commits which
exposed the gap.

## Context

bomdive is a small, single-contributor Go CLI at tier t1. Practices are scaled to that: enough to
keep quality high and make AI-assisted sessions consistent, without ceremony that will be ignored.

This project follows the
[AI-Assisted Project Orchestration patterns](https://github.com/jrjsmrtn/ai-assisted-project-orchestration).

## Decision

### 1. Testing

- **Framework**: Go's `testing`, table-driven.
- **Approach**: TDD for the traversal core, which is where the real complexity is.
- **Fixtures are committed, real, and adversarial.** The test corpus includes BOMs measured to
  contain **cycles**, a BOM whose declared root is absent from the dependency graph, and an OBOM
  with `dependencies: []`. Every one of those broke a naive implementation during analysis; a
  fixture set of well-formed trees would test nothing that matters.
- **Coverage floor**: **≥80% for every package under `internal/`**, enforced by
  `scripts/check-coverage.sh` in the pre-push gate.

  ⚠ **Amended 2026-09-10.** This read ">80% for the traversal and rendering packages, not enforced
  elsewhere", written when there were two packages and no gate. There are now five, and the wording
  had gone stale in the worst way: `internal/tui` sat at **78.5%** — below the stated floor, for as
  long as it existed — and the only reason anyone noticed was a question about whether coverage was
  gated at all. **A threshold nothing enforces is a wish.** Widened to every package because a
  curated list is a second thing to keep in step, and all five clear it.

### 2. Semantic Versioning

[SemVer 2.0.0](https://semver.org/). Patch-level during development (0.1.x). No 1.0 until `ls`
and `tree` are proven against the full fixture corpus and the CLI surface has stopped moving.

### 3. Git Workflow

Gitflow — `main`, `develop`, `feature/*` — matching the `ansible-bom` sibling.
Conventional Commits.

**What moves each branch** — amended 2026-09-17, because naming the branches settled nothing:
work lands on `develop`, and **`main` advances at a release**, which is where the annotated tag
goes. Fast-forwarding `main` between releases is not part of the workflow. It was done three times
by hand on 2026-09-17, once per documentation commit, until the pattern was noticed — and the
suggestion that followed, to flatten the two branches into one, was **refused**: gitflow is this
ADR's decision, and one busy afternoon is not evidence against it.

**A commit message MUST NOT claim more than the commit contains.** Re-read it against
`git diff --cached`, not against intent. The risk is highest when the message is generated,
because fluent prose about an intended change reads identically whether or not the change landed.

### 4. Change Documentation

Keep a Changelog in `CHANGELOG.md`.

### 5. Formatting and Quality Automation

- `gofmt` / `go vet` — non-negotiable, and cheap.
- **Git hooks via lefthook** (see `setup-git-hooks`): pre-commit is fast — format, `go vet`,
  `gitleaks`; pre-push is thorough — `go test`, `staticcheck`, `govulncheck`.
- The repository inherits a baseline `gitleaks` pre-commit hook from `~/.git-template`.

### 6. Documentation

**No Diátaxis tree at t1** — there is nothing to sort into it yet. Documentation is: `README.md`
(the front door), `CLAUDE.md` (session context), this ADR log, and `docs/inception/` (the
analysis and its evidence). The tree arrives with promotion to t2.

**A README or CLAUDE.md MUST NOT assert a number a command can derive.** Counts and percentages
are the fastest-decaying content in a repository and nothing validates them. Point at the evidence
script instead.

### 7. Licensing

**Not applicable at the Private profile.** No `LICENSE`, no REUSE, no SPDX headers. This is a
decision, not an omission — adding them would imply a distribution intent that has not been taken.
Revisit at the `public-release` gate, which needs its own ADR.

### 8. Code Conventions

- **Documentation placeholders**: RFC 5737 for IPs, RFC 7042 for MACs, RFC 2606 for domains.
- **Never commit a real BOM from a private estate.** Test corpora may be measured, but only
  anonymised *shape* is recorded — see `docs/inception/evidence/poc2-corpus-shape.py`, which
  takes the corpus directory as an argument for precisely this reason.
- Interface comments explain what a caller needs without reading the implementation.
  Implementation comments only where the code is non-obvious — the test is whether deleting the
  comment would let someone reintroduce a bug or repeat a rejected approach.

## Consequences

**Positive**: consistent sessions; the adversarial fixture set means the hard cases are tested
first rather than last; hooks catch the mechanical failures.

**Negative**: gitflow is heavier than a single contributor strictly needs; the fixture corpus has
to be generated and committed before much code exists.

## References

- [AI-Assisted Project Orchestration](https://github.com/jrjsmrtn/ai-assisted-project-orchestration)
- [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
