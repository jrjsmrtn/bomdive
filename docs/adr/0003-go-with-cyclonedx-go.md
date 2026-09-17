# 3. Use Go with cyclonedx-go

Date: 2026-09-10

## Status

Accepted

## Context

bomdive must read CycloneDX JSON, walk a dependency DAG of ~10⁴ nodes, and render it at a terminal.
Requirements:

- **A single self-contained binary** — the tool is used over ssh and in pipelines.
- **Free cross-compilation** to linux/amd64, linux/arm64, darwin/arm64, windows/amd64.
- **A maintained CycloneDX library**, because the spec moves (1.0 through 1.7) and hand-rolling
  the model means re-implementing a first-party library badly.

Performance is *not* a differentiating requirement, and saying so avoids optimising a non-problem:
parsing a 10 MB BOM and walking 10⁴ nodes is far inside what every candidate language does easily.

## Decision

**Go**, with:

- [`CycloneDX/cyclonedx-go`](https://github.com/CycloneDX/cyclonedx-go) — Apache-2.0, spec
  1.0–1.7, JSON **and** XML. XML support is free but untested here, so it is unclaimed until it is.
- [`spf13/cobra`](https://github.com/spf13/cobra) for the CLI.
- [`itchyny/gojq`](https://github.com/itchyny/gojq) as a **read-only** query escape hatch —
  pure Go, no cgo.

⚠ **gojq does not preserve object key order** and has no `keys_unsorted`/`-S`. Harmless for
querying, disqualifying for emitting a BOM. This is an independent reason writing BOMs is out of
scope. Never emit a BOM through gojq.

### Alternatives considered

| Option | Verdict |
|---|---|
| **Go + cyclonedx-go** | **Selected** — official library covering 1.0–1.7, single binary, free cross-compilation, matches the `ansible-bom` sibling |
| Rust | **Rejected, closely.** The only other language with an official CycloneDX library *and* official LadybugDB, GraphQLite and DuckDB bindings. But its CycloneDX library is "a simple library to encode and decode BOMs" against cyclonedx-go's 1.0–1.7 in two encodings, and a second language in the workspace buys nothing |
| Go + hand-rolled structs | Rejected — re-implements a maintained library, and the spec moves |
| Zig | Rejected — best-in-class cross-compilation, no CycloneDX library. Retained as a **tool**, not a language: see ADR-0005 |
| Swift | Rejected — no CycloneDX library |
| .NET AOT / Java+GraalVM | Rejected — official CycloneDX libraries exist (`cyclonedx-cli` is .NET, `cyclonedx-core-java` is the reference implementation), but heavier toolchains for a small CLI |
| Python | Rejected — good library, wrong distribution story |

## Consequences

**Positive**: single static binary; cross-compiles to four targets for free (verified); one
toolchain shared with `ansible-bom`; the spec-tracking burden sits with the upstream library.

**Negative**: Go's error handling makes the traversal core verbose. `cyclonedx-go` v0.12.0+
requires Go 1.25+ (go1.27.1 present, so not currently binding).

**Risk**: the tool's value depends on `cyclonedx-go` keeping pace with the spec. Mitigation: it
is first-party to the CycloneDX org, which is the best available signal.

## References

- [CycloneDX/cyclonedx-go](https://github.com/CycloneDX/cyclonedx-go)
- [itchyny/gojq](https://github.com/itchyny/gojq)
- `docs/inception/spark-analysis.md` — the full option comparison
