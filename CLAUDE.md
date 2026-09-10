# lsxbom

A Go CLI that reads CycloneDX xBOM JSON — SBOM, OBOM, HBOM — and navigates it with the muscle
memory of `ls(1)` and `tree(1)`.

## Project Context

- **Category**: Development
- **Type**: CLI tool
- **Stack**: Go 1.25+ (`cyclonedx-go` for the model, `cobra` for the CLI, `gojq` as query escape hatch)
- **License**: Not set (Private profile, unpublished)
- **Tier**: t1
- **Distribution profile**: Private (ships-artifacts: no)

## Project Tier

Current tier: **t1**, bootstrapped 2026-09-10 after a SPARK analysis
([docs/inception/spark-analysis.md](docs/inception/spark-analysis.md)).

Tier-specific artifacts:

- `CLAUDE.md` (this file), `.gitignore`, `README.md`, `CHANGELOG.md`
- `docs/adr/` — foundation ADRs plus the decisions this project exists to make
- `docs/inception/` — the SPARK analysis and its **evidence**, which is re-runnable
- `docs/reference/audience-registry.md` — A1–A4 and the artifacts each is owed
- `docs/roadmap/roadmap.md` — the phases, and what is deliberately not a phase
- `docs/roadmap/watched-pglite-age.md` — a watched trigger, with a checker
- `docs/reference/quality-configuration.md` — the gates, and what is deliberately absent
- `scripts/` — `check-evidence-refs.py` (a gate), and `check-pglite-trigger.py` (**not** a
  hook; it needs a schedule — see the quality-configuration reference)
- `.editorconfig`, `.lefthook.yml`, `.lefthook/pre-push/gates.sh`
- git on `main` + `develop`, conventional commits

**Deliberately not adopted at t1** — decisions, not oversights: Diátaxis `docs/` tree (nothing to
sort yet — though `docs/reference/` has grown organically), C4 model (a single binary reading a
file does not earn one), sprint cadence (one contributor, no deadline), `SECURITY.md` (Private
profile), `LICENSE`, `.github/` (no GitHub remote).

⚠ **A roadmap was adopted at t1 anyway**, which `bootstrap-project` lists as a t2 artifact. Taken
deliberately: real phases existed and needed recording, and adopting a whole tier to get one
document is cargo-culting it. The rest of t2 stays unadopted for the reasons above.

Promotion triggers being watched:

- **t1 → t2** if a second contributor appears, **or user-facing documentation needs sorting** —
  meaning tutorials and how-tos exist and are hard to find, not merely that the doc count grew.

  ⚠ **The previous wording was "docs outgrow README + ADRs", and it was too vague to act on.** By
  2026-09-10 there were 14 documents, so it had arguably fired while meaning nothing: the growth was
  into evidence and reference, which already have homes. A trigger that fires on volume rather than
  on a felt problem is not a trigger. Diátaxis sorts *user-facing* docs, and there are none yet
  because there is no code.
- **Private → Public** is a separate axis, running through the `public-release` gate and needing
  its own ADR. Public is *defensible* — the tool is generic and carries no homelab shape — but it
  is unproven, so it stays private. Precedent: `okf-gate`.

## Status

Bootstrapped 2026-09-10. No code yet. The SPARK analysis is complete and its measurements are
reproducible; see the evidence directory rather than trusting any number quoted in prose.

## What this is, and what it is not

**Is**: a terminal-native navigator for a CycloneDX BOM you already have.

**Is not**: a BOM *generator* (that is syft, cdxgen, trivy), a BOM *editor*, a vulnerability
scanner (grype, osv-scanner, Dependency-Track), a signer (`cosign`, `cyclonedx-cli`), or a format
converter (`cdx-convert`). **Nor is it a query tool** — `CycloneDX/sbom-utility` already does
SQL-ish query over BOMs, in Go, under the CycloneDX org. If `query` ever becomes the headline
here, the honest advice to a user is "use sbom-utility", and this project should stop.

## The three findings that shape the design

All measured 2026-09-10 against real BOMs; reproduce with `docs/inception/evidence/*.py`.

1. **The dependency graph is a DAG with cycles, and often barely present.** A syft SBOM entered 46%
   of its components into the graph on one project and **5%** on another. Cycles are ordinary — 3 in
   a 153-component BOM, 31 in a 9,314-component one.
2. **The declared root is frequently not in the graph.** syft's `metadata.component.bom-ref` is a
   content hash; the graph is keyed by purl. Walking from the declared root yields **zero**
   components. Roots must be derived from in-degree-0 nodes.
3. **An OBOM has no dependency graph at all** — `dependencies: []`, present and empty, in **14 of
   14** real files. `tree` is meaningless there; the navigation axis is the
   `cdx:osquery:category` property, which 3,109 of 3,110 components carry. An HBOM is depth-1.

