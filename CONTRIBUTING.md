<!--
SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
SPDX-License-Identifier: Apache-2.0
-->

# Contributing to bomdive

Thanks for considering it. This is a small project with one maintainer, so the fastest path is
usually an issue before a pull request — not ceremony, just so nobody builds something twice.

Participating here means keeping to the [Code of Conduct](CODE_OF_CONDUCT.md), which is short and
says who reads a report.

## Inbound terms: the Developer Certificate of Origin

Contributions arrive under the **[DCO 1.1](https://developercertificate.org/)**, the same licence
as the project ([Apache-2.0](LICENSE)). Sign off every commit:

```bash
git commit -s        # appends: Signed-off-by: Your Name <your@email>
```

Forgot? `git rebase --signoff <base>` fixes a whole branch before you push.

The DCO is a statement that **you have the right to submit the work** — its clauses (a), (b) and
(c) are alternatives, so code you did not write yourself can qualify, provided its licence allows
it and you say where it came from. It is not an authorship claim and grants no patents. Its text
may not be modified, so this project adopts it verbatim rather than paraphrasing it.

There is **no CLA**. Nothing here asks you to sign a contract or to give anyone rights beyond the
project licence.

## AI-assisted contributions

**Permitted, and disclosure is not required.** No trailer is demanded, no threshold applies, and a
patch is judged on the patch.

Two things still hold, and they are the whole policy:

- **The DCO applies unchanged.** You sign off; a tool cannot, and must not be made to appear to.
  You are asserting the right to submit what you send, however it was produced.
- **You must be able to explain every line under review.** This is not an AI rule — it is the
  same bar a hand-written patch meets, and it is the one test that fails a contribution nobody
  understands.

This repository's own history is AI-assisted throughout and says so: commits carry
`Co-Authored-By: Claude …`. That is the maintainer's convention, offered as an example, not a
requirement placed on you.

## Development setup

Pure Go, no cgo in the default build, no code generation:

```bash
go build ./...
go test ./...
lefthook install          # the two-stage gates below
```

The gates that must pass before a push — the same ones CI runs:

```bash
gofmt -l .                       # must print nothing
go vet ./...
go test ./...
./scripts/check-coverage.sh      # every internal/ package at 80% or above
./scripts/check-conformance.sh   # the CycloneDX specification's own corpus
reuse lint                       # SPDX headers on every file
```

Fixtures are generated, not hand-edited: change `testdata/generate.py`, re-run it, and
`testdata/verify.py` plus `testdata/validate-schema.py` will hold you to what the manifest claims.

## Branching

Gitflow: work branches from `develop`, `main` advances at releases
([ADR-0002](docs/adr/0002-adopt-development-best-practices.md)). Branch names are
`feature/<short-description>`. Conventional Commits for messages, and one rule beyond the grammar:
**a commit message must not claim more than the commit contains.** Re-read it against
`git diff --cached`.

## What this project is not

Before proposing a feature, check it is not deliberately out of scope. bomdive reads and navigates
CycloneDX documents. It is **not** a BOM generator, a query tool (`sbom-utility` owns that), a
vulnerability scanner, a signer, or a converter — see [`README.md`](README.md) and
[`CLAUDE.md`](CLAUDE.md). A feature request in one of those directions will be declined with a
pointer to the tool that does it.

Traversal semantics, what counts as a correctness guarantee, and anything touching the decision
log are maintainer-led. Everything else — parsing, rendering, tests, fixtures, docs — is open.

## Bugs

A bug report needs the document that triggered it, or its shape if it cannot be shared: this tool
reads other people's BOMs, and almost every defect found so far came from a document that did
something the specification permits and nobody expected. **Never attach a BOM you cannot publish.**
`bomdive ls --json` output, with names redacted, is usually enough.

Include: the command, what you expected, what happened, and `bomdive --version`.
