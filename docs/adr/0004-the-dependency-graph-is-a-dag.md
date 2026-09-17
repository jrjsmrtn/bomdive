# 4. The dependency graph is a DAG, and how tree renders it

Date: 2026-09-10

## Status

Accepted. **Amended by [ADR-0006](0006-column-view-as-a-first-class-renderer.md)**, which
repositions `tree` as one renderer of this model rather than the design centre, and adds a lazy
one-level-at-a-time expansion requirement so a column view is a second renderer over the same
traversal. Every decision below still holds — they are properties of the model, not of `tree`.

## Context

The project's premise was that a BOM can be browsed like a filesystem: `ls` a component, `tree`
the whole thing. Measurement showed the premise is wrong in three specific ways. All figures below
are reproducible with `docs/inception/evidence/bom-graph-shape.py` and
`poc2-corpus-shape.py`; the raw results are in the same directory.

**1. It is a DAG with cycles, not a tree.** Real Go SBOMs contained shared nodes in quantity and
cycles in ordinary numbers. A naive recursive walk hangs or blows the stack on the first real file
it meets.

**2. The declared root is frequently not in the graph.** syft's `metadata.component.bom-ref` is a
content hash while the graph is keyed by purl, so walking from the declared root reaches **zero**
components. A tool doing the obvious thing prints an empty tree and looks broken.

**3. Graph coverage varies by an order of magnitude between generators, and an OBOM has no graph
at all** — `dependencies` present and **empty** in every OBOM measured. HBOMs are depth-1.

Referential integrity, by contrast, was perfect in every file measured: every `dependsOn` target
resolved to a real component. The graph is well-formed; it is rooted differently and sparsely
populated.

## Decision

### Roots are derived, and labelled when synthetic

Roots come from **in-degree-0 nodes** in the dependency graph, not from `metadata.component`.
When the declared root is absent, the derived roots are rendered as **synthetic** and say so. A
tool that silently invents a root is making a claim the document does not support.

### Traversal is cycle-safe, and the semantics are a stated choice

A visited set, with repeat encounters rendered as an explicit back-reference rather than silently
truncated.

⚠ **Cycle semantics are a real fork, not an implementation detail.** For the same fixture,
Cypher-style **relationship**-uniqueness (an edge may not repeat; a node may) and
**node**-uniqueness give *different* answers to "what is reachable from x" — both were reproduced
during analysis. **bomdive adopts node-uniqueness**, because for the question a BOM reader is
asking — *what does this pull in?* — reporting a component as its own dependency is confusing
rather than correct. This is a decision, is documented in `--help`, and is fixture-tested.

### Coverage is a correctness guarantee, not a flag

`tree` reports reached / total on every invocation, in **both** the human rendering and
`--output json`. A clean three-node tree for a nine-thousand-component BOM, with nothing saying
most of it is not in the graph, is worse than `jq` — it is confidently wrong. This is not
`--verbose` material.

### A graphless BOM is a normal document

`dependencies: []` is the document stating it has no relations, which differs from a generator
having failed to compute them. `tree` must distinguish "this BOM declares no graph" from "this
tool found no graph", and direct the user to `ls` rather than printing nothing.

### How the tool knows which kind of BOM it has

Not by sniffing content. `metadata.lifecycles[]` and `metadata.component.type` together
discriminated every real BOM measured, and the classifier is
`docs/inception/evidence/poc7-bom-type-discriminator.py`.

⚠ A lifecycle entry is **either** `{"phase": ...}` **or** `{"name": ..., "description": ...}`.
Reading only `.phase` silently misclassifies a BOM using the custom form. A BOM may also carry no
`metadata.component` at all, which is schema-valid, so "undeclared" is a branch the classifier must
have rather than an error.

⚠ **The exception is now MEASURED** — see
`docs/inception/evidence/poc8-merged-host-view-2026-09-10.md`. A merged host view
(`hbom --include-runtime`) does carry a real graph: 33 edges, root in the graph, the documented
`cdx:hostview:*` topology links present, and genuine hardware→runtime linkage.

**And it reaches 31 of 4,952 components — 0.6%**, the sparsest coverage measured anywhere in this
project. So the exception is real but narrow: a host-level `tree` is meaningful, and rendering it
without saying what it omits would be the most misleading output this tool could produce. It is the
strongest case for coverage being a guarantee rather than a flag.

⚠ It classifies as an **HBOM**, not a distinct type, so nothing can tell a merged view from a plain
one except the presence of runtime components.

### Therefore ls and tree are not symmetric

`ls` is the universal command and works on every BOM type — over components, or over the
`cdx:osquery:category` property for an OBOM, which nearly every component in one carries.
`tree` is conditional, and pays off on container-image SBOMs and directory scans.

⚠ **ADR-0006 gives the fuller statement**: the difficulty above comes from `tree` rendering the
whole graph at once. A Miller-column view renders one path at a time and so avoids the shared-subtree
and cycle-rendering decisions entirely, while also working on the graphless BOMs where `tree` is
meaningless. The model must therefore expand **lazily, one level from an arbitrary node**, with
`tree` as a complete walk of that same model.

## Consequences

**Positive**: the tool is honest on the most common document in a real corpus; cycles are handled
from the first commit rather than retrofitted; the coverage line is the concrete reason to prefer
this over `jq`.

**Negative**: two commands with genuinely different behaviour per input type is a documentation
burden. `ls -l` cannot have a fixed column set, because meaningful columns for an OBOM depend on
the osquery category.

**Reversibility**: the node-uniqueness choice is the reversible part and is isolated behind one
predicate. The coverage guarantee is not intended to be reversible.

## References

- `docs/inception/evidence/` — the measurements, re-runnable
- **`docs/inception/evidence/poc6-cycle-semantics.sh`** — runs the edge-uniqueness vs
  node-uniqueness experiment this record's traversal decision rests on, and asserts both answers
- `docs/inception/evidence/poc6-backend-survey-2026-09-10.md` — where that split was found, and why
  it is a property of the question rather than a bug in either engine
- `docs/inception/spark-analysis.md` — R1, R2, R7 and POC-2
