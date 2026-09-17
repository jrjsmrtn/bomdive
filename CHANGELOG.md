# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-09-17

### Changed

- **Renamed from `lsxbom` to `bomdive`** ([ADR-0010](docs/adr/0010-rename-the-tool-to-bomdive.md)).
  The command, the Go module `github.com/jrjsmrtn/bomdive` and `cmd/bomdive` all change; `ls`,
  `tree` and `browse` stay as subcommands. `xbom` was measured and rejected: `safedep/xbom` already
  installs that command and generates BOMs, which this tool deliberately does not. `bomdive` was
  free on GitHub, Codeberg, npm, PyPI, crates.io, Homebrew core and MacPorts, and is not a command
  on the development machine. Every record was rewritten to the new name except two passages that
  say which name was checked on which date.

### Security

- Bumped `golang.org/x/text` 0.21.0 → 0.39.0 for **GO-2026-5970** (infinite loop on invalid input),
  pulled in transitively by the TUI library. Not reachable from this code, but a supply-chain tool
  shipping a known-vulnerable dependency is a poor look and the fix was free.

### Fixed

- **Named files with no components could not be opened in `browse`.** A file opened on its
  component list, which is empty for a standalone VEX and for a product BOM that only names its
  product, so CISA case 8's three files all showed no arrow. A document with no components now opens
  on its vulnerability records, else its subject, and `?` says which. A record row now behaves the
  same in the component view as on the vulnerability axis: it descends to what it affects and shows
  its own detail.
- **The `browse` header pushed its path line off the screen at 80 columns.** The header is two rows,
  and a first line that wrapped took the second: an OBOM's identity alone is 58 characters, so
  `showing: dependencies` beside it already wrapped. The identity is now trimmed to fit — never the
  direction, which changes with every flip and must stay visible.
- **`check-corpora.sh` saved GitHub's `404` responses as documents.** A path containing `?` or
  `%` was sent as a query string, the error body was written where the document belonged, and the
  run skipped it as "not a BOM" — a download failure reported as a property of the corpus, the
  fourth bug of that kind in this script. Paths are now percent-encoded, and any failed download
  leaves an empty file, which the run already reports as a fetch failure; a planted missing path
  proves it. No path in the three existing corpora changes under the encoding, so their results are
  unaffected. A fetched URL now goes through Python, not curl: one curl build resolved the Moderne
  feed's CDN name to addresses that never answered.
- **A CycloneDX document that is not a bill of materials was called an SBOM and drawn as a blank
  pane.** `bomFormat: "CycloneDX"` is a format marker, not a claim of BOM-ness — CycloneDX also
  carries VEX, and a standalone VEX inventories nothing. `identify()` defaulted to `SBOM` and had
  no rule for it, so `browse` on a CISA VEX use case showed `SBOM (root application; …)` over an
  empty column and `coverage 0/0 (0%)`, which reads as the tool having failed. It is now identified
  as `VEX (a CycloneDX document, not a bill of materials: it inventories nothing)` in **all 25**
  corpus VEX documents. **The first version of this fix reached only 19 of 25**: `identify()`
  returned early on a document with no `metadata` — which is optional — before deciding a kind at
  all, so the six VEX documents without it were still called `SBOM`. Found when two new fixtures
  with no metadata came out labelled `SBOM` over a note saying they were not bills of materials.
  The kind is now decided before the metadata check. The rule keys on the **absence of
  components**, not the presence of vulnerabilities, so an SBOM with embedded VEX stays an SBOM;
  and a declared root type still outranks it, so an OBOM carrying vulnerability records stays an
  OBOM.
- **An empty component list now says why, on every surface.** A standalone VEX, a services-only
  document and one carrying nothing but metadata all list zero components and all rendered
  identically. Measured across the public corpora: **40 of 137 documents** carry no components —
  25 VEX, 14 metadata-only, 1 services-only. Each explains itself differently, and the `browse`
  status bar reads `no components` rather than reporting a graph state for a document with nothing
  to relate. Evidence: `docs/inception/evidence/poc10-empty-documents-2026-09-11.md`.
- **A document's declared root was invisible to every reverse edge.** `metadata.component` is the
  *subject* of a BOM, not a member of its inventory, so it is routinely absent from `components`
  while still driving `dependencies`. `bom.Node` synthesised such a root (POC-8) but `resolve` —
  the single funnel behind `Children`, `Parents` and `Reachable` — read the component map directly,
  so a component the root depends on reported **no dependents at all**. Measured across the public
  corpora: **58 of 137 documents (42%)** carry that shape, and **1,658 components** were affected;
  the worst single document hid the root from 71 of its own direct dependencies. `resolve` now asks
  `Node`, so one rule decides what a ref names. `Coverage` deliberately still counts only the
  inventory — counting the root would put `InGraph` above `Components`. Evidence:
  `docs/inception/evidence/poc9-reverse-edges-2026-09-11.md`.
