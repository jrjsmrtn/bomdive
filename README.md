# lsxbom

Read a CycloneDX xBOM — SBOM, OBOM, HBOM — at the terminal, with the muscle memory of `ls(1)`
and `tree(1)`.

> **Status**: bootstrapped 2026-09-10, no code yet. See
> [`docs/inception/spark-analysis.md`](docs/inception/spark-analysis.md) for the design and the
> measurements behind it. This file quotes no counts or percentages — run the evidence scripts.

## Why

Reading a CycloneDX BOM today means hand-written `jq` or a SQL-ish query language. Neither lets
you *navigate the dependency graph*, which is the structure that answers the question people
actually bring to a BOM: **what pulled this in?**

## What it is not

- Not a BOM **generator** — use [syft](https://github.com/anchore/syft),
  [cdxgen](https://github.com/CycloneDX/cdxgen) or trivy.
- Not a BOM **query tool** — [`CycloneDX/sbom-utility`](https://github.com/CycloneDX/sbom-utility)
  already does SQL-ish query and list, in Go, under the CycloneDX org. If you want
  `--select/--from/--where`, use it.
- Not a **vulnerability scanner** (grype, osv-scanner, Dependency-Track), a **signer**
  (`cosign`, `cyclonedx-cli`), a **converter** (`cdx-convert`), or an **editor**.
- Not a corpus database. Cross-BOM questions are answered today by the `duckdb` and `lbug` CLIs
  against the files on disk.

A Finder-style **column view** is planned as a second renderer over the same model
([ADR-0006](docs/adr/0006-column-view-as-a-first-class-renderer.md)); `ls` and `tree` stay, because
pipelines need non-interactive output.

## Design in one paragraph

A CycloneDX `dependencies` array is a **DAG with cycles**, the declared root is often absent from
it, and an OBOM carries no graph at all. So `ls` is the universal command — over components, or
over `cdx:osquery:category` for an OBOM — and `tree` is the conditional one, for BOMs that have a
graph. Coverage is always reported: a partial graph is never rendered as complete.

## Documentation

| | |
|---|---|
| Design, risks, decisions | [`docs/inception/spark-analysis.md`](docs/inception/spark-analysis.md) |
| Reproducible measurements | [`docs/inception/evidence/`](docs/inception/evidence/) |
| Audiences and their artifacts | [`docs/reference/audience-registry.md`](docs/reference/audience-registry.md) |
| Decision log | [`docs/adr/`](docs/adr/) |
| Phases, and what is not a phase | [`docs/roadmap/roadmap.md`](docs/roadmap/roadmap.md) |
| Watched: cgo-free graph backend | [`docs/roadmap/watched-pglite-age.md`](docs/roadmap/watched-pglite-age.md) |

## License

Not set — this project is private and unpublished.