**Therefore `ls` and `tree` are not symmetric.** `ls` is the universal command; `tree` is the one
that pays off where a graph exists — container-image SBOMs and directory scans.

**And `tree` is not the design centre** ([ADR-0006](docs/adr/0006-column-view-as-a-first-class-renderer.md)).
Its difficulty comes from rendering the whole graph at once; a Miller-column view (Finder-style)
shows one path at a time, which avoids the shared-subtree and cycle-rendering decisions and *also*
works on the graphless BOMs. **The traversal model must expand lazily, one level from an arbitrary
node** — a recursive full-graph walk would have to be rewritten.

## Correctness obligations

These are guarantees, not niceties, and they are why this tool beats `jq`:

- **Never render a partial graph as complete.** Coverage (reached / total) is reported by `tree`,
  in both the human and the `--output json` surface.
- **A BOM with no `dependencies` is a normal document, not a broken one.** Both surfaces must
  distinguish "this BOM declares no graph" from "this tool found no graph".
- **A synthetic root is labelled as synthetic.**
- **Cycle traversal semantics are a stated decision, not an accident.** Cypher-style
  *relationship*-uniqueness and *node*-uniqueness give different answers for the same real BOM;
  both were reproduced. Whichever this tool picks is documented and fixture-tested.

## Identity

`bom-ref` is the only reliable key. In a real OBOM, `bom-ref` was unique across all 3,110
components while `name` was not (2,708), and only 60–72% carried a `purl`. Anything keyed on name
or purl will collide or miss a third of the inventory.

## Development Practices

Follows the [AI-Assisted Project Orchestration patterns](https://github.com/jrjsmrtn/ai-assisted-project-orchestration).

- **Versioning**: semantic, patch-level during development (0.1.x)
- **Commits**: Conventional Commits. A commit message MUST NOT claim more than the commit contains
  — re-read it against `git diff --cached`, not against intent
- **Git workflow**: gitflow (`main`/`develop`), matching the `ansible-bom` sibling
- **Testing**: table-driven Go tests over committed BOM fixtures, including the **cyclic** ones

## Quick Commands

```bash
go build ./...                 # pure Go, CGO_ENABLED=0, cross-compiles free
go test ./...
python3 scripts/check-pglite-trigger.py --self-test   # prove the watcher can fail
python3 docs/inception/evidence/bom-graph-shape.py <bom.json>   # re-measure graph shape

go test ./internal/bom/ -bench . -benchtime 200x -run XXX     # benchmarks
lsxbom tree --cpuprofile cpu.prof <bom.json>                  # profile a real BOM
go tool pprof -top -nodecount=15 cpu.prof
```

⚠ **Profile before optimising.** The one hotspot found so far was not the one guessed at, and the
guess made things slower — see `docs/reference/quality-configuration.md`.

## Engines are opt-in build tags; the default binary links none

Proven 2026-09-10 — see [ADR-0005](docs/adr/0005-engines-behind-build-tags.md),
`docs/inception/evidence/poc5-cgo-linking-2026-09-10.md` (Ladybug linking) and
`docs/inception/evidence/poc6-backend-survey-2026-09-10.md` (DuckDB, zig, build tags, the
backend survey):

| Build | Size | Non-system dylibs |
|---|---|---|
| default, `CGO_ENABLED=0` | ~2 MB (stub) | 0 |
| `-tags ladybug` | 30 MB | 0 |
| `-tags duckdb` | 60 MB | 0 |

The cgo file must sit behind `//go:build ladybug` — with `CGO_ENABLED=0` the binding does not
degrade, it **fails to compile**. Cross-compiling a cgo build works via `zig cc`:

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC="zig cc -target aarch64-linux-musl" go build
```

⚠ Ladybug's Linux prebuilts are `compat`/`perf`, not musl — try `-gnu` there. Untested.

## AI Collaboration Notes

**What AI should know:**

- **Do not trust a number quoted in prose here.** Every quantitative claim has a re-runnable script
  in `docs/inception/evidence/`. Run it.
- The sibling `software-supply-chain-landscape` is the sourced reference corpus for the subject
  matter (`knowledge/bom-types/`). Prefer it over restating spec facts.
- Homelab BOMs are used as test corpora. **Never commit one, and never record a hostname, path or
  address** — measure shape, emit anonymised labels. `docs/inception/evidence/poc2-corpus-shape.py`
  takes the corpus directory as an argument for exactly this reason.
- `cdxi`'s `.osinfocategories` → `.<category>` flow is prior art for category navigation; it is
  TTY-only and JSONata-based, which is why the scriptable version is unoccupied.

**AI delegation:**

- **AI leads**: parsing glue, table-driven tests, fixture generation, docs
- **Human leads**: traversal semantics, what counts as a correctness guarantee, going public
- **Collaborative**: CLI surface design
