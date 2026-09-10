# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- Bumped `golang.org/x/text` 0.21.0 → 0.39.0 for **GO-2026-5970** (infinite loop on invalid input),
  pulled in transitively by the TUI library. Not reachable from this code, but a supply-chain tool
  shipping a known-vulnerable dependency is a poor look and the fix was free.

### Fixed

- **A BOM with no relations was reported as a defective one.** The column view coloured any
  document whose coverage was under 100% red `partial`, which is right for a declared dependency
  graph that misses components and wrong for a document carrying no `dependencies` field at all —
  there, nothing is missing, because nothing was ever declared. Found while browsing a syft
  fixture, where `0/1 (0%) partial` reads as a broken tool. There are now four states
  (`bom.GraphState`), decided once: complete, `partial`, `no dependency graph` (`dependencies`
  present and empty — an assertion), and `relations undeclared` (the field absent — silence).
- **Only `tree` explained why coverage was zero.** The absent-versus-empty distinction was written
  out by the tree renderer alone, so `ls` and the column view printed a bare `0%` with nothing to
  read it by. The explanation now comes from `render.New`, which is the same argument that put
  coverage in that type: a caveat a renderer must remember to add is a caveat that gets forgotten.
- **The column view offered a category axis to documents with no categories**, grouping everything
  into one `(no category)` bucket — a navigation step that says nothing, under a title claiming a
  structure the document does not have. It now falls back to a flat component list, and `tree` and
  `ls` stop suggesting `--by-category` where it would not help. With no edges at all the entry
  column is titled `components` rather than `roots (derived)`, since every component trivially has
  in-degree zero and calling that a root implies a hierarchy.
- **The detail pane could not be scrolled**, so a component with more properties than fit the screen
  was silently cut off — a real HBOM device carries up to 37. `PgDn`/`PgUp`, `Ctrl-D`/`Ctrl-U`,
  `J`/`K`, `Home`/`End`, and the pane title now reports `n-m of total` with `↑`/`↓`, because a pane
  truncated without an indicator looks complete. The pane **wraps**, so how far it can scroll is
  measured by wrapping each key and value at the pane's own width — an earlier estimate of two rows
  per field undercounted a real OBOM, whose values run past 70 characters in a pane around 55 wide,
  and a pane that overflowed by wrapping alone still refused to move.
- A component with **no name** rendered as nothing — a blank row in the column view, an empty
  segment in the header path, `@1.0.0` from `ls`, and a bare `├──` from `tree`. `bom.Node.Label()`
  is now the single place that decides, falling back to `bom-ref`, and filtering matches it too so
  such a component can be found at all.

### Added

- **`?` in `browse` explains the status bar**, for the document in front of you: what the coverage
  numbers mean, why the leftmost column shows what it shows, any dangling `dependsOn` targets, and
  the key bindings. The status bar has room for one word, and a word a reader cannot expand is
  jargon — `partial` in particular meant nothing without it.
- **XML input**, spec **1.0–1.7** — two versions below anything CycloneDX JSON can express. Format is
  detected from **content**, not the file extension, and a UTF-8 byte order mark is tolerated.
- **BDD with Gherkin** (godog): 14 scenarios across `user-ls`, `user-tree` and `api-json`, driving
  the CLI in-process. `Strict: true`, so an undefined step fails rather than passing quietly.
- Audience registry **aligned with `ansible-bom`** — A1–A7 IDs unchanged, so an ID names the same
  person in both repositories. A4 moves from Integration to Primary, because a viewer's security
  engineer reads BOMs rather than consuming output.
- `scripts/check-audience-tags.py` enforcing traceability in both directions.
- `scripts/check-corpora.sh` — runs lsxbom over 137 real BOMs from `sbom-examples` (CC0-1.0),
  cdxgen and syft. Brings the first **public** OBOM, HBOM, CBOM, SaaSBOM and MBOM documents into the
  test set; POC-7's samples were all private.
- `scripts/check-conformance.sh` — runs lsxbom against the CycloneDX specification's own
  conformance corpus (359 valid / 141 invalid documents). Found an upstream `cyclonedx-go` defect on
  its first run: a document the spec calls valid fails to decode.
- `scripts/check-coverage.sh` — a ≥80% floor for every package under `internal/`, wired into the
  pre-push gate and reconciled against `go list`, so a package with no tests at all cannot go
  unreported. ADR-0002 amended to match what is enforced.
- `--cpuprofile` and `--memprofile` on every command, plus benchmarks that accept the same. Stdlib
  `runtime/pprof`, no new dependency, no effect on output — and profiles are written even when the
  command fails, which is when they are most wanted.

### Performance

- `Walk` was **O(n²)** on a deep dependency chain: path membership was a linear scan of a slice.
  With a set it is **41× faster** — 115ms → 2.8ms over a 10k-component BOM with a 10k-deep spine.
  Reproducible benchmarks (`BenchmarkLoad10k`, `BenchmarkWalk10k`) and a budget test now guard it.

### Added

- **`lsxbom browse`** — the Finder-style column view. Each column lists the children of the
  selection to its left, so the chain of columns *is* the dependency path. Tab flips the whole view
  between dependencies and dependents; `/` filters a column; a BOM with no dependency graph opens on
  its osquery categories. Coverage is on screen, because a TUI is a third surface and the guarantee
  is not per-surface.

- CLI smoke tests (`internal/cli`, 97.0%): every flag verified to reach the renderer, error paths
  to exit non-zero with stderr-only diagnostics, and stdout kept clean on failure so a `--json`
  pipeline is never corrupted.
- **`ls` and `tree`** (`internal/cli`, `internal/render`): `ls` lists components or one level of
  dependencies and groups a graphless BOM by `cdx:osquery:category`; `tree` walks the DAG, expanding
  a shared component once and back-referencing it after, marking cycles distinctly, and **refusing
  informatively** rather than printing nothing when a BOM declares no graph. `--json` and `--long`.
- **Coverage as a structural guarantee**: both surfaces render from one `Result` type, so neither
  can omit coverage, synthetic-root or no-graph caveats. A 9,314-component BOM reports
  "829 of 9314 components in the dependency graph (8%)" rather than a clean-looking partial tree.
- A document that parses as JSON but is not a CycloneDX BOM is now **rejected**, where it previously
  rendered as an empty BOM with exit 0.
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
- ADR-0007 settling the column view's three open questions by measurement: direction is a
  whole-view mode (fan-in is *smaller* than fan-out — avg 2.0 vs 3.5), columns show the name
  truncated from the head (tail-truncation gives 52 collisions where head gives 395 and
  middle-elision 100), and the detail pane has no per-category schema (39 categories, 1–37 keys).
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
