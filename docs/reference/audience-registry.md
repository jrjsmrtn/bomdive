# Audience Registry

Single source of truth for project audiences and their artifact needs. Derived from the
[SPARK analysis](../inception/spark-analysis.md), 2026-09-10.

**Aligned with [`ansible-bom`](../../../ansible-bom/docs/reference/audience-registry.md), 2026-09-10.**
The IDs are deliberately the same, so **A3 names the same person in both repositories** — a packager
is a packager whichever sibling they are holding. The two tools are opposite ends of one pipeline:
`ansible-bom` *produces* a BOM for these people, `lsxbom` lets them *read* one. The audiences
therefore transfer intact and only the verb changes, which is why the registry was aligned rather
than invented.

⚠ **The emphasis shifts, and that is the interesting part.** A4 and A5 are Integration audiences for
a generator, because they consume its output. For a viewer, A4 reads BOMs directly at a terminal and
is **Primary**; A5 is the `--json` consumer and stays Integration.

## Audiences

| ID | Audience | Category | Needs | Derived artifacts |
|----|----------|----------|-------|-------------------|
| A1 | Enterprise IaC / platform engineer | Primary | Read a BOM produced elsewhere and answer *what is in here* and *what pulled this in*, without a browser or a memorised schema | BDD `user-*`, Tutorial, How-to (inspect a delivered BOM), C4:Person |
| A2 | OEM delivery team | Primary | Verify what a delivered BOM actually contains before it ships; see what the document does **not** cover | BDD `user-coverage`, How-to (check a delivery before sign-off) |
| A3 | Packager (RPM/deb, internal mirror, OEM bundle) | Primary | Inspect the component list, versions and identity of what is redistributed; navigate by component type | BDD `user-ls`, Reference (identity: `bom-ref` is the only reliable key) |
| A4 | SecOps / product security | **Primary** | Triage a BOM at a terminal: what depends on this, what is in this category, what is *not* in the graph | BDD `user-tree`, `user-columns`, How-to (trace a component's dependents) |
| A5 | SBOM toolchain (Dependency-Track, grype, jq pipelines) | Integration | Stable `--json` output carrying the same caveats the human surface carries | BDD `api-json`, Reference (JSON shape), conformance corpus |
| A6 | Homelab / small-team operator | Secondary | Same tool, no enterprise tooling; reads OBOMs and HBOMs of their own estate | Tutorial — validation channel, not the design target |
| A7 | Contributor / maintainer | Contribution | Why the graph is a DAG and how `tree` renders it; why the column view is a second renderer; why engines sit behind build tags | Explanation, ADRs, C4:Component view |

## Category definitions

| Category | Focus | Roles here | Primary artifacts |
|----------|-------|-----------|-------------------|
| **Primary** | Reading a BOM directly | A1, A2, A3, A4 | Tutorials, How-tos, user-facing BDD, SystemContext |
| **Integration** | Consuming the tool's output | A5 | Reference docs, `api-*` BDD, Container view |
| **Operational** | Running the tool in a pipeline | folded into A1/A2 — a stateless CLI, not a service | How-tos |
| **Contribution** | Extending the tool | A7 | Explanation, ADRs, Component view |

**Note on the Operational category**: deliberately not populated with a distinct audience, for the
same reason as in `ansible-bom` — a stateless CLI invoked from a shell or CI, with no deployment, no
service to operate, and no operator role separate from A1/A2. Recorded rather than left blank so the
gap reads as a decision.

## Traceability

Every artifact references an audience ID:

- BDD features: `@audience:A1` — **enforced**, see below
- Documentation frontmatter: `audience: A1`
- C4 persons map to Primary and Integration audiences

## The obligation that binds every Primary and Integration audience

**Neither a human reading a tree nor a pipeline parsing JSON can tell a sparse dependency graph from
a complete one.** POC-2 measured real BOMs where only 5% of components appear in the graph, and
**14 of 14 OBOMs with no dependency graph whatsoever**. Coverage is therefore reported in *every*
surface — rendered for A1–A4, a field for A5, the status bar for the column view — never only in
prose that A5 cannot read.

A second obligation follows: **a BOM with no `dependencies` array is a normal document, not a
malformed one.** Every audience must be able to distinguish *this BOM has no graph* from *this tool
found no graph*.

## Artifact coverage matrix

| Audience | BDD | Tutorial | How-to | Reference | Explanation | C4 element |
|----------|-----|----------|--------|-----------|-------------|------------|
| A1 Enterprise IaC | [x] | [ ] | [ ] | - | - | [ ] |
| A2 OEM delivery | [x] | - | [ ] | - | - | [ ] |
| A3 Packager | [x] | - | - | [ ] | [ ] | [ ] |
| A4 SecOps | [x] | - | [ ] | [ ] | - | [ ] |
| A5 SBOM toolchain | [x] | - | - | [ ] | - | [ ] |
| A6 Homelab operator | [x] | [ ] | - | - | - | - |
| A7 Contributor | - | - | - | - | [x] | [ ] |

BDD is covered for every audience owed it, and A7's Explanation is seven ADRs plus five POC evidence
documents. The rest is open — Diátaxis arrives with t2, and the C4 model is not adopted at t1.

⚠ **The BDD column is checked, not claimed.** `scripts/check-audience-tags.py` asserts in both
directions that every feature carries an `@audience` tag naming an ID this registry defines, and
that every audience this matrix marks as owed BDD has at least one scenario. It exists because this
project has already found three promises nothing checked.

---
*Created from SPARK analysis on 2026-09-10 · aligned with `ansible-bom` on 2026-09-10*
