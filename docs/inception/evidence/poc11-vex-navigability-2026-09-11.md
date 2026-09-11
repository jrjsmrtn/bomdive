# POC-11 — can a VEX be navigated? Three sets of VEX, measured apart

Run 2026-09-11 with `docs/inception/evidence/poc11-vex-navigability.py`, over two sets of documents.
This is the evidence behind [ADR-0009](../../adr/0009-browse-a-standalone-vex.md) and its
precondition [C].

## What was measured, and where it came from

| Set | Source | Documents |
|---|---|---|
| **use cases** | the public corpora cache — `sbom-examples` (CC0-1.0) | 25 standalone VEX |
| **tool output** | VEX exports from a Dependency-Track 5.1.0 instance, private portfolio | 9 embedded VEX |

**The tool output is recorded as shape only.** The exports describe a private portfolio, so no
document, project name, hostname or identifier is committed, here or anywhere in this repository.
They were fetched read-only, with GET requests only, using a key whose team holds exactly
`VIEW_PORTFOLIO` and `VULNERABILITY_ANALYSIS_READ` — the export's documented permission, and no
more. The key that had been tried first held `BOM_UPLOAD` and `VIEW_PORTFOLIO`; the export
refused it with HTTP 403, and a broader key was deliberately not used to get round that.

Ten projects in that portfolio have findings. **Nine exported; one failed with HTTP 500 and an
empty body, twice.** That is a server-side failure on one project, observed and not diagnosed.

⚠ **All 25 use cases come from one source, and no tool produced any of them** — none carries
`metadata.tools`. They were written to illustrate VEX.

## Measured

| | Use cases | Tool output |
|---|---|---|
| shape | **standalone** — no components | **embedded** — components and vulnerabilities |
| vulnerabilities per document | 1–19 | **3–1,505** |
| vulnerabilities, total | 112 | 3,560 |
| `affects[]` per vulnerability | 0–3 | 1–2 |
| most vulnerabilities on one target | 19 | **1,134** |
| `analysis.state` | `not_affected` 82, `exploitable` 15, `in_triage` 8, `resolved` 7 | **none** — all 3,560 `(no analysis)` |
| most severe rating | `(no rating)` 47, `none` 39, `high` 12, `critical` 7, `medium` 7 | `high` 1,584, `medium` 1,577, `critical` 257, `low` 102, `unknown` 40 |
| root component type | `application` 18, none 7 | `operating-system` 5, `library` 2, `container` 2 |
| `dependencies` graph | none | none |
| local refs | 76, all resolve | 3,672, all resolve |
| BOM-Link refs | 80: 79 resolve, 1 names a missing `bom-ref` | **none** |
| components | — | 183, **every one affected by some vulnerability** — the export carries only affected components, in all 9 documents |

No public corpus document is embedded VEX, and no tool export is standalone. The two sets do not
overlap on shape at all.

## A third set: real VEX from public publishers

Measured after ADR-0009 was accepted, and the reason it was amended. The documents come from the
`vex` corpus in `scripts/check-corpora.sh`, which fetches them at run time and caches them in
`.corpora-cache-vex`, apart from the other corpora, so the POC-9 and POC-10 re-runs keep
reproducing what they recorded. **207 documents from seven publishers; lsxbom loads every one.**
169 carry vulnerabilities.

| Source | Licence | Generator | Shape | Vulnerabilities | Analysis | References |
|---|---|---|---|---|---|---|
| Apache Camel ×4 | Apache-2.0 | Dependency-Track 4.10.1 | standalone | 249 | none | `bom-ref` |
| Softing Industrial ×5 | none stated | Dependency-Track 4.13.2, 4.14.1 | standalone | 129 | 2–3 groups per document | `bom-ref` |
| Liquibase ×108 | Apache-2.0 | none recorded | standalone | 686 | `not_affected` only | purl, no version |
| EvergreenImageRegistry ×49 | Apache-2.0 | evergreenctl 1.0.0, trivy 0.58.0 | standalone | 171 | outside the schema | none — no `affects` |
| Moderne feed ×1 | none stated | none recorded | embedded | 2,057 | 3 states | purl, used as `bom-ref` |
| hyades-e2e, Jahia | Apache-2.0, MIT | none recorded | standalone | 1 each | one group each | `bom-ref`, purl |