- **A BOM with 278 dangling `dependsOn` targets printed all 278**, filling the `browse` overlay to
  the full screen and pushing the explanation it annotates off the top. Both human-readable
  surfaces now name five and count the rest; `--json` still carries the full list. The overlay also
  scrolls (`⇞`/`⇟`, `J`/`K`) and reports `n-m of total` when it does not fit, so a clipped key
  table is not mistaken for a complete one.
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

- **`browse` takes a directory** (ADR-0009 [D]): `bomdive browse examples/` loads the CycloneDX
  documents directly inside it, one level deep and sorted by name, and can be mixed with named
  files. Membership is decided by content, not by extension. A file that is not CycloneDX is
  skipped, counted in the files column's title, and named with its reason by `?`. A CycloneDX file
  that fails to load is a row marked *not loaded* that cannot be opened. On it, the header, status
  bar, detail and `?` say why, instead of describing another document. A file named on its own
  still fails hard, even when a named directory also holds it. The title says where the set came
  from, for example `66 documents from examples/`. A directory is a set nobody chose file by file:
  in that one, 64 of 80 BOM-Links read *linked, ambiguous*, which `?` explains.
- **`browse` takes several documents, and follows BOM-Links between them** (ADR-0009 [B]):
  `bomdive browse app.vex.json app.cdx.json`. With two or more named, the leftmost column lists the
  files, each with its kind, and the header, coverage and `?` describe the file under the cursor;
  with one, nothing changes. `v` shows every document's vulnerabilities, so an SBOM named first
  still answers which of its components a VEX says are exploitable. A BOM-Link resolves by serial
  number and version among the named documents: byte-identical copies count as one, and a serial
  and version claimed by two different documents is **linked, ambiguous**, with every candidate file
  named — serial numbers are not reliable identities, since 9 of the 11 serial-and-version pairs
  shared across the public corpora carry different content. Across documents a component is
  identified by its document and its `bom-ref`, because a `bom-ref` is unique only within its own.
  A reference to a document's own **subject** — its `metadata.component` — resolves whether or not
  that drives a dependency graph. Every CISA use-case link points at a product BOM's subject, and a
  standalone VEX names its own product the same way; with both of case 8's product BOMs named, all
  11 links first read *names nothing*, and a VEX's reference to its own product read as dangling.
- **The vulnerability records in a document can be browsed** — `v` in `browse` switches between
  the component axis and a vulnerability axis (ADR-0009), and switching back returns you to where
  you were. The first column groups by analysis state when the states divide the document, by
  severity when they do not, and not at all when neither divides it — measured on 169 real public
  documents, 128 split on neither, and a column holding one group is not an axis. From a
  vulnerability you reach what it affects, and from a component what affects it; `⇥` flips between
  the two. Every `affects` reference is shown in one of five states — resolved, names a package,
  linked but not loaded, linked at another version, names nothing — and the status bar reports how
  many resolved, as coverage does for the dependency graph. Values the schema forbids but real VEX
  carries (OpenVEX states and justifications, severities in capitals) are shown as written and
  said to be so; `MEDIUM` ranks with `medium`. A vulnerability id is not a key — 2,169 of 3,294 real
  records share theirs with another record in the same document — so a row whose id repeats in its
  column also names what the record affects, and a row too long for its column keeps the id whole
  and trims the rest. A component's detail now summarises the
  vulnerabilities that affect it, on either axis. `ls` and `tree` are unchanged.
- `testdata/validate-schema.py` can name a fixture that must be **invalid**, with the reason it
  exists. The gate fails if such a fixture validates — its reason has gone — or no longer exists, so
  the list cannot become a place to hide a broken file. `vex-out-of-schema` is the first entry.
- **A VEX corpus in `scripts/check-corpora.sh`** (`--corpus vex`): 207 documents from seven
  publishers — Liquibase's VEX feed, Apache Camel's four project VEX files (Dependency-Track 4.10.1),
  EvergreenImageRegistry, Softing Industrial (Dependency-Track 4.13 and 4.14), the Moderne Backpatch
  feed, and two single documents. **All 207 load.** It is cached in `.corpora-cache-vex`, apart
  from the other corpora, so the POC-9 and POC-10 re-runs keep reproducing what they recorded.
  Softing and Moderne state no reuse terms, so they are fetched for local testing only — never
  committed or redistributed — by the maintainer's decision.
