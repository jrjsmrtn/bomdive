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
| `poc6-src/cycle-semantics.go.txt` | Source for the above — both CTE variants side by side |
| `poc6-src/sqlite-cte.go.txt` | Pure-Go SQLite answering the BOM questions with recursive CTEs, `CGO_ENABLED=0` |
| `poc6-src/duckdb-embed.go.txt` | DuckDB embedded in Go, reading CycloneDX in-process via `read_json_auto` |
| `poc6-src/tagdemo-main.go.txt` | The build-tag pattern: one entry point, two backends |
| `poc6-src/tagdemo-backend_stub.go.txt` | `//go:build !ladybug` — the pure-Go default |
| `poc6-src/tagdemo-backend_ladybug.go.txt` | `//go:build ladybug` — the cgo opt-in |
| `poc6-src/zigcross.go.txt` | Minimal cgo program used to prove `zig cc` cross-compiles cgo to a static Linux ELF |

## Re-running

```bash
python3 bom-graph-shape.py <bom.json>              # POC-1, any BOM
python3 poc2-corpus-shape.py <out.json> <corpus>   # POC-2, any corpus directory
./poc6-cycle-semantics.sh                          # POC-6, self-asserting
```

The `.go.txt` sources are kept as text so they are not compiled as part of the module. The cgo ones
need their libraries; the build recipes are in the POC-5 and POC-6 documents.
