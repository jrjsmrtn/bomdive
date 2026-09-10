# POC-6 — backend survey: what can answer cross-BOM questions, at what cost

Run 2026-09-10 on darwin/arm64. Toolchain as measured: go1.27.1, MacPorts `ladybug @0.19.1_0`,
`go-ladybug v0.17.0`, `duckdb 1.5.5`, `zig 0.16.0`, `modernc.org/sqlite v1.58.0`.

**Extends [POC-5](poc5-cgo-linking-2026-09-10.md)**, which measured only the LadybugDB linking
question. Everything below was measured *after* POC-5 was written, which is why ADR-0005 and
`CLAUDE.md` briefly quoted numbers with no evidence behind them. This file closes that gap.

## The question

A single BOM cannot answer *which hosts run something that transitively depends on X*. Embedding an
engine would. What does each candidate actually cost, and is embedding necessary at all?

## Verdicts

| Option | Cypher | cgo | Cross-compiles | Reads CDX directly | Usable from Go today |
|---|---|---|---|---|---|
| `modernc.org/sqlite` + recursive CTEs | no (SQL) | **no** | **free** | no | **yes** |
| **DuckDB** (`duckdb/duckdb-go`) | no (SQL) | yes | via `zig cc` | **yes, in-process** | **yes** |
| SQLite + GraphQLite | yes | yes | via `zig cc` | no | **no Go binding** |
| **LadybugDB** (`go-ladybug`) | **yes** | yes | via `zig cc` | no | **yes** |
| PGlite + Apache AGE | yes (wrapped) | no, in principle | free, in principle | no | **no** — see the roadmap watch |

## Measured: binary sizes, all self-contained

Every binary below reports **zero non-system dylibs** under `otool -L`.

| Build | Size |
|---|---|
| pure Go, `CGO_ENABLED=0`, trivial | 2.3 MB |
| `-tags ladybug`, static `liblbug.a` + static OpenSSL | **30 MB** |
| `-tags duckdb` | **60 MB** |
| pure Go + `modernc.org/sqlite` | 9.5 MB |
| cgo cross-compiled to linux/arm64 via `zig cc` | 4.4 MB |

## Measured: cross-compilation, and the zig fix

The cost POC-5 identified as decisive turns out to be largely removable.

```bash
# fails: macOS clang cannot target linux
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build
#   runtime/cgo: gcc_clearenv.c:16: implicit declaration of function 'clearenv'

# works:
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
  CC="zig cc -target aarch64-linux-musl" CXX="zig c++ -target aarch64-linux-musl" go build
#   -> ELF 64-bit LSB executable, ARM aarch64, statically linked
```

⚠ **`zig cc` compiles *your* C; it does not conjure a third-party library for the target.** Ladybug
ships static prebuilts (`liblbug-static-{osx,linux,windows}-{x86_64,arm64/aarch64}`, with an
`LBUG_ARCH` override), but its Linux variants are **`compat`/`perf`, not musl** — so the musl
target may not link and `-gnu` is untried. `duckdb-go` bundles static libs for linux/amd64,
darwin/amd64 and darwin/arm64.

## Measured: the build-tag pattern

`CGO_ENABLED=0` does **not** degrade the Ladybug binding — it fails to compile:

```
./driver.go:61:18: undefined: DefaultSystemConfig
```

So the guard must be `//go:build`, not `CGO_ENABLED`. Verified with a stub under
`//go:build !ladybug` and the backend under `//go:build ladybug`: the default build still
cross-compiles to **linux/arm64, linux/amd64, windows/amd64, darwin/arm64** with the cgo file
present in the tree. Sources: `poc6-src/tagdemo-*.go.txt`.

## Measured: DuckDB is much cheaper to embed than Ladybug

`go get` + `go build` in **3.0 seconds**, with no `CGO_LDFLAGS` and none of POC-5's static-archive
isolation, because it bundles prebuilt libraries. In-process it read a real CycloneDX file:

```
embedded duckdb: v1.5.5
read CDX in-process: components=153 dependencies=70
```

Those match the counts POC-1 measured with Python and cross-checked with `jq` — four independent
tools, same answer. Source: `poc6-src/duckdb-embed.go.txt`.

## Measured: DuckDB needs no ingest at all

The finding that most affects the design. `read_json_auto` reads CycloneDX directly, and a glob
reads a whole corpus in one statement — no schema, no import, no database file:

```sql
SELECT ... FROM read_json_auto('*/*.json', filename=true, union_by_name=true) GROUP BY kind;
```

