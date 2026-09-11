# Evidence

Every quantitative claim in this project's docs is backed by something here. `CLAUDE.md` tells a
reader not to trust a number quoted in prose and to run the script instead — this index is what
makes that instruction actionable.

**It is checked.** `scripts/check-evidence-refs.py` asserts, in both directions, that no document
names a file that is missing and no file here goes unnamed. It was written *because* that promise
had already been broken: ADR-0005 quoted DuckDB and zig numbers for three days while the only
evidence on disk covered LadybugDB linking.

## POC-1 — BOM graph shape

| File | What it is |
|---|---|
| `bom-graph-shape.py` | Measures one BOM: components, edges, shared nodes, cycles, depth, rooting, reachability |
| `measurements-2026-09-10.json` | Raw POC-1 output over syft and cdxgen SBOMs |

Established that the graph is a DAG with cycles, that the declared root is often absent from it,
and that coverage varies by an order of magnitude between generators.

## POC-2 — the OBOM/HBOM corpus

| File | What it is |
|---|---|
| `poc2-corpus-shape.py` | Runs POC-1's measurement across a corpus directory, emitting **anonymised** labels only |
| `poc2-bom-corpus-2026-09-10.json` | Raw output over 23 real BOMs |

Established that an OBOM carries **no dependency graph at all** (0 of 14) and an HBOM is depth-1 —
the finding that made `ls` the universal command and `tree` the conditional one.

⚠ The corpus directory is an **argument**, never hardcoded: real BOMs carry host identifiers, and
only shape is recorded.

## POC-5 — what cgo costs

| File | What it is |
|---|---|
| `poc5-cgo-linking-2026-09-10.md` | The LadybugDB static-linking recipe and measurements |
| `poc5-ladybug-linking.go.txt` | The BOM-shaped Cypher program used to prove correctness while linking |

Established that a self-contained binary **does** survive cgo, correcting an earlier claim.

## POC-6 — backend survey

| File | What it is |
|---|---|
| `poc6-backend-survey-2026-09-10.md` | The full survey: DuckDB embed and zero-ingest, `zig cc` cross-compilation, build-tag verification, Cypher-for-SQLite and DuckPGQ, the Ladybug↔DuckDB bridge |
| **`poc6-cycle-semantics.sh`** | **Runnable.** Proves edge-uniqueness and node-uniqueness give different answers over `x → y → x`, and asserts both. ADR-0004's traversal decision rests on it |
| **`poc6-ladybug-duckdb-bridge.sh`** | **Runnable.** The combo: DuckDB reads CycloneDX with zero ingest, LadybugDB takes it by Parquet *and* by `ATTACH (dbtype duckdb)`, and variable-length Cypher traverses the result. Asserts both handoffs agree with DuckDB |
| `poc6-duckdb-corpus.sql` | The zero-ingest DuckDB queries, runnable against a committed fixture |
| `poc6-src/cycle-semantics.go.txt` | Source for the above — both CTE variants side by side |
| `poc6-src/sqlite-cte.go.txt` | Pure-Go SQLite answering the BOM questions with recursive CTEs, `CGO_ENABLED=0` |
| `poc6-src/duckdb-embed.go.txt` | DuckDB embedded in Go, reading CycloneDX in-process via `read_json_auto` |
| `poc6-src/tagdemo-main.go.txt` | The build-tag pattern: one entry point, two backends |
| `poc6-src/tagdemo-backend_stub.go.txt` | `//go:build !ladybug` — the pure-Go default |
| `poc6-src/tagdemo-backend_ladybug.go.txt` | `//go:build ladybug` — the cgo opt-in |
| `poc6-src/zigcross.go.txt` | Minimal cgo program used to prove `zig cc` cross-compiles cgo to a static Linux ELF |

## POC-7 — BOM identity and OBOM navigation

| File | What it is |
|---|---|
| `poc7-bom-identity-2026-09-10.md` | The discrimination rule (`lifecycles` + root component type), the custom-lifecycle trap, the 40-category OBOM vocabulary and its platform split, and the merged host view as an **unmeasured** exception |
| `poc7-bom-type-discriminator.py` | Runnable classifier. Emits shape only, so its output is safe over a private corpus |

## POC-8 — the merged host view

| File | What it is |
|---|---|
| `poc8-merged-host-view-2026-09-10.md` | The one shape ADR-0004 carried as an unmeasured exception. It *does* have a graph — 33 edges with real hardware→runtime links — reaching **0.6%** of the document. Found a root-resolution bug on the way |

Not re-runnable from a committed file by design: it needs `hbom --include-runtime` on a live host,
and the output is a personal inventory. The **shape it exposed** is a committed fixture instead —
`testdata/root-outside-components.cdx.json`.

## POC-9 — the declared root was invisible to every reverse edge

| File | What it is |
|---|---|
| `poc9-reverse-edges-2026-09-11.md` | The second half of POC-8's root-resolution bug, found by dogfooding: `Node()` knew about a root outside `components` and `resolve()` did not, so **1,658** components across **58 of 137** public corpus documents reported no dependent while `dependencies` declared one |
| `poc9-root-outside-components.py` | Runnable over any corpus directory. Counts documents, not identifiers, so it is safe over a private corpus too |

## POC-10 — documents with no inventory

| File | What it is |
|---|---|
| `poc10-empty-documents-2026-09-11.md` | **40 of 137** public corpus documents carry no `components` — 25 standalone VEX, 14 metadata-only, 1 services-only. All were identified as "SBOM" and drawn as an empty pane. The VEX discrimination rule, and why it keys on the absence of components rather than the presence of vulnerabilities |
| `poc10-documents-without-components.py` | Runnable over any corpus directory. Counts documents, not identifiers |

## POC-11 — can a VEX be navigated?

| File | What it is |
|---|---|
| `poc11-vex-navigability-2026-09-11.md` | Two worlds that do not overlap. The **25 public use cases** are standalone VEX with analysis states, mostly unrated, and 79 of 80 BOM-Links resolving across documents. **Nine Dependency-Track 5.1.0 exports** are embedded VEX — only the affected components, up to 1,505 vulnerabilities and 1,134 on one component, every one rated and none analysed, no BOM-Links. Recorded as shape only; the exports are private. The evidence behind [ADR-0009](../../adr/0009-browse-a-standalone-vex.md) and its revised section 1 |
| `poc11-vex-navigability.py` | Measures standalone and embedded VEX separately, over any corpus directory. Counts only, never identifiers |

## Re-running

```bash
python3 bom-graph-shape.py <bom.json>              # POC-1, any BOM
python3 poc2-corpus-shape.py <out.json> <corpus>   # POC-2, any corpus directory
./poc6-cycle-semantics.sh                          # POC-6, self-asserting
./poc6-ladybug-duckdb-bridge.sh [bom.json]         # POC-6, the combo; needs duckdb + lbug
duckdb -c ".read poc6-duckdb-corpus.sql"           # POC-6, zero-ingest queries
python3 poc7-bom-type-discriminator.py <bom.json>  # POC-7, classify a BOM
python3 poc7-bom-type-discriminator.py --corpus <dir>   # POC-7, anonymised summary
python3 poc9-root-outside-components.py <corpus>   # POC-9, how far the root bug reached
python3 poc10-documents-without-components.py <corpus>  # POC-10, BOMs with no inventory
python3 poc11-vex-navigability.py <corpus>        # POC-11, VEX shape and link resolution
```

The `.go.txt` sources are kept as text so they are not compiled as part of the module. The cgo ones
need their libraries; the build recipes are in the POC-5 and POC-6 documents.
