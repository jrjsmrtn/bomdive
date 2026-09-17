# POC-8 — the merged host view, measured at last

Run 2026-09-10 with `hbom --include-runtime` on a darwin/arm64 host, cdxgen as installed.
**Shape only is recorded**: the document is a live inventory of a personal machine, so it is not
committed and no identifier from it appears here. Defaults are redacted (`--sensitive` was *not*
used).

Closes the one item POC-7 left open, and the exception ADR-0004 carried without evidence.

## What a merged host view is

`hbom --include-runtime` emits **one** CycloneDX document holding both inventories — the HBOM
(NICs, disks, firmware, TPM) *and* the OBOM (processes, services, packages, certificates) — joined
by dependency edges cdxgen adds only on **exact** evidence: interface-name equality, kernel-module
equality, device node or mount path, Secure Boot certificate identifiers. It explicitly refuses
fuzzy name matching.

It mattered because POC-2 measured **0 dependency entries in 14 of 14 OBOMs** and depth-1 in every
HBOM, so `tree` is meaningless for both. The merged view was *documented* to add edges, making it
the only shape in that family where a host-level tree could mean anything.

## Measured

| | |
|---|---|
| components | **4,952** — `application` 3,531, `data` 1,391, `device` 29, `operating-system` 1 |
| dependency entries | 3 |
| edges | **33** |
| root declared, and in the graph | yes / yes |
| max depth | 2 |
| cycles | 0 |
| **components reachable from root** | **31 of 4,952 — 0.6%** |

**The documented topology links are real.** The `cdx:hostview:*` properties are present exactly as
described — `topologyLinkCount`, `linkedRuntimeCategory`, `runtimeAddressCount` — and the edges
resolve to **29 root→`device`** plus **4 `device`→`data`**, which is genuine hardware-to-runtime
linkage.

## The verdict for ADR-0004

**A host-level `tree` is meaningful here — and it shows 0.6% of the document.**

That is the sparsest coverage measured anywhere in this project, beating the 8% cdxgen case. The
merged view links *hardware* to a handful of runtime facts; the other 4,921 components — every
process, package and certificate — are not in the graph at all.

So the exception ADR-0004 carried is **real but narrow**: the shape has a graph, and rendering it
without saying what it omits would be the most misleading output the tool could produce. This is
the strongest evidence yet for coverage as a correctness guarantee rather than a flag.

⚠ It classifies as an **HBOM**, not as a distinct type: root `device`, phases `operations` +
`pre-build`. There is no "merged" BOM type to detect, so a tool cannot tell a merged view from a
plain HBOM except by looking at whether runtime components are present.

## The bug it found

bomdive reported **"no roots found to walk from"** on a document carrying 33 edges.

The declared root drives the graph but is **not listed in `components`** — legitimate, because
`metadata.component` is the *subject* of the document rather than a member of its inventory.
`Roots()` required the root to be a known component and dropped it.

⚠ **Fixing `Roots()` alone made it worse.** The tree then rendered *rooted and empty*, because
`Walk` looked the node up separately. An empty result that looks like an answer is worse than an
error. Resolution now happens in one place — `Node()` — so `Walk`, `Children` and the detail pane
agree.

The shape is now a committed fixture, `root-outside-components`, so the corpus carries it without
needing a host: removing the resolution makes the test fail.