Against a 24-file private corpus (host identifiers withheld; only shape recorded), this reproduced
POC-2's entire finding as one query — 14 OBOMs with **0** dependency entries across 47,445
components, 4 HBOMs, 6 image/dir SBOMs. Transitive closure straight off the JSON gave 70 roots and
2,712 closure pairs.

⚠ That closure terminated only because of a `depth < 20` cap. With `depth` in the recursion tuple,
`UNION` does not dedupe cycles away. **Cycle handling is a design decision in every backend**, which
is the same lesson as the semantics split below.

## Measured: Cypher over DuckDB data, without a DuckDB Cypher extension

There is **no Cypher extension for DuckDB** — the local install lists 31 extensions and no graph
extension, and `duckpgq` (which is SQL/PGQ, not Cypher) 404s for this version and platform:

```
HTTP Error: Failed to download extension "duckpgq" at
http://community-extensions.duckdb.org/v1.5.5/osx_arm64/duckpgq.duckdb_extension.gz (HTTP 404)
```

But LadybugDB reads DuckDB and Parquet, and both paths work:

```cypher
INSTALL duckdb; LOAD duckdb;
ATTACH 'bom.duckdb' AS bomdb (dbtype duckdb);
LOAD FROM bomdb.nodes RETURN count(*);          -- 153
```

Parquet handoff gave the same 153 components / 245 edges. The real query, over a real BOM:

```cypher
MATCH (a:Component)-[:DEPENDS_ON*1..12]->(b:Component)
WHERE b.name = 'golang.org/x/sys' RETURN count(DISTINCT a.name);   -- 28
```
`Time: 2.38ms (compiling), 1.53ms (executing)`

⚠ `INSTALL duckdb` is a **runtime extension download** — inside a shipped binary that is a network
fetch on first use, which is wrong for CI or an air-gapped host.

## Measured: the pure-Go path needs no graph engine

`modernc.org/sqlite` is a CGo-free port of SQLite. Built with `CGO_ENABLED=0`, recursive CTEs
answered the same questions correctly. Source: `poc6-src/sqlite-cte.go.txt`.

```
what transitively pulls in log4j?      a app b shared
which hosts reach log4j?               host1
shortest path app -> log4j             3
```

⚠ **modernc cannot load C extensions at all**, so neither SQLite Cypher extension is reachable from
it; it offers pure-Go virtual tables (`modernc.org/sqlite/vtab`) instead.

## The cycle-semantics split — ADR-0004 rests on this

One query disagreed with LadybugDB, and it is not a bug in either.

| | reachable from `x`, over `x -> y -> x` |
|---|---|
| LadybugDB / Cypher | `x y` |
| SQLite CTE, node-uniqueness | `y` |

Isolated by writing both variants; **edge-uniqueness reproduces Cypher exactly**:

```
edge-uniqueness from x:   x y     <- matches Cypher (relationship isomorphism)
node-uniqueness from x:   y
```

Cypher's variable-length patterns forbid repeating an *edge* but allow repeating a *node*, so
`x -> y -> x` is a legal two-hop path and `x` is genuinely reachable from itself.

**Re-run it: [`poc6-cycle-semantics.sh`](poc6-cycle-semantics.sh)** — self-contained, pure Go, and
it asserts both answers rather than printing them.

## Cypher-for-SQLite, surveyed

| | GraphQLite | agentflare-ai/sqlite-graph |
|---|---|---|
| Maturity | **97.7% openCypher TCK** over 3,876 scenarios; MIT | **v0.1.0-alpha.0**, "production use is not recommended" |
| Bindings | Python, Rust, raw SQL — **no Go** | — |
| Form | C loadable extension | C loadable extension |

Both are C extensions, so both mean cgo *and* a missing Go binding — strictly worse than Ladybug,
which has an official one that POC-5 proved links statically.

## What this corrected

- **POC-5 overstated the cgo cost.** A self-contained binary survives cgo, and `zig cc` largely
  removes the cross-compilation penalty too. The surviving cost is the per-target third-party
  library.
- **DuckDB, not Ladybug, is the better single engine** if exactly one is ever embedded: it earns its
  place on *every* BOM type, including the OBOMs with no graph at all, and it is far cheaper to build.
- **Neither needs embedding now.** The corpus questions are answered today by the `duckdb` and
  `lbug` CLIs against the files on disk, with nothing to keep in sync — which is what ADR-0005
  records.

## Sources preserved

`poc6-src/` holds every program run here as `.go.txt`, so none of it has to be reconstructed from
prose. The cgo ones need their libraries; the recipes are above.
