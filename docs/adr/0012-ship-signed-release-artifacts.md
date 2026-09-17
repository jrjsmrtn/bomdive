<!--
SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
SPDX-License-Identifier: Apache-2.0
-->

# 12. Ship signed release artifacts

Date: 2026-09-17

## Status

Accepted on 2026-09-17 by the maintainer, who asked for it after establishing that CI built
nothing anyone could download. Amends [ADR-0011](0011-go-public-under-apache-2.md), which said
plainly that **no artifact is published** and `ships-artifacts` stays `no` — that is now wrong, and
the marker flips to `yes`. Depends on ADR-0011: the flip to public must happen first, for the
reason in *Why this cannot run while private*.

## Context

`go install github.com/jrjsmrtn/bomdive/cmd/bomdive@latest` covers people who have Go. It covers
nobody else — and a BOM is most often read on a CI runner or a jump host where installing a
toolchain to inspect a file is absurd. So the tool that argues BOM readers should be honest was
reachable only by the audience least in need of it.

Shipping binaries changes what this project *is*, which is why it needs a record rather than a
workflow file. It makes the maintainer a distributor: the artifacts are what a supply-chain attack
would target, and this being a supply-chain tool, shipping them unsigned would be an argument
against itself.

## Decision

**Publish signed, attested archives on every `v*` tag, built in GitHub Actions.**

| Choice | Decision | Why |
|---|---|---|
| Platforms | `linux` and `darwin`, `amd64` and `arm64` — four archives | where BOMs are actually read: CI runners and laptops. `CGO_ENABLED=0` makes all four free |
| Windows | **not shipped** | the TUI has never been run on Windows Terminal here; `ls` and `tree` would work, `browse` would ship untested. `go install` remains |
| Build | a hand-rolled matrix loop | no new tool in the trust base, matching the project's pure-Go posture. GoReleaser was considered and rejected as more behaviour than is needed |
| Provenance | **`actions/attest-build-provenance`, SLSA Level 2** | hosted source, CI build, signed provenance — with the build under our control |
| Level 3 | **declined for now** | it requires the SLSA reusable builder to do the building, which puts the generator in the trust base and takes the build out of this repository. Revisit if a consumer asks for L3 |
| Signing | `cosign sign-blob --bundle` over `checksums.txt` | one keyless signature covers every artifact the file lists. cosign v3 emits a bundle; the old `--output-signature` flags were removed |
| SBOM | **one per artifact**, CycloneDX, from the **binary** | a merged SBOM over `dist/` was the first draft and was wrong: the skill's rule is one per released artifact |
| Verification | **in the same run, as a gate** | see below |
| Registries | **none** | no Homebrew tap, no MacPorts port, no container. Each is a separate decision with its own maintenance |

**Every signature is verified in the run that makes it.** `cosign verify-blob` against the bundle,
with the certificate identity pinned to this repository's release workflow and tag ref, and
`gh attestation verify` against a published archive. Signing produces evidence; only verifying
proves the evidence is usable, and a signature no consumer could verify is worse than none —
it looks like protection.

**The SBOM is catalogued from the binary, and CI asserts it.** Measured with syft 1.36.0 on
2026-09-17: a `bomdive` binary yields **15 components**, zero `pkg:github` purls, and
`pkg:golang/stdlib@1.27.1` — the package stdlib advisories match on. Scanning the source tree
instead yields `pkg:github` components and no Go packages at all, which is a plausible-looking
SBOM that answers nothing. The release fails if either property is missing, so that mistake cannot
regress quietly.

⚠ **The whole pipeline was dry-run locally before it was committed**, not written and hoped for:
four archives built, four SBOMs asserted, checksums produced, the archive layout inspected, and
the extracted `darwin/arm64` binary run — it reported `bomdive version v0.1.1-probe` from the tag
and read a fixture. What remains untested is what only GitHub can do: attestation, keyless signing
and the release upload.

## Why this cannot run while private

Keyless signing and provenance write the repository name and the workflow path to the **public
Rekor transparency log**. On a private repository that publishes the one thing the repository is
keeping private. The release job is therefore gated on `!github.event.repository.private`, like
CodeQL, dependency review and Scorecard — so a tag pushed before the flip builds nothing rather
than leaking a name.

## Consequences

**Positive**

- A reader with no Go toolchain can use the tool, which is most of the audience.
- The project's own claims now apply to itself: signed artifacts, verifiable provenance, and an
  SBOM per artifact that a `bomdive` user can open **with bomdive** — it reads its own SBOM, which
  is the dogfooding case worth having.
- OpenSSF Scorecard's `Signed-Releases` check becomes satisfiable rather than structurally zero.

**Negative**

- **Four binaries to stand behind.** `darwin/amd64` and `linux/arm64` are built and never run
  here; a platform-specific defect would reach a user before it reached the maintainer.
- **A release is now a distribution event.** It cannot be quietly retracted: pulling a tag leaves
  the artifacts in caches and the signature in a public log.
- **The EU CRA framing shifts.** ADR-0011 leaned on *open-source steward*; shipping binaries makes
  the question sharper, though unpaid stewardship is still the operative fact. Not legal advice,
  and recorded so a change of circumstances re-opens it.
- The release workflow is the most complex file in the repository and the least exercised — it
  runs once per tag, which is exactly the shape that rots unnoticed.

**Risk**

- **The first real release will fail at something**, most likely attestation permissions or the
  certificate-identity regex in `verify-blob`. That is a prediction, not a hedge: three of the
  four remaining unknowns are things only GitHub's own infrastructure can execute.
