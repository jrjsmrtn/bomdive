# Audience Registry — lsxbom

Single source of truth for project audiences and their artifact needs.
Derived from the [SPARK analysis](../inception/spark-analysis.md), 2026-09-10.

## Audiences

| ID | Audience | Category | Needs | Derived Artifacts |
|----|----------|----------|-------|-------------------|
| **A1** | BOM reader at a terminal | Primary | Answer *"what is in this BOM, and what pulled this in?"* without a browser or a memorised schema | BDD:`user-ls`, `user-tree`; Tutorial:*; C4:Person |
| **A2** | CI / pipeline author | Integration | Stable `--output json` contract, meaningful exit codes, no interactive prompts | BDD:`api-json-output`, `api-exit-codes`; Reference:*; C4:ExternalSystem (CI runner) |
| **A3** | Packager / installer | Operational | A single static binary, reproducible build, no cgo, no runtime deps | Howto:`install`, `build-from-source`; C4:Deployment node |
| **A4** | Go contributor | Contribution | Understand *why* the walk is cycle-safe and the root synthetic — the non-obvious core | Explanation:`graph-model`; ADRs; C4:Component view |

⚠ **A3 is deliberately thin.** This is a single-binary CLI with no service to run, so the
Operational category collapses to installation and build reproducibility. It is recorded rather than
omitted so that the absence is a decision, not a gap.

⚠ **A1 and the author are currently the same person.** This is honest, not an oversight — the
usability assumption in the SPARK analysis ("the `ls`/`tree` metaphor is genuinely more usable than a
query language") is untested on a second human and is flagged there as a High-priority assumption.

## Category definitions

| Category | Focus | Typical Roles | Primary Artifacts |
|----------|-------|---------------|-------------------|
| **Primary** | Using the system | End-users, consumers | Tutorials, User BDD, SystemContext |
| **Integration** | Connecting to the system | Developers, API consumers | Reference docs, API BDD, Container view |
| **Operational** | Running the system | Sysadmins, operators, SREs | How-tos, Ops BDD, Deployment view |
| **Contribution** | Extending the system | Contributors, maintainers | Explanation, ADRs, Component view |

## Traceability

Every artifact should reference an audience ID:

- BDD features: `@audience:A1`
- Documentation frontmatter: `audience: A1`
- C4 persons/actors map to Primary/Integration audiences

## Artifact Coverage Matrix

| Audience | BDD | Tutorial | How-to | Reference | Explanation | C4 Element |
|----------|-----|----------|--------|-----------|-------------|------------|
| A1 | [ ] | [ ] | - | - | - | [ ] |
| A2 | [ ] | - | - | [ ] | - | [ ] |
| A3 | - | - | [ ] | - | - | [ ] |
| A4 | - | - | - | - | [ ] | [ ] |

Nothing is built yet — every box is open by construction, not by neglect.

## Audience-specific correctness obligation

One obligation cuts across A1 and A2 and is recorded here because it is an *audience* property, not
an implementation detail:

**Neither a human reading a tree nor a pipeline parsing JSON can tell a sparse dependency graph from
a complete one.** The SPARK measurements found real BOMs where only 5% of components appear in the
graph — and POC-2 found **14 of 14 OBOMs with no dependency graph whatsoever**, across thousands of
components each. Coverage must therefore be reported in **both** surfaces — rendered for A1, and a
field for A2 — never only in prose that A2 cannot read.

A second obligation follows from the same measurement: **a BOM with no `dependencies` array is a
normal document, not a malformed one.** Both audiences must be able to distinguish "this BOM has no
graph" from "this tool found no graph" — for A1 in the rendered message, for A2 in the exit code.

---
*Created from SPARK analysis on 2026-09-10*
*Last updated: 2026-09-10*
