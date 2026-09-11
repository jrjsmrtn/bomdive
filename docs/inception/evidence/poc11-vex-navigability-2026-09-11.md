# POC-11 — can a VEX be navigated? The use cases say one thing, the tool another

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

## What it means for the design

- **Tools produce embedded VEX.** The standalone shape ADR-0009 was first written around appears
  only in illustrative documents.
- **Analysis state and severity are complementary, not alternatives.** Where one is present the
  other is largely absent: the use cases carry states and mostly no ratings; the tool output
  carries ratings and no states. A first column fixed to either axis is degenerate on half the
  evidence. ADR-0009 therefore groups by state when any vulnerability carries one, and by severity
  otherwise.
- **The absent states are a fact about this portfolio, not about the tool.** Nothing in it had been
  triaged. A triaged portfolio would carry states; none has been measured.
- **Cross-document BOM-Links appear only in the use cases**, so loading a second document ([B]) is
  needed for the standalone shape and not for what the tool emits.
- **Scale is two orders of magnitude beyond the use cases**: 1,505 vulnerabilities in one
  document, 1,134 on one component.

## Re-run

```bash
scripts/check-corpora.sh                                                 # populate .corpora-cache
python3 docs/inception/evidence/poc11-vex-navigability.py .corpora-cache  # the use cases
python3 docs/inception/evidence/poc11-vex-navigability.py <dir>           # any exports, e.g. <dir>/export/*.json
```

The tool-output column is not re-runnable from a committed file, by design: it needs a portfolio
and a key, and the exports are private. The script is, and it reports counts only, so it is safe to
run over any private export.
