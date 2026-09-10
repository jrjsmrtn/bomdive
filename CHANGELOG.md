# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Traversal core** (`internal/bom`): CycloneDX loading across spec 1.4–1.7, BOM-type
  discrimination from `metadata.lifecycles` + root component type, a graph exposing **one level at
  a time in both directions**, roots derived from in-degree-0 nodes and flagged synthetic,
  cycle-safe depth-first walk under node-uniqueness, and coverage accounting. 84.9% covered.

- Project bootstrapped at tier t1.
- SPARK analysis, with re-runnable evidence for every quantitative claim
  (`docs/inception/`).
- Audience registry (A1–A4), including the coverage-reporting obligation that binds both the
  human and the JSON surface.
- Watched trigger for a cgo-free graph backend via PGlite + Apache AGE, with a self-testing
  checker (`docs/roadmap/watched-pglite-age.md`, `scripts/check-pglite-trigger.py`).
- `docs/roadmap/roadmap.md` — three phases plus a 1.0 gate, and an explicit list of what is *not*
  a phase. Adopted at t1 as a deliberate deviation; the rest of t2 stays unadopted.
- Foundation ADRs, plus ADR-0006 recording the Miller-column view as a first-class renderer and
  the lazy-expansion requirement it puts on the traversal model.
- Fixture corpus (`testdata/`): 19 generated CycloneDX fixtures, each isolating one shape
  the traversal core must handle — diamonds, four kinds of cycle, an absent root, a
  graphless OBOM, and `dependencies` empty versus absent. Generated, not hand-written, and
  entirely synthetic, so no fixture derives from a real estate.
- POC-6 backend survey (`docs/inception/evidence/`): DuckDB embedding and zero-ingest reading,
  `zig cc` cross-compilation of cgo, build-tag verification, the Cypher-for-SQLite and DuckPGQ
  survey, and the Ladybug↔DuckDB bridge — with every program preserved and the cycle-semantics
  result made runnable and self-asserting.
- `scripts/check-evidence-refs.py`, asserting in both directions that documents and evidence files
  agree. Written because ADR-0005 had quoted numbers with no evidence behind them.
- POC-7: the BOM-type discrimination rule (`metadata.lifecycles` + root component type) with a
  runnable classifier, the 40-category OBOM navigation vocabulary and its platform split, and the
  merged host view recorded as a **documented-but-unmeasured** exception to ADR-0004.
- `poc6-ladybug-duckdb-bridge.sh` — the LadybugDB↔DuckDB combination made runnable and
  self-asserting, and `poc6-duckdb-corpus.sql` for the zero-ingest queries. Both existed only as
  prose until a review found the survey's most useful result was also its least reproducible.
- `testdata/verify.py`, which asserts each fixture *contains* what its manifest claims, and
  `testdata/validate-schema.py`, which validates each against its own official CycloneDX
  schema across 1.4–1.7.

### Notes

- Renamed from `lsbom` before the first commit: macOS ships `lsbom(8)` for Installer `.bom`
  files — a collision in the same semantic space. `lsxbom` was verified free in `PATH`, MacPorts,
  `man`, and GitHub repository search on 2026-09-10.

[Unreleased]: https://github.com/jrjsmrtn/lsxbom/compare/v0.1.0...HEAD
