# bomdive Roadmap

## Vision

Read a CycloneDX xBOM at the terminal and navigate it the way you navigate a filesystem — answering
*what is in here* and *what pulled this in* without a browser, a query language, or a memorised
schema, and without ever presenting a partial answer as a complete one.

## Phases

### Phase 1 — Read and navigate one BOM
**Target**: v0.1.x · **Status**: **complete** — `ls`, `tree`, text and JSON. `internal/bom` 84.9%, `internal/render` 98.8%, ten planted defects all caught by mutation testing

The traversal core, and the two non-interactive renderers. Everything here is constrained by
decisions already recorded, so this phase is implementation rather than design.

- [x] Parse via `cyclonedx-go` — JSON, spec 1.4–1.7 ([ADR-0003](../adr/0003-go-with-cyclonedx-go.md))
- [x] **BOM-type discrimination** from `metadata.lifecycles` + root component type, including the
      custom-lifecycle form and the undeclared-root branch (POC-7 has a working classifier)
- [x] **Traversal model with lazy, one-level expansion from an arbitrary node** — the constraint
      [ADR-0006](../adr/0006-column-view-as-a-first-class-renderer.md) puts on this phase, so the
      column view is a second renderer rather than a rewrite
- [x] Derived roots from in-degree-0 nodes, labelled synthetic
- [x] Cycle-safe traversal, node-uniqueness ([ADR-0004](../adr/0004-the-dependency-graph-is-a-dag.md))
- [x] `ls` — components, and `cdx:osquery:category` for a graphless BOM, with `--type` filtering
- [x] `tree` — full walk of the same model, refusing informatively on a BOM that declares no graph
- [x] **Coverage as a correctness guarantee**, in both the human and `--json` surfaces
- [x] The 19 committed fixtures pass as red-then-green tests

**Done when**: every fixture renders correctly, including the four cycle cases and the rootless one,
and no output can present a partial graph as complete.

### Phase 2 — The column view
**Target**: v0.2.x · **Status**: **implemented** — `bomdive browse`. Model in `internal/columns` (95.2%), drawing in `internal/tui` (78.5%), driven headlessly on a tcell simulation screen

Miller columns ([ADR-0006](../adr/0006-column-view-as-a-first-class-renderer.md)) — the renderer
that spans both the graph and no-graph cases.

- [x] **Settle the three open questions** — done, by measurement:
      [ADR-0007](../adr/0007-column-view-open-questions.md)
- [x] Column renderer over the Phase 1 model
- [x] Type-to-filter within a column
- [x] Per-category detail layout for an OBOM (no fixed column set — POC-7)

⚠ **One measurement carries a stated limit.** Fan-in was measured only on sparse graphs (81% and
8% coverage). If a dense BOM shows fan-in in the hundreds, reverse mode needs paging that forward
mode does not — re-check before relying on it beyond these shapes.

### Phase 2b — Vulnerability records
**Target**: v0.2.x · **Status**: **implemented** — `browse` and `v`, across every document named —
[ADR-0009](../adr/0009-browse-a-standalone-vex.md)

Browse the vulnerability records a CycloneDX document carries. **Both shapes are produced by
tools**: Dependency-Track 4.x exports standalone VEX, and the 5.1.0 instance measured exports
embedded VEX. **Embedded first**, by the maintainer's decision of 2026-09-11 — it is what that
instance exports, so it can be dogfooded at once, and component → vulnerabilities then needs no
second document. Its export carries only the affected components, up to 1,505 vulnerabilities in
one document and 1,134 on one component
(`../inception/evidence/poc11-vex-navigability-2026-09-11.md`).

- [x] Embedded VEX: from a component, the vulnerabilities that affect it; and the vulnerabilities,
      grouped, then what each affects
- [x] A grouping column only when it splits: analysis state if that gives two or more groups,
      otherwise severity if that does, otherwise none — the view opens on the vulnerabilities.
      `?` says which, and why
- [x] A key to switch between the component axis and the vulnerability axis — `v`. Not `⇥`,
      which reverses dependency edges
- [x] Unresolved references shown in one of five states — including a purl that names a package,
      not a component in the document — with a resolution count
- [x] Values outside the schema shown as they are, and severities sorted regardless of case
- [x] Standalone VEX, within one document
- [x] BOM-Links into documents named on the command line ([B]): a files column when two or more
      are named, and a sixth state, *linked, ambiguous*, for a serial two documents claim

⚠ **Real VEX does not keep to the schema.** Two public sources carry OpenVEX states and
justifications, and uppercase severities. The view must show what it does not recognise, not
reject it or file it under the wrong group.

### Phase 3 — Corpus questions
**Target**: v0.3.x · **Status**: deliberately deferred, may never be built

Cross-BOM questions are answered **today** by the `duckdb` and `lbug` CLIs against the files on
disk, with nothing to keep in sync — proven end to end in
`../inception/evidence/poc6-ladybug-duckdb-bridge.sh`.

- [ ] `export` emitting node/edge tables (Parquet or CSV) — the entire integration surface, and it
      costs no cgo
- [ ] Documented DuckDB and Cypher recipes rather than embedded engines
- [ ] **Only if the recipes prove insufficient**: `-tags duckdb` (the better single engine — it
      earns its place on every BOM type) per [ADR-0005](../adr/0005-engines-behind-build-tags.md)

⚠ **The trigger for embedding is evidence of use, not appetite.** If the recipes go unused, this
phase is complete as documentation and no engine is ever linked.

### Toward 1.0
**Target**: v1.0.0 · **Status**: not scheduled

- [ ] Phases 1 and 2 stable, CLI surface no longer moving
- [x] **XML input claimed** — tested across **1.0–1.7**, which reaches *two versions below* anything
      CycloneDX JSON can express. Format is detected from content, not the file extension
- [x] Performance budget met: `tree` over a 10 MB / ~10⁴-component BOM in under 2 seconds —
      measured **0.05s** end to end on a real 10 MB BOM, and **13ms** for load+walk of a synthetic
      10k-component one. `BenchmarkLoad10k` / `BenchmarkWalk10k` and a budget test make it
      reproducible without depending on any particular file
- [x] A decision on Private → Public — **public since 2026-09-17**
      ([ADR-0011](../adr/0011-go-public-under-apache-2.md))
- [x] **Signed release artifacts** ([ADR-0012](../adr/0012-ship-signed-release-artifacts.md)):
      four platforms, an SBOM per artifact, SLSA L2 provenance, cosign signature, verified in-run
- [x] **Fuzzing the parser and the link resolver**, weekly; seeds run with every test

## Not phases

| | Why it is not scheduled |
|---|---|
| **PGlite + Apache AGE** | A *watched trigger*, not a plan — see [watched-pglite-age.md](watched-pglite-age.md). Reviewed 2027-03-10; `scripts/check-pglite-trigger.py` watches it mechanically |
| ~~**Merged host view**~~ | **Measured 2026-09-10 (POC-8).** It has a real graph reaching 0.6% of the document, and found a root-resolution bug. Nothing further is scheduled: the shape is handled and its fixture is committed |
| SPDX input | Would need normalisation, not a second parser. No demand yet |
| BOM generation, editing, signing, scanning | Out of scope by decision — other tools own these. See the README |

## Tracker boundary

Backlog lives **in-repo only**; this roadmap is canonical. There is one contributor and no tracker.
If that changes, `graduate-backlog` moves per-issue status to a forge and this file keeps the
narrative and version ledger.
