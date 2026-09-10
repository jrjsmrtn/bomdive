# 5. Graph and SQL engines stay behind opt-in build tags

Date: 2026-09-10

## Status

Accepted

## Context

Cross-BOM questions — *which hosts run something that transitively depends on X* — are real and
unanswerable from a single BOM. Embedding an engine (LadybugDB for Cypher, DuckDB for SQL and
zero-ingest JSON) would answer them, at a cost that had to be measured rather than assumed. See
`docs/inception/evidence/poc5-cgo-linking-2026-09-10.md`.

**What was measured:**

- A **self-contained binary survives cgo.** Statically linking `liblbug.a` produced a macOS
  binary with **zero non-system dylibs**. The "cgo means you cannot ship one file" objection is
  false, and an earlier draft of the analysis was corrected on this point.
- **Cross-compilation is the real cost** — and `zig cc` largely removes it: a cgo build
  cross-compiled from darwin/arm64 to a *statically linked* linux/arm64 ELF, where the system
  clang fails outright.
- **DuckDB is much cheaper to embed than Ladybug** — `go get` plus `go build` in about three
  seconds, no linker gymnastics, because it bundles prebuilt static libraries. It also reads
  CycloneDX **in-process** via `read_json_auto`.
- **With `CGO_ENABLED=0` the Ladybug binding does not degrade — it fails to compile.**

Meanwhile the corpus questions are already answered today by the `duckdb` and `lbug` CLIs
against the BOM files as they sit on disk, with nothing to keep in sync.

## Decision

**The default binary links no engine.** `lsxbom` builds with `CGO_ENABLED=0`, stays pure Go,
and cross-compiles for free. Engines are **opt-in build tags**:

| Build | Links |
|---|---|
| default | nothing — pure Go |
| `-tags duckdb` | DuckDB, statically |
| `-tags ladybug` | LadybugDB, statically |

**The guard must be `//go:build`, not `CGO_ENABLED`**, because the binding fails to compile
rather than degrading. The pattern is verified: a stub file under `//go:build !ladybug` and the
real backend under `//go:build ladybug`, with the default build still cross-compiling to all
four targets with the cgo file present in the tree.

**Neither engine ships in v0.1.** The scaffolding is proven so it is available when wanted; the
feature is deferred because the CLIs already do the job.

**If exactly one is ever embedded, it is DuckDB** — it earns its place on *every* BOM type,
including the OBOMs that have no graph at all, where Ladybug's Cypher would earn nothing.

⚠ **If DuckDB is embedded, `cyclonedx-go` remains the only BOM parser.** Letting DuckDB read
CycloneDX in the tagged build would mean the default and tagged binaries parse by different code
paths — the same two-backends-disagree hazard ADR-0004 records for cycle semantics. DuckDB does
corpus and aggregate work only.

⚠ **Ladybug's `INSTALL duckdb` is a runtime extension download.** Inside a shipped binary that
is a network fetch on first use — wrong for CI or an air-gapped host.

## Consequences

**Positive**: the artifact people download stays small, pure and portable; the heavyweight build
is a deliberate opt-in; no engine choice is locked in while the questions are still being learned.

**Negative**: two build configurations to test. A tagged build needs the target-platform library,
which `zig cc` does not conjure — Ladybug's Linux prebuilts are `compat`/`perf` rather than
musl, so the musl target may not link and `-gnu` is untried.

**Watched alternative**: PGlite + Apache AGE is the only architecture that could give Cypher
without cgo *and* without losing cross-compilation. Recorded with an explicit trigger and a
self-testing checker in `docs/roadmap/watched-pglite-age.md`; review by 2027-03-10.

## References

- `docs/inception/evidence/poc5-cgo-linking-2026-09-10.md` — the Ladybug linking recipe
- **`docs/inception/evidence/poc6-backend-survey-2026-09-10.md`** — the rest of this record's
  evidence: the DuckDB embed and its zero-ingest reading, the `zig cc` cross-compilation result,
  the build-tag verification, the Cypher-for-SQLite and DuckPGQ survey, and the Ladybug↔DuckDB
  bridge. Written after POC-5, which is why this record briefly cited numbers with nothing behind
  them
- `docs/inception/evidence/poc6-src/` — every program run, preserved
- `docs/roadmap/watched-pglite-age.md` — the cgo-free alternative, watched
