# bomdive

[![CI](https://github.com/jrjsmrtn/bomdive/actions/workflows/ci.yml/badge.svg)](https://github.com/jrjsmrtn/bomdive/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/jrjsmrtn/bomdive/badge)](https://scorecard.dev/viewer/?uri=github.com/jrjsmrtn/bomdive)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![REUSE status](https://api.reuse.software/badge/github.com/jrjsmrtn/bomdive)](https://api.reuse.software/info/github.com/jrjsmrtn/bomdive)

Read a CycloneDX document at the terminal — an **SBOM, OBOM or HBOM**, and the **VEX** records
carried inside one or shipped beside it — with the muscle memory of `ls(1)` and `tree(1)`, plus
**Miller columns** (the column browser macOS Finder made familiar). **JSON and XML, spec
1.0–1.7.**

> **Status**: bootstrapped 2026-09-10 as `lsxbom`, renamed `bomdive` on 2026-09-17
> ([ADR-0010](docs/adr/0010-rename-the-tool-to-bomdive.md)). `ls`, `tree` and `browse` work:
> Phases 1, 2 and 2b are done, Phase 3 is not started, and the project is private and unpublished
> — see [`docs/roadmap/roadmap.md`](docs/roadmap/roadmap.md), which is the source of what is
> implemented. Design and the measurements behind it:
> [`docs/inception/spark-analysis.md`](docs/inception/spark-analysis.md). This file quotes no counts
> or percentages — run the evidence scripts.

## Installation

```bash
go install github.com/jrjsmrtn/bomdive/cmd/bomdive@latest   # Go 1.26 or newer
```

From a clone, with the version stamped into the binary:

```bash
go build -ldflags "-X main.version=$(git describe --tags --dirty --always)" \
  -o ~/.local/bin/bomdive ./cmd/bomdive
```

Pure Go, `CGO_ENABLED=0`, so it cross-compiles without a toolchain.

## Quick start

```bash
bomdive ls app.cdx.json                  # the components, flat
bomdive ls --by-category obom.cdx.json   # an OBOM has no graph: group by osquery category
bomdive tree app.cdx.json                # the dependency graph, with coverage reported
bomdive tree --json app.cdx.json         # the same answer for a pipeline
bomdive browse app.cdx.json              # Miller columns
bomdive browse app.cdx.json app.vex.json # follow BOM-Links between documents; v for vulnerabilities
bomdive browse exports/                  # every CycloneDX document in one directory
```

In `browse`: `→`/`←` to move between columns, `⇥` to flip between depends-on and
depended-on-by, `v` for the vulnerability axis, `/` to filter, `?` to explain what the status bar
says, `H` for the keys, `q` to quit.

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

**Miller columns** are the second renderer over the same model
([ADR-0006](docs/adr/0006-column-view-as-a-first-class-renderer.md)), reached with `bomdive browse`.
It carries a vulnerability axis for the VEX records a CycloneDX document may hold, and follows
BOM-Links between the documents — or the one directory — you name
([ADR-0009](docs/adr/0009-browse-a-standalone-vex.md)). `ls` and `tree` stay, because pipelines need
non-interactive output.

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

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Contributions arrive under the
[DCO](https://developercertificate.org/) — sign off with `git commit -s`. There is no CLA.
AI-assisted contributions are permitted and need no disclosure; you sign off, and you must be able
to explain every line under review.

Security reports: [`SECURITY.md`](SECURITY.md) — use GitHub's private vulnerability reporting, not
a public issue. Conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

## License

[Apache-2.0](LICENSE). Every file carries an SPDX header and the repository is
[REUSE](https://reuse.software/) compliant, so `reuse lint` is part of the gates.

The released binary links only Apache-2.0, MIT and BSD-3 dependencies — there is no copyleft floor
on what it distributes.