- **Two more CycloneDX documents that are not bills of materials are named**: an **attestation**
  (`declarations` only — CycloneDX Attestations) and a **definitions** document (`definitions`
  only — it defines standards others attest against). Both were labelled `SBOM` and explained as
  "carries metadata and nothing else", which was false. Their evidence is weaker than VEX's, and is
  stated: attestation rests on **one** sample, the specification's own `valid-attestation-1.6.json`
  — which bomdive cannot load, because `cyclonedx-go` fails to decode it (CycloneDX/cyclonedx-go#275)
  — and definitions on the **schema alone**, with no sample in any corpus. Fixtures:
  `testdata/attestation-only.cdx.json`, `testdata/definitions-only.cdx.json`. When a document
  carries several payloads and no components, the best-evidenced reading wins: VEX, then
  attestation, then definitions.
- **A row you can descend into is marked with `→`** at the right of its column, the way macOS
  Finder chevrons a folder. Without it a leaf and a component with fifty dependencies look
  identical until you press `→` and nothing happens. It follows the view's direction, so in
  `dependents` mode it marks what something is *pulled in by*, and it is driven by the same
  predicate `Right` guards on — an indicator cannot promise a descent that `Right` then refuses.
  A dangling `dependsOn` resolves to no component and is correctly **not** marked.
- **`?` in `browse` explains the status bar**, for the document in front of you: what the coverage
  numbers mean, why the leftmost column shows what it shows, and any dangling `dependsOn` targets.
  The status bar has room for one word, and a word a reader cannot expand is jargon — `partial` in
  particular meant nothing without it.
- **`H` lists the keys.** Kept separate from `?` on purpose: *why does it say that* and *what can I
  press* are different questions, and merging them buries the contextual half under a key table the
  reader has usually already learned. **`H` and not `h`** — `h` is Left, and this tool's premise is
  that `ls`/`tree` muscle memory carries over; `J`/`K` are already the shifted forms of `j`/`k`.
- **XML input**, spec **1.0–1.7** — two versions below anything CycloneDX JSON can express. Format is
  detected from **content**, not the file extension, and a UTF-8 byte order mark is tolerated.
- **BDD with Gherkin** (godog): 14 scenarios across `user-ls`, `user-tree` and `api-json`, driving
  the CLI in-process. `Strict: true`, so an undefined step fails rather than passing quietly.
- Audience registry **aligned with `ansible-bom`** — A1–A7 IDs unchanged, so an ID names the same
  person in both repositories. A4 moves from Integration to Primary, because a viewer's security
  engineer reads BOMs rather than consuming output.
- `scripts/check-audience-tags.py` enforcing traceability in both directions.
- `scripts/check-corpora.sh` — runs bomdive over 137 real BOMs from `sbom-examples` (CC0-1.0),
  cdxgen and syft. Brings the first **public** OBOM, HBOM, CBOM, SaaSBOM and MBOM documents into the
  test set; POC-7's samples were all private.
- `scripts/check-conformance.sh` — runs bomdive against the CycloneDX specification's own
  conformance corpus (359 valid / 141 invalid documents). Found an upstream `cyclonedx-go` defect on
  its first run: a document the spec calls valid fails to decode.
- `scripts/check-coverage.sh` — a ≥80% floor for every package under `internal/`, wired into the
  pre-push gate and reconciled against `go list`, so a package with no tests at all cannot go
  unreported. ADR-0002 amended to match what is enforced.
- `--cpuprofile` and `--memprofile` on every command, plus benchmarks that accept the same. Stdlib
  `runtime/pprof`, no new dependency, no effect on output — and profiles are written even when the
  command fails, which is when they are most wanted.

- **`bomdive browse`** — the Finder-style column view. Each column lists the children of the
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

### Performance

- `Walk` was **O(n²)** on a deep dependency chain: path membership was a linear scan of a slice.
  With a set it is **41× faster** — 115ms → 2.8ms over a 10k-component BOM with a 10k-deep spine.
  Reproducible benchmarks (`BenchmarkLoad10k`, `BenchmarkWalk10k`) and a budget test now guard it.

### Notes

- Renamed from `lsbom` before the first commit: macOS ships `lsbom(8)` for Installer `.bom`
  files — a collision in the same semantic space. `lsxbom` was verified free in `PATH`, MacPorts,
  `man`, and GitHub repository search on 2026-09-10. (`lsxbom` was itself renamed to `bomdive` on
  2026-09-17 — see ADR-0010; this entry records what was checked at the time.)

[Unreleased]: https://github.com/jrjsmrtn/bomdive/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jrjsmrtn/bomdive/releases/tag/v0.1.0
