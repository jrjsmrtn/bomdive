# POC-9 — the declared root was invisible to every reverse edge

Run 2026-09-11 against the public corpora cache (`sbom-examples` CC0-1.0, cdxgen and syft test
data) with `docs/inception/evidence/poc9-root-outside-components.py`. Public corpora only — no
private document is measured here.

Found by dogfooding: a component in a public npm SBOM carried no `→` in the column view's
`dependents` mode, while `dependencies` plainly declared something that depended on it.

## The defect

`metadata.component` is the **subject** of a document, not a member of its inventory, so it is
routinely absent from `components` while still driving `dependencies`. POC-8 established that and
fixed `Node()` to synthesise such a root.

`resolve()` — the single funnel behind `Children`, `Parents` and `Reachable` — read the component
map **directly** rather than asking `Node()`. So the root resolved when asked for by name and
vanished when reached over an edge:

```
juice-shop@14.1.1 declares dependsOn body-parser@1.20.2   (in `dependencies`)
g.in["body-parser@1.20.2"]                = [juice-shop@14.1.1]
Node("juice-shop@14.1.1")            ok   = true      ← POC-8's synthesised root
g.nodes["juice-shop@14.1.1"]         ok   = false     ← not in `components`
Parents("body-parser@1.20.2")             = 0         ← the edge was dropped
```

Same root cause as POC-8, one layer down: `Node` knew about the root and the two functions built
on it did not.

## Measured

| | |
|---|---|
| CycloneDX JSON documents scanned | **137** |
| documents whose declared root is outside `components` **and** drives the graph | **58 — 42%** |
| components whose only declared dependent was invisible | **1,658** |
| by corpus | cdxgen 46 documents, sbom-examples 12 |

The worst single document is a 837-component npm SBOM where **71** direct dependencies of the root
each reported no dependent at all.

## What was affected, and what was not

- **`Parents` / `HasParents`** — the bug. Reverse navigation could never reach the root, and
  `lsxbom ls --from X --reverse` omitted it.
- **`Children` / forward `tree`** — unaffected: the root appears there as a *source*, and its
  targets are ordinary components.
- **`Roots()`** — unaffected. Its in-degree-0 fallback only runs when the declared root does *not*
  drive the graph, and in that case the root is correctly unresolvable.
- **`Coverage`** — deliberately unchanged. It counts the document's **inventory**; the root is not
  in it. Counting the root would put `InGraph` above `Components` and make a fully covered
  document report as incomplete.

## The fix

`resolve` and `anyResolves` ask `Node()`, so **one** rule decides what a ref names: a component,
or the declared root when that root drives the graph.

`dangling` moved out of the edge-reading loop for the same reason — whether a ref resolves can
depend on an edge seen later, so deciding mid-loop made the answer depend on the order
`dependencies` happened to be written in.

## Re-run

```bash
scripts/check-corpora.sh                                              # populate .corpora-cache
python3 docs/inception/evidence/poc9-root-outside-components.py       # the numbers above
```
