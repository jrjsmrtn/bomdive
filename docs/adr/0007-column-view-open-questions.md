# 7. Settling the column view's three open questions

Date: 2026-09-10

## Status

Accepted. Closes the open questions
[ADR-0006](0006-column-view-as-a-first-class-renderer.md) left, which gate Phase 2.

## Context

ADR-0006 made the Miller-column view a first-class renderer and deliberately left three questions
open, because each decides whether the view is usable and none could be answered by preference.
All three are now settled by **measurement over the real corpus**, not by taste. Two of the three
contradicted the expectation stated in ADR-0006.

## Decision 1 — Direction is a MODE, not a leftward move

**The column chain is a path, not a hierarchy.** In a filesystem, going left goes to the parent,
and the parent is also where you came from — those coincide. **In a DAG they do not**: a component
has many dependents, so "the parent" is not a thing. Moving left retraces the route you took;
it is history, not an edge.

So the reverse edge is a **whole-view mode**, not a direction per column:

- **forward** (default) — each column lists what the selection *depends on*
- **reverse** — each column lists what *depends on* the selection

The mode flips the entire chain and **the column header states which way the view faces**, because
the two are visually identical and silently swapping them would make the view lie.

**Reverse is entered from a selection, not from the start.** You cannot begin at "what depends on
X" without first finding X, so forward browsing and filtering are how you arrive, and the flip is
what you do next. That is the shape of the actual question — *what pulled this in?*

### Measured: the reverse direction is MORE column-friendly, not less

| | max | avg | p50 | p95 |
|---|---|---|---|---|
| fan-out (`dependsOn`) | 28 | 3.5 | 1 | 12.6 |
| **fan-in (dependents)** | 13–28 | **2.0** | 1 | **5.0** |

ADR-0006 worried the reverse edge might be unwieldy. It is the opposite: dependents fit a column
more comfortably than dependencies do.

⚠ **Measured on sparse graphs** — the two SBOMs had 81% and 8% of components in the graph at all.
A complete dependency graph would give a popular library far higher fan-in, and this conclusion
should be re-checked against one before it is relied on beyond these shapes.

## Decision 2 — Columns show the NAME, truncated from the head

**A column shows `name`**, with `@version` appended only when it fits. `purl` and `bom-ref`
never appear in a column; they belong in the detail pane.

Measured identifier lengths across two real SBOMs:

| Field | p50 | p95 | max |
|---|---|---|---|
| `name` | 11 | 34 | 154 |
| `name@version` | 18 | 44 | 155 |
| `purl` | 26 | 53 | 190 |
| `bom-ref` | **54** | 81 | 218 |

In an 80-column terminal showing three columns, each pane is roughly 24 characters. `name` fits at
the median and not at p95; `bom-ref` does not fit at the *median*. That settles what goes where.

### Truncation keeps the TAIL, and this is the finding that surprised us

At a 24-wide column, 797 distinct names need truncating. Ambiguity introduced by each strategy:

| Strategy | Collisions |
|---|---|
| head-truncate — keep the first 23 | **395** |
| middle-elide — 11 head + 11 tail | 100 |
| **tail-truncate — keep the last 23** | **52** |

**ADR-0006 proposed middle-elision. The data says it is nearly twice as bad as keeping the tail.**
Package names are hierarchical with the distinguishing part at the *end* —
`github.com/anchore/clio` is one of many `github.com/anchore/…`, and truncating the head yields
`github.com/anch…`, which identifies nothing. Rendering is therefore `…anchore/clio`.

## Decision 3 — The detail pane has NO per-category schema

The rightmost column shows the selected component's full identity plus **every property, as a
key/value list in document order**.

**No curated per-category layout.** POC-7 established there can be no fixed column set for an OBOM;
the tempting fix is a layout per category, and the measurement shows why that fails:

- **39 categories** carry properties, holding between **1 and 37 distinct keys** each — a 37× spread.
- Curating them means 39 hand-written schemas, each of which is a derived fact copied into code.
- The vocabulary is **platform-dependent and open** (POC-7: 6 universal, the rest split by OS), so a
  new category appears whenever a new kind of host is inventoried — and a schema-driven pane would
  render it blank rather than erroring.

Document order is kept rather than sorting alphabetically, because osquery's field order is the
generator's own and carries meaning that sorting destroys.

## Consequences

**Positive**: every decision is grounded in the corpus and re-measurable; the detail pane cannot go
stale because it has no schema to go stale; direction-as-mode keeps the path honest instead of
pretending a DAG has parents.

**Negative**: tail-truncation looks wrong at first glance — a reader sees `…anchore/clio` where
they expected a prefix — and needs the ellipsis to be unmistakable. A 37-key detail pane will need
scrolling.

**Unresolved**: fan-in was measured only on sparse graphs. If a dense BOM shows fan-in in the
hundreds, reverse mode needs paging or filtering that forward mode does not.

## References

- [ADR-0006](0006-column-view-as-a-first-class-renderer.md) — the questions this closes
- `docs/inception/evidence/poc7-bom-identity-2026-09-10.md` — the category vocabulary
- `docs/roadmap/roadmap.md` — Phase 2, which these unblock
