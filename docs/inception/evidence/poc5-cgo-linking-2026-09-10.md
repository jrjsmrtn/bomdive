# POC-5 — what relaxing the "no cgo" constraint actually costs

Run 2026-09-10 on darwin/arm64, go1.27.1, MacPorts `ladybug @0.19.1_0`, `go-ladybug v0.17.0`.
Program under test: `poc5-ladybug-linking.go.txt` — a BOM-shaped graph (diamond + cycle + host→component
edges) queried with Cypher.

## Result: a self-contained macOS binary IS achievable

| Build | Linkage | Size | Correct? |
|---|---|---|---|
| `system_ladybug`, default | `/opt/local/lib/liblbug.0.dylib` + libc++ | 6.9 MB | yes |
| `-L` isolating `liblbug.a` | MacPorts OpenSSL still dynamic | 25 MB | yes |
| **+ static OpenSSL** | **`/usr/lib` only — 0 non-system dylibs** | **30 MB** | **yes** |
| pure-Go trivial baseline (reference) | `/usr/lib` only | 3.6 MB | n/a |

The recipe: point `-L` at a directory containing **only** the `.a` files, because the binding
hardcodes `-llbug` and the macOS linker prefers a `.dylib` when both are on the path.

```
CGO_CFLAGS="-I/opt/local/include" \
CGO_LDFLAGS="-L<dir-with-only-.a-files> -lssl -lcrypto -lc++ -lz" \
go build -tags system_ladybug
```

⚠ The **default** (bundled, no build tag) mode sets `-Wl,-rpath,${SRCDIR}/lib` — an rpath into the
**Go module cache**. That binary works on the build machine and breaks anywhere else. Do not ship it.

## What it does cost

| | pure Go | cgo + LadybugDB |
|---|---|---|
| Self-contained macOS binary | yes | **yes** (proven above) |
| **Cross-compile from macOS** | **linux/arm64, linux/amd64, windows/amd64 all OK** | **fails** — needs a C toolchain per target |
| `CGO_ENABLED=0` fallback | n/a | **does not compile at all** — the binding's symbols vanish |
| Binary size | 3.6 MB baseline | 30 MB |
| Build inputs | `go` | `liblbug.a` (87 MB) + OpenSSL `.a` + a C++ toolchain |

Cross-compilation is the real cost, and it is not avoidable by static linking. Shipping Linux and
Windows builds becomes a per-platform CI job rather than a free `GOOS=` flag.

## Correctness of the graph engine on real BOM shapes

All four queries returned correct answers, including the two that motivate a graph store at all:

- **transitive pull-in** (`DEPENDS_ON*1..10`) over a diamond — correct, deduplicated.
- **cross-BOM join** host → component → transitive dependency — correctly returned only the host
  that reaches the target, *excluding* the host trapped in a cycle that never reaches it.
- **cycle** `x↔y` — traversed and terminated. Did not hang, which is the failure mode a naive
  hand-rolled walk has on the real BOMs POC-1 measured.
- `SHORTEST` path length — correct.

## Maintenance note

`go-ladybug v0.17.0` was built against **liblbug 0.19.1** and worked. That is ABI luck, not a
guarantee: the binding lags the library, and this project would be pinning both while also
maintaining the MacPorts port that supplies the `.a`.

## Consequence for the design

The "no cgo" constraint as originally written was **too strong** — it claimed a self-contained
binary was unattainable, and that is measurably false on macOS. The defensible version of the
constraint is about **cross-compilation and graceful degradation**, which build tags resolve:

- default build — pure Go, cross-compiles free, single-BOM `ls`/`tree`/`query`.
- `-tags ladybug` — adds corpus indexing and Cypher queries, built per-platform in CI.

Because `CGO_ENABLED=0` does not merely disable the binding but fails to compile it, the
ladybug-backed code **must** sit behind a `//go:build ladybug` guard rather than relying on
`CGO_ENABLED`.
