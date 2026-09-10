# lsxbom Roadmap

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
**Target**: v0.2.x · **Status**: designed, not started

Miller columns ([ADR-0006](../adr/0006-column-view-as-a-first-class-renderer.md)) — the renderer
that spans both the graph and no-graph cases.

- [ ] **Settle the three open questions first**: column direction (`dependsOn` versus dependents),
      purl truncation, and the leaf detail pane
- [ ] Column renderer over the Phase 1 model
- [ ] Type-to-filter within a column
- [ ] Per-category detail layout for an OBOM (no fixed column set — POC-7)

⚠ **The open questions gate the work.** Direction in particular is not cosmetic: the useful BOM
query is often the reverse edge, which the Finder model has no analogue for.

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
- [ ] XML input claimed only once tested (free from the library, currently unclaimed)
- [ ] Performance budget met: `tree` over a 10 MB / ~10⁴-component BOM in under 2 seconds
- [ ] A decision on Private → Public, which needs its own ADR

## Not phases

| | Why it is not scheduled |
|---|---|
| **PGlite + Apache AGE** | A *watched trigger*, not a plan — see [watched-pglite-age.md](watched-pglite-age.md). Reviewed 2027-03-10; `scripts/check-pglite-trigger.py` watches it mechanically |
| **Merged host view** (`hbom --include-runtime`) | **Documented, never run.** It is the one HBOM/OBOM shape where a host-level `tree` would be meaningful, so it qualifies ADR-0004 — but nothing is scheduled on an unmeasured claim. Measure it first (POC-7) |
| SPDX input | Would need normalisation, not a second parser. No demand yet |
| BOM generation, editing, signing, scanning | Out of scope by decision — other tools own these. See the README |

## Tracker boundary

Backlog lives **in-repo only**; this roadmap is canonical. There is one contributor and no tracker.
If that changes, `graduate-backlog` moves per-issue status to a forge and this file keeps the
narrative and version ledger.