The first column each document would get:

| | documents |
|---|---|
| carry vulnerabilities | 169 |
| **one group** under the rule first accepted — state whenever any vulnerability has one | **159** |
| split under the amended rule — state if it splits, otherwise severity | 41 |
| split on **neither** axis — mostly single-vulnerability or uniform documents | 128 |

Values outside the schema's own enums, checked against the schema file rather than from memory:

| Source | Value | Count |
|---|---|---|
| EvergreenImageRegistry | state `under_investigation` | 123 |
| EvergreenImageRegistry | justification `vulnerable_code_not_present_execute_path` | 123 |
| EvergreenImageRegistry | severity `MEDIUM`, `NONE`, `HIGH`, `UNKNOWN`, `LOW`, `CRITICAL` | 61, 48, 48, 7, 4, 3 |
| Liquibase | justification `vulnerable_code_not_present` — valid in none of 1.5, 1.6, 1.7 | 42 |

**A vulnerability id is not a key.** 2,169 of the 3,294 records share their id with another record
in the same document: the Moderne feed 1,958 of 2,057, because it writes one record per affected
artifact; Apache Camel 117 of 249; Liquibase 94 of 686; the other sources none. Found by dogfooding
the column view, which showed four identical `CVE-2015-2080` rows in one column. The same lesson as
component names, where `bom-ref` is the only reliable key.

Softing Industrial and the Moderne feed state no reuse terms. By the maintainer's decision they are
fetched for local testing only, never committed or redistributed.

**Two fetch failures were found on the way, and neither was a property of the corpus.** Two
Liquibase paths contain `?` and `%`; sent unencoded, they came back as GitHub `404` bodies, which
the corpus script then skipped as "not a BOM". And one curl build resolved the Moderne feed's CDN
name to addresses that never answered, while the system resolver's did. Both are fixed in the
script, and a planted missing path proves a failed download now fails the run.

## What it means for the design

- ~~**Tools produce embedded VEX.** The standalone shape ADR-0009 was first written around appears
  only in illustrative documents.~~ **Corrected by the third set:** true of one Dependency-Track
  5.1.0 instance only. Dependency-Track 4.10.1, 4.13.2 and 4.14.1 export standalone VEX.
- **Analysis state and severity are complementary, not alternatives.** Where one is present the
  other is largely absent: the use cases carry states and mostly no ratings; the tool output
  carries ratings and no states. A first column fixed to either axis is degenerate on half the
  evidence. ADR-0009 therefore groups by whichever of the two splits the document, state first, and shows
  no grouping column when neither does — 128 of the 169 real documents.
- **The absent states are a fact about this portfolio, not about the tool.** Nothing in it had been
  triaged. A triaged portfolio would carry states; none has been measured.
- **References come in three forms.** Besides a local `bom-ref` and a BOM-Link, Liquibase uses a
  purl with no version, naming no component in its documents.
- **Cross-document BOM-Links appear only in the use cases**, so loading a second document ([B]) is
  needed for the standalone shape and not for what the tool emits.
- **Scale is two orders of magnitude beyond the use cases**: 1,505 vulnerabilities in one
  document, 1,134 on one component.

## Re-run

```bash
scripts/check-corpora.sh                                                 # populate .corpora-cache
python3 docs/inception/evidence/poc11-vex-navigability.py .corpora-cache  # the use cases
python3 docs/inception/evidence/poc11-vex-navigability.py <dir>           # any exports, e.g. <dir>/export/*.json
scripts/check-corpora.sh --corpus vex                                    # fetch the third set
python3 docs/inception/evidence/poc11-vex-navigability.py .corpora-cache-vex  # the third set, by source
```

The tool-output column is not re-runnable from a committed file, by design: it needs a portfolio
and a key, and the exports are private. The script is, and it reports counts only, so it is safe to
run over any private export.
