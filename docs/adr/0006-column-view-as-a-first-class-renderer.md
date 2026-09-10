# 6. The column view is a first-class renderer, and tree is not the design centre

Date: 2026-09-10

## Status

Accepted. Amends the emphasis of
[ADR-0004](0004-the-dependency-graph-is-a-dag.md) without superseding its decisions.

## Context

The project's opening premise was `ls` and `tree` — browse a BOM like a filesystem. Measurement
then found three things that make `tree` awkward, all recorded in ADR-0004 and reproducible from
`docs/inception/evidence/`:

- the dependency graph is a **DAG with cycles**, so a full render must decide whether to repeat a
  shared subtree or back-reference it, and which cycle semantics to use;
- the declared root is often **absent from the graph**, so roots must be derived;
- **coverage is frequently tiny** — as low as 5% of components appearing in the graph — and an
  **OBOM has no graph at all**, in 14 of 14 real files.

Every one of those is hard *because `tree` renders the whole graph at once*. That is the property
forcing the difficulty, not the data.

**Miller columns** — the macOS Finder column view, from the NeXTSTEP file viewer — render **one path
at a time, lazily**: each column lists the children of the item selected in the column to its left.

⚠ Recorded so nobody goes looking: **`nnn` does not have Miller columns.** It is deliberately
single-pane, with contexts, type-to-nav and plugin previews. `ranger` and `lf` have the column
model. The idea here is the *interaction*, not any particular tool, and this project integrates
with none of them.

## Decision

**The column view is a first-class renderer over the traversal model, and `tree` is one renderer
rather than the design centre.**

Concretely, and this is the part that binds v0.1 even though the view itself is later work:

**The traversal model must expand lazily, one level at a time, from an arbitrary node.** A column
view needs exactly that. A model built as a recursive full-graph walk — the natural shape if `tree`
is the only consumer — would have to be rewritten. `tree` then becomes a complete walk *of the same
model*, not a separate implementation.

### Why the column view suits this data

- **The DAG problem does not arise.** Showing one path at a time means never deciding how to draw
  the diamond. ADR-0004's cycle-semantics choice still governs *reachability queries*, but it stops
  being a rendering decision.
- **The columns are the answer to the question.** The chain left-to-right *is* the dependency path,
  permanently visible — which is the question a BOM reader actually brings. Under `tree` you trace
  indentation upward.
- **Cycles become visible and harmless.** Re-entering a component you already passed simply repeats
  it in the chain. Depth is bounded by how far the user scrolls, not by a recursion guard.
- **It works where `tree` is meaningless.** For an OBOM the three levels are
  category → component → properties, which fits the `cdx:osquery:category` axis POC-7 measured.
  **This is the only interaction model examined that serves both the graph and no-graph cases.**
- **Sparse coverage becomes self-evident** rather than misleading. Column one can list every
  component; you descend only where edges exist, so "most of these have no children" is something
  you see instead of something a coverage line has to warn you about.

### What does not change

ADR-0004's decisions are properties of the **model**, not of `tree`, and all still hold: roots
derived from in-degree-0 nodes and labelled synthetic, cycle-safe traversal, node-uniqueness for
reachability, coverage reported as a correctness guarantee, and a graphless BOM distinguished from a
BOM whose graph was not computed. What changes is emphasis: ADR-0004 called `ls` universal and
`tree` conditional; the fuller statement is that **`tree` and the column view are two renderers of
one model**, and the column view is the one that spans every BOM type.

## Open questions, to be settled before the view is built

> **✅ All three are now settled by [ADR-0007](0007-column-view-open-questions.md), by measurement.
> Two of the three answers contradict what is proposed below** — the reverse edge turned out to be
> *more* column-friendly than the forward one, and middle-elision is nearly twice as bad as keeping
> the tail. The text below is kept as the record of what was expected.

1. **Which direction do columns face?** Finder descends into children. The useful BOM question is
   often the *reverse* edge — what depends on this — which has no Finder analogue. Likely both, with
   the column header stating which way you are facing.
2. **Column width versus purls.** `pkg:golang/github.com/anchore/clio@v0.1.1?package-id=cf14a1bc159bc1a6`
   in a narrow pane. Truncation is load-bearing here, not cosmetic: it decides whether the view is
   usable. Middle-elision, or `name@version` in the column with the full `bom-ref` in the detail
   pane — noting that `bom-ref` is the only unique key (2,708 unique names across 3,110 components).
3. **What fills the rightmost column at a leaf?** The component detail. Per POC-7 that cannot have a
   fixed layout for an OBOM, because meaningful fields depend on the osquery category.

## Consequences

**Positive**: the hardest rendering decisions stop being forced; one model serves both renderers;
the interaction matches what people already know from Finder rather than imitating it.

**Negative**: an interactive view is a larger commitment than v0.1, with its own testing story. It
does **not** replace `ls` and `tree` — pipelines and CI need non-interactive output carrying the
coverage line, and no TUI provides that.

**This supersedes the stated reason for deferring `explore`.** The SPARK analysis deferred it as
"a TUI, unspecified, with different failure modes". Miller columns is a specified interaction with a
known shape, so the vagueness that justified the deferral no longer applies. The *scheduling* is
unchanged — v0.1 is still `ls` and `tree` — but the deferral is now about sequencing rather than
about the idea being ill-defined.

## References

- [ADR-0004](0004-the-dependency-graph-is-a-dag.md) — the model this renders
- `docs/inception/evidence/poc7-bom-identity-2026-09-10.md` — the OBOM category axis the three-level
  column layout rests on
- `docs/inception/evidence/poc2-bom-corpus-2026-09-10.json` — the 14-of-14 no-graph measurement
