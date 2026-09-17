<!--
SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
SPDX-License-Identifier: Apache-2.0
-->

# 11. Go public under Apache-2.0

Date: 2026-09-17

## Status

**Proposed** on 2026-09-17. The flip itself is the maintainer's, and `CLAUDE.md` makes it so:
Private → Public is an axis of its own, running through the `public-release` gate and needing this
record. Nothing in this ADR makes the repository public; it states the decision being asked for
and what has been done to earn it.

Supersedes the *Private → Public* promotion trigger in `CLAUDE.md`, which named the gate and
deferred the argument. Depends on nothing; [ADR-0010](0010-rename-the-tool-to-bomdive.md) settled
the name this would publish under.

## Context

The project was private because it was unproven — the reason `CLAUDE.md` gives, with `okf-gate` as
precedent. Two things have changed.

**It is proven enough to be judged.** Phases 1, 2 and 2b are implemented: `ls`, `tree` and
`browse`, the column view, the OBOM category axis, the vulnerability axis across documents and
directories. v0.1.0 is tagged. Coverage is at or above 80% in every package, correctness claims
are mutation-tested, and the design rests on measurements that are re-runnable rather than
asserted — the SPARK analysis' own findings reversed the design twice.

**The niche is still empty, and that was re-checked rather than assumed.** ADR-0010's naming search
turned up `safedep/xbom`, a *generator*, which this survey had missed for nine days; the
navigation half remains unoccupied. `CycloneDX/sbom-utility` owns query, in Go, under the CycloneDX
org, and this tool deliberately does not compete there.

**What publishing is for**, stated so it can be judged later: the measurements are the valuable
part. A tool that reports coverage honestly, and refuses to render a partial graph as complete, is
an argument about how BOM readers should behave — and that argument is worth nothing kept private.
The corollary is that going public is not a distribution decision here: **no artifact is
published.** `ships-artifacts` stays `no`.

## Decision

**Publish the source, under Apache-2.0, as a public GitHub repository. Publish no binaries.**

Settled in the preparation, each with its own record:

| Choice | Decision | Why |
|---|---|---|
| Outbound licence | **Apache-2.0** | the patent grant, and it matches the `ansible-bom` sibling |
| Inbound terms | **DCO 1.1**, verbatim, no CLA | one line, CI-enforceable, no contract; a CLA needs a reason and there is none |
| AI-assisted contributions | **permitted, disclosure not required** | the two rules that carry weight are the sign-off and being able to explain every line |
| Artifacts | **none published** | a Go source repository; `go install` needs no registry |
| Work organization | **tracker stays thin** | the roadmap already says what is open; a seeded tracker is surface to keep in sync, and there is one contributor |
| Branch protection | **required status checks on `main`, no required reviews** | CI must pass before `main` moves; requiring a review would mean approving one's own PR or bypassing as admin, and protection routinely bypassed teaches nothing |
| Participation | **Issues only** | the issue forms are written and scope-checked; Discussions with no traffic reads as abandoned, and can open the day a question arrives that is not a defect |

**What stays private, and must:** the BOM corpora. `.corpora-cache*` is git-ignored, every
measurement script takes the corpus directory as an argument and emits counts rather than
identifiers, and no document from a private portfolio is committed. The Dependency-Track
measurements in [ADR-0009](0009-browse-a-standalone-vex.md) record shape only, deliberately. That
constraint does not relax when the repository opens — it tightens.

## Consequences

**Positive**

- The measurements and the correctness obligations become citable, which is the point.
- CI controls that cannot work on a private repository start working: `dependency-review` is
  gated on `!github.event.repository.private` and self-activates, rather than waiting for someone
  to remember.
- `go install github.com/jrjsmrtn/bomdive/cmd/bomdive@latest` becomes the whole installation
  story, with no registry to claim a name on.

**Negative**

- **Issues arrive from strangers.** The scope boundary is the load-bearing defence and it is
  written down in three places — the README, `CONTRIBUTING.md` and the feature-request form —
  because "make it a query tool" is the request that would destroy the project's reason to exist.
- **A published mistake is published.** Every claim in `docs/` is now checkable by people with
  their own corpora, which is a benefit and an exposure at once. The evidence scripts exist so
  that a disagreement can be settled by re-running something.
- **The EU Cyber Resilience Act** frames an unpaid maintainer publishing source as an
  *open-source steward* rather than a manufacturer, which carries lighter duties — but the
  distinction turns on not monetising it. Naming it here so a later change of mind re-opens the
  question rather than sliding past it. This is not legal advice.
- The repository keeps homelab-shaped tooling out of itself by convention, and that convention is
  now load-bearing: the private git server stays the `origin` remote and is never named in a
  tracked file.

**Risk**

- **Going public is effectively irreversible.** A repository can be made private again; a clone
  cannot be recalled. The name, the history and the measurements are out once they are out.

## What the private repository cannot prove

Three controls are gated on `!github.event.repository.private` and have never run, by design:
**CodeQL's SARIF upload** (measured: the analysis itself succeeds and the upload fails with
*Resource not accessible by integration*), **dependency review**, and anything that would write to
the public Rekor transparency log. They self-activate at the flip, which means **the first public
push is also the first time they are exercised** — expect to fix something there rather than
assuming a clean run.

Branch protection is the fourth: on a private repository it needs a paid plan, so it is applied
after the flip, not before.

## Open

- **The flip itself**, which is the maintainer's to make, and this record exists to be read first.
- Repository description and topics — cosmetic, and done with the flip.
