# Watched: PGlite + Apache AGE as a cgo-free graph backend

**Status:** watched, not planned. **Recorded 2026-09-10. Review by 2027-03-10.**

## Why it is worth watching

It is the only architecture surveyed that could give `bomdive` **openCypher without cgo and
without losing cross-compilation**. PGlite is Postgres compiled to WebAssembly; a WASM module is
architecture-neutral, and `wazero` is a zero-dependency **pure-Go** WASM runtime. If the two meet,
the result is Postgres + Apache AGE + pgvector inside a `CGO_ENABLED=0` binary that still
cross-compiles for free.

Every other option trades one of those away — see `../inception/evidence/poc5-cgo-linking-2026-09-10.md`
and the consolidated comparison in the SPARK analysis.

## What is true as of 2026-09-10 (dated facts, not states)

| Fact | Observed |
|---|---|
| `@electric-sql/pglite-age` published | 0.0.9; created 2026-06-02, updated 2026-08-26 |
| `@electric-sql/pglite` core | 0.5.8, updated 2026-08-26 (same day — released in lockstep) |
| AGE upstream tracked | Apache AGE PG17 / v1.7.0-rc0, with 32-bit WASM fixes |
| `electric-sql/pglite#89` "bindings for other runtimes" | **open**, created 2024-05-15, last updated 2026-03-27, 15 comments, labels `wasi/fsm` + `libpglite` |
| `roastedroot/pglite4j` | Java, 14★, pushed 2026-07-02 — **a working PGlite WASI build** driving a pure-Java JDBC driver |
| `elliots/go-pglite` | Go, 12★, pushed 2026-03-18, author describes it as "barely tested" |
| `tw1nk/go-pglite` | Go, 4★, pushed 2025-02-27 — dead; README: "No data is returned from the postgres wasi binary" |

**The hard part is done and it is not the part we need.** `pglite4j` demonstrates PGlite running
under a plain WASM runtime outside JavaScript. The missing piece is a maintained Go wrapper over
the same WASI artifact, which is a packaging problem rather than a research one.

## Trigger — adopt only when ALL of these hold

1. A Go PGlite binding exists that **returns query results** and is maintained (a commit inside the
   last 6 months), **or** upstream ships `libpglite` / an official WASI artifact.
2. `pglite-age` loads under that binding — the extension is distributed as a WASM bundle, and
   nothing guarantees a non-JS loader can install it.
3. A `CGO_ENABLED=0` build runs a variable-length Cypher query against a real BOM graph and agrees
   with the LadybugDB answer from POC-5 — **including the cycle case**, where edge-uniqueness and
   node-uniqueness disagree.

⚠ Condition 3 is not ceremony. Two backends already disagreed on `reachable from x` under the
`x↔y` cycle; a third must be pinned against the same fixture before it is trusted.

## Known cost even if the trigger fires

Apache AGE does not expose Cypher as a first-class language. Every query is wrapped, with an
explicit `agtype` column list required even when nothing is returned:

```sql
SELECT * FROM cypher('g', $$ MATCH (a)-[:DEPENDS_ON*1..12]->(b) RETURN a.name $$) AS (name agtype);
```

Against `lbug`'s bare `MATCH … RETURN count(DISTINCT a.name)`, that is a real ergonomic tax on
every query the tool would generate.

## What can be done NOW, without the trigger

PGlite + AGE **works today from Node**, and `ragu-pglite` / `pglite-libversion` mean that ground is
already familiar — including how to build a "complex extension" WASM bundle, should an AGE bundle
ever need rebuilding.

So the query design can be de-risked independently of the runtime decision: prototype the corpus
questions in PGlite+AGE under Node, learn which ones matter, and carry that knowledge to whichever
backend wins. **That work is not wasted if the trigger never fires** — the same questions are the
ones `lbug` and `duckdb` answer today.

## Related

- `../inception/spark-analysis.md` — the backend comparison and the DAG-rendering decision
- `../inception/evidence/poc5-cgo-linking-2026-09-10.md` — what cgo actually costs, measured
- `../../scripts/check-pglite-trigger.py` — the mechanical check for condition 1
