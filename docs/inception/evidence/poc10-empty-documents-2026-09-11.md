# POC-10 — 40 documents with no inventory, all called "SBOM" and drawn blank

Run 2026-09-11 against the public corpora cache (`sbom-examples` CC0-1.0, cdxgen and syft test
data) with `docs/inception/evidence/poc10-documents-without-components.py`. Public corpora only.

Found by dogfooding: `browse` on a CISA VEX use-case document showed
`SBOM (root application; phases: none)` over an empty column and `coverage 0/0 (0%)`.

## The defect

`checkIsBOM` rejects a document that parses but is not a BOM, and its comment states why:

> A document that parses but is not a BOM would otherwise render as an empty one: "0 of 0
> components", exit 0. That is the confidently-wrong shape this tool exists to avoid.

That shape was being reached one step later, by a document that is a **valid CycloneDX document
and not a bill of materials at all**.

That distinction is the whole finding. `bomFormat: "CycloneDX"` is a format marker, not a claim of
BOM-ness — CycloneDX also carries VEX, and a standalone VEX inventories nothing. This workspace's
glossary already draws the line: the xBOM family table lists SBOM, HBOM, OBOM, CBOM, ML-BOM,
SaaSBOM and MBOM, while VEX sits under *adjacent concerns*. `identify()` defaulted to `KindSBOM`
and had no rule for a document carrying `vulnerabilities` instead of `components`, so a standalone
VEX was labelled an SBOM — a claim the document never makes — and no surface said anything about
the emptiness, so the blank pane was the whole answer.

A standalone VEX is **not** a degenerate SBOM, an empty one, or a failure to inventory. It is a
different kind of statement sharing the same envelope, and the tool now says so on the first line:

```
VEX (a CycloneDX document, not a bill of materials: it inventories nothing)
```

## Measured

| `components` | `vulnerabilities` | documents |
|---|---|---|
| `>0` | absent | 96 |
| `>0` | `0` | 1 |
| absent | `>0` | **25** |
| absent | absent | 11 |
| `[]` | absent | 4 |

**137 CycloneDX JSON documents scanned; 40 carry no components at all.** What those 40 hold instead:

| | |
|---|---|
| vulnerability records — **standalone VEX** | **25** |
| metadata and nothing else | **14** |
| services — the **SaaSBOM** shape | **1** |

**No document in the corpus carries both components and vulnerabilities.** The embedded-VEX case is
therefore unmeasured here, which is exactly why the rule keys on the *absence of components* rather
than on the presence of vulnerabilities — and why `testdata/sbom-with-vex.cdx.json` exists as a
committed fixture for the shape the corpus does not supply.

## The rule

```
VEX  ⟸  vulnerabilities present and non-empty  AND  components absent or empty
```

Placed **last** in `identify()`'s switch: a declared root type is stronger evidence than an absent
component list, so an operating-system-rooted document carrying vulnerability records stays an
OBOM. `TestRootTypeOutranksTheVEXRule` pins that ordering in both directions.

## What is deliberately NOT done

Navigating vulnerabilities or services. The SPARK analysis puts vulnerability *lookup* out of scope
and nothing here changes that — `lsxbom` still lists components only. What changed is that it now
**says so**: an empty component list is explained by naming what the document carries instead,
which is the difference between *"this tool does not cover that"* and *"this tool is broken"*.

The three empty shapes must read differently, and
`TestEveryEmptyDocumentExplainsItselfDistinctly` fails if any two produce the same sentence.

## Re-run

```bash
scripts/check-corpora.sh                                                   # populate the cache
python3 docs/inception/evidence/poc10-documents-without-components.py      # the table above
```
