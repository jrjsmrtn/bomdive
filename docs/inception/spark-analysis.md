# SPARK Analysis — lsxbom

*Conducted 2026-09-10. Method: SPARK (Stakeholders, Problem, Analysis, Risks, Knowledge).*

**Concept**: a Go CLI that reads CycloneDX xBOM JSON (SBOM, OBOM, HBOM) and navigates it with the
muscle memory of `ls(1)` and `tree(1)`.

> ## ⚠ Amended — read this before the body
>
> **This is a dated inception record, not a live design document.** It says what was known and
> decided on 2026-09-10. Later work superseded parts of it, and those parts are **marked in place
> rather than rewritten**, so the record of what was believed at inception survives. Where this
> document and an ADR disagree, **the ADR wins**.
>
> | Superseded here | By |
> |---|---|
> | `tree` as the primary renderer, and the `explore` deferral reasoning | [ADR-0006](../adr/0006-column-view-as-a-first-class-renderer.md) — the column view is a first-class renderer, and the model must expand lazily |
> | "must be a single static binary — no cgo" | Corrected in place below; measured in [POC-5](evidence/poc5-cgo-linking-2026-09-10.md) and [POC-6](evidence/poc6-backend-survey-2026-09-10.md) |
> | The backend comparison | [POC-6](evidence/poc6-backend-survey-2026-09-10.md) — DuckDB, zig, SQLite CTEs, the Cypher survey, the Ladybug↔DuckDB bridge |
> | "no real OBOM or HBOM measured" | [POC-2](evidence/poc2-bom-corpus-2026-09-10.json), then [POC-7](evidence/poc7-bom-identity-2026-09-10.md) for the discrimination rule and category vocabulary |
> | Open questions 1, 2 and 4 | Decided — see the marks on each |
>
> **Still open and unmeasured**: the merged host view (`hbom --include-runtime`), which would
> qualify ADR-0004. Documented, never run — see POC-7.
>
> Phases and scheduling now live in [`../roadmap/roadmap.md`](../roadmap/roadmap.md), not here.

**Naming note**: the working name was `lsbom`, abandoned because macOS ships `lsbom(8)`
(`/usr/bin/lsbom`, "list contents of a bom file") for Installer `.bom` files — a collision in the
same semantic space, on the development platform. `lsxbom` was verified free in `PATH`, in MacPorts,
as a man page, and on GitHub (0 repositories) on 2026-09-10. The `x` is the literal placeholder the
landscape bundle defines: *xBOM is not a format — it is the placeholder, written with a literal `x`*.

---

## S — Stakeholders

### Primary Stakeholders (Direct Users)

| Stakeholder | Role | Needs | Influence | Engagement |
|---|---|---|---|---|
| Supply-chain engineer at a terminal | Reads a BOM someone else generated | Answer "what is in here, and what pulled it in?" without a browser or a jq incantation | **High** (is the author) | Dogfooding — this is the development environment |
| CI/pipeline author | Wires BOM inspection into a gate | Stable, parseable output; meaningful exit codes | Medium | Via `--output json` contract |
| Auditor / reviewer | Spot-checks a released BOM | Faithful rendering; no silent omission | Medium | Via correctness guarantees, below |

### Secondary Stakeholders (Indirect Impact)

| Stakeholder | Interest | Impact | Communication |
|---|---|---|---|
| BOM *generator* authors (syft, cdxgen) | Their output becomes visibly comparable | A viewer exposes graph gaps their users had not noticed | Issue reports upstream, if gaps are defects rather than design |
| CycloneDX community | Another consumer of the spec | Tool Center listing is possible, not required | Only if published |
| MacPorts | Potential packager | A port, if it goes public | Deferred |
| `sbom-utility` maintainers | Adjacent tool, same org, same language | Overlap is real — see Analysis | Differentiate, do not compete |

### Key questions answered

- **Who uses it daily?** One person, initially — the author, inspecting BOMs across this workspace.
- **Who maintains it?** The same person. Single-maintainer risk is real (R6).
- **Who could derail it?** Nobody external. The threat is internal: scope creep into a TUI (R4).
- **Whose domain expertise is needed?** Already held — the `software-supply-chain-landscape`
  sibling is the reference corpus, with per-claim sourcing.

---

## P — Problem Definition

### Problem statement

Reading a CycloneDX BOM at the terminal today means either hand-written `jq` expressions or a
SQL-ish query language. **Neither lets you navigate the dependency graph** — the structure that
answers the question people actually bring to a BOM: *what pulled this in?*

### Current state

- `jq` works but you must know the schema by heart, and it renders nothing.
- `sbom-utility` gives `--select/--from/--where` over flat lists — a *table*, not a graph walk.
- `cyclonedx-cli` converts, diffs, merges, signs; it does not browse.
- `kubernetes-sigs/bom document outline` does render structure — for **SPDX**, not CycloneDX.
- Browser visualisers exist, which is the wrong medium for a BOM that arrives in a pipeline.

The workaround in practice is jq plus scrolling, or giving up and opening Dependency-Track.

### Desired future state

`lsxbom ls <ref>` answers "what does this depend on" in one keystroke-length command, and
`lsxbom tree` renders the whole graph honestly — including the parts the BOM does not contain.

### Scope boundaries

**In scope (v0.1)**
- Read CycloneDX **JSON**, spec 1.0–1.7, autodetected.
- **`ls` — the universal command.** Lists components, with `ls(1)` flag vocabulary, and *optionally*
  scoped to one level of the dependency graph for a ref. Works on every BOM type measured, including
  the 14 that have no dependency graph at all.
- **`tree` — the conditional command.** Recursive walk, depth-capped, **cycle-safe**, DAG-aware.
  Refuses honestly, rather than printing nothing, when the BOM carries no graph.
- **`--type`** filtering, which is `ls`-shaped and is the *primary* navigation axis for a graphless
  BOM: POC-2 found `application`, `data`, `library`, `operating-system`, `cryptographic-asset`,
  `framework`, `device` and `firmware` in real files.
- Synthetic roots when the BOM declares none (measured to be the common case — see Analysis).
- A **coverage report**: what fraction of components the dependency graph actually reaches.
- `--output json` for machine consumers.

⚠ **The `ls`/`tree` split is no longer symmetric, and POC-2 is why.** `ls` is the command that always
works; `tree` is the one that pays off when a graph exists. The original framing had them as equals.

**Out of scope (explicitly excluded)**
- *Generating* BOMs — syft, cdxgen and trivy own that, and doing it badly is worse than not.
- *Writing* or patching BOMs — `sbom-utility patch/trim` exists; and see the gojq key-order
  constraint in Analysis, which makes round-tripping actively unsafe through the query path.
- Vulnerability lookup — that is grype/osv-scanner/Dependency-Track, and it needs a live database.
- Signing and verification — `cyclonedx-cli` and `cosign` own it; trust decisions are human-led
  per this workspace's `CLAUDE.md`.
- Conversion between formats — lossy, and `cyclonedx-cli` already documents its own losses.

**Deferred (future consideration)**
- ~~**`explore` (interactive TUI)** — a different project with different dependencies, testing story
  and failure modes. Deferred deliberately; see R4.~~
  **⚠ This reasoning is retired by [ADR-0006](../adr/0006-column-view-as-a-first-class-renderer.md).**
  The deferral was justified by the idea being *unspecified*; a Miller-column view is a specified
  interaction with a known shape, and it is the only renderer that works on the graphless BOMs where
  `tree` is meaningless. It is still not in v0.1, but that is now sequencing, not vagueness.
- SPDX input — `protobom`-style normalisation would be the route, not a second parser.
- XML input — free from the chosen library, but untested, so unclaimed until it is.
- `diff` between two BOMs of the same artifact.

### Success criteria

1. Renders a graph from a real syft-generated SBOM **without hanging on the cycles it contains**
   (measured: 3 cycles in a 153-component BOM, 31 in a 9,314-component one).
2. **Never presents a partial graph as complete.** Coverage is stated, not implied.
3. `tree` over a 10 MB / 9,314-component BOM completes in under 2 seconds.
4. A user who knows `ls` and `tree` needs no manual for the common cases.

---

## A — Analysis

### Existing solutions

| Solution | Lang | Pros | Cons | Why not sufficient |
|---|---|---|---|---|
| [`CycloneDX/sbom-utility`](https://github.com/CycloneDX/sbom-utility) | **Go**, Apache-2.0 | Official org; `query` with `--select/--from/--where`; `component/license/resource/vulnerability list`; `validate`, `patch`, `trim`, `diff`; txt/csv/md/json out | Flat, table-shaped results | **No graph navigation.** Answers "which components match X", never "what pulled X in" |
| `cyclonedx-cli` | .NET | convert, diff, merge, validate, sign/verify; spec 1.0–1.7 | Not a viewer; lossy conversion (documented upstream); PKCS1 signing, not Sigstore | Does not read *out* a BOM for a human |
| [`kubernetes-sigs/bom`](https://github.com/kubernetes-sigs/bom) | Go | `document outline` is exactly the structure-rendering idea, and it works | **SPDX**, not CycloneDX | Wrong format; also a generator, which we are not |
| `jq` / `gojq` | C / Go | Universal, precise | You must hold the schema in your head; renders nothing | The status quo we are replacing |
| Browser visualisers | JS | Good graph rendering | Wrong medium — BOMs arrive in pipelines and over ssh | Not terminal-native |

**Differentiation, stated plainly**: the query half of this space is occupied, in Go, by the
CycloneDX org itself. **The graph-navigation half is empty.** That must be the core of the tool,
not a veneer over a query engine — otherwise the honest recommendation to a user is "use
sbom-utility".

### Technology options

| Option | Fit | Maturity | Experience | Decision |
|---|---|---|---|---|
| **Go** + [`CycloneDX/cyclonedx-go`](https://github.com/CycloneDX/cyclonedx-go) | **High** | Apache-2.0, spec 1.0–1.7, JSON **and** XML, v0.12.0+ (needs Go 1.25+; go1.27.1 present) | Stated preference; `ansible-bom` next door is Go | **Adopt** |
| Go + hand-rolled structs | Low | n/a | n/a | **Reject** — re-implements a maintained first-party library, and the spec moves |
| Rust | Medium | `cyclonedx-rust-cargo` exists | Less | **Reject** — no advantage here; splits the workspace toolchain |
| Python | Low | `cyclonedx-python-lib` is good | High | **Reject** — single-binary distribution is the point |
| [`itchyny/gojq`](https://github.com/itchyny/gojq) as query escape hatch | Medium | Pure Go, embeddable, no cgo | Moderate | **Adopt, constrained** — read-only (see below) |
| `cobra` for CLI | High | Ubiquitous; already in the `ansible-bom` dependency graph | High | **Adopt** |

**The gojq constraint is load-bearing**: gojq does not preserve object key order and has no
`keys_unsorted` or `-S`. That is harmless for querying and **disqualifying for emitting a BOM** —
which is a second, independent reason writing BOMs is out of scope. Never emit a BOM through gojq.

### Constraints

**Technical**
- CycloneDX `dependencies` is a **DAG with cycles**, not a tree — measured, not assumed.
- BOMs reach 10 MB and ~10⁴ components in ordinary use (measured on this machine).
- Must be a single self-contained binary. **Originally stated as "no cgo"; POC-5 (2026-09-10)
  measured that as too strong** — a fully self-contained 30 MB macOS binary with zero non-system
  dylibs was built against a static `liblbug.a`. The constraint that survives measurement is
  **cross-compilation**: pure Go cross-builds to linux/arm64, linux/amd64 and windows/amd64 for
  free; the cgo build does none of them without a per-target C toolchain. Resolved by build tags —
  see `evidence/poc5-cgo-linking-2026-09-10.md`.

**Organisational**
- Single contributor. Argues for t0/t1 and a small v0.1.
- This workspace's `CLAUDE.md`: deep work belongs in a session rooted at the sibling, not here.
- Private profile by default; Private → Public runs through the `public-release` gate and needs
  its own ADR.

### Dependencies

| Dependency | Type | Status | Risk if unavailable |
|---|---|---|---|
| `cyclonedx-go` | Technical | Available, Apache-2.0 | Medium — would mean hand-rolling the model |
| `gojq` | Technical | Available, MIT | **Low** — it is the escape hatch, not the core |
| `cobra` | Technical | Available, Apache-2.0 | Low |
| Go ≥ 1.25 | Technical | **go1.27.1 present** | None |
| Real BOM corpus for tests | Technical | **Generated locally**; syft 1.36.0, cdxgen and grype all present | Medium — fixtures must be committed, not regenerated per run |

### Upstream Acceptance

**This project depends on no upstream accepting anything**, which is the material difference from
`ansible-bom` next door, whose v1.0 is gated on `purl-spec#854` landing. `lsxbom` consumes a
published spec and a permissively-licensed library; nothing needs to be merged anywhere for it to
work.

| Upstream | What we need accepted | Requirement | Met? | Cost |
|---|---|---|---|---|
| CycloneDX Tool Center | A listing | Unknown — only relevant if published | N/A | Low, deferred |
| MacPorts | A port | Portfile review, standard | N/A | Low, deferred |
| syft / cdxgen | *Possibly* bug reports about graph gaps | DCO/CLA and AI-contribution policy **unread** | **Unknown** | Low — check before filing, not before building |

⚠ The last row is the only live one, and it is deliberately **not** a blocker: filing an upstream
issue is a consequence of using the tool, not a precondition for building it. Read the target's
`CONTRIBUTING`, DCO/CLA and AI-contribution policy — in all five places the skill names — *before
filing*, not before starting.

---

## R — Risk Assessment

### Risk register

| ID | Risk | Category | L | I | Score | Mitigation |
|---|---|---|---|---|---|---|
| **R1** | **The dependency graph is absent, partial, or unrooted in real BOMs** — so `tree` renders little or nothing | Technical | **H** *(measured)* | **H** | **9** | Synthetic roots from in-degree-0 nodes; mandatory coverage reporting; fixtures from both generators |
| **R2** | Cycles hang or duplicate the walk | Technical | **H** *(measured)* | H | **9** | Visited-set with explicit back-reference rendering; cycle fixtures in the test suite from day one |
| **R3** | Overlap with `sbom-utility` leaves the tool with no reason to exist | Strategic | M | H | 6 | Graph navigation is the core; query is explicitly the secondary escape hatch |
| **R4** | Scope creep into an interactive TUI before the graph model is proven | Schedule | **M** | M | 4 | `explore` deferred out of v0.1 by decision, recorded here |
| **R5** | Performance on 10 MB / 10⁴-component BOMs | Technical | M | M | 4 | Budget in the success criteria (<2 s); benchmark against the measured 10 MB fixture |
| **R6** | Single maintainer; project stalls | Resource | M | M | 4 | Keep v0.1 genuinely small; t0/t1 scope; no public commitment until it works |
| **R7** | Rendering a partial graph as if complete — a *confidently wrong* answer | **Correctness** | M | **H** | 6 | R1's coverage line, promoted to a correctness guarantee rather than a nicety |

### Top 3 risks

**1. R1 — the graph may not be there (measured, not hypothetical)**

This is the finding that most shapes the project. Measurements taken 2026-09-10, reproducible via
`docs/inception/evidence/bom-graph-shape.py`, cross-checked with independent `jq` expressions:

| BOM | Generator | Components | Dependency entries | Root declared? | **Root in graph?** | Shared nodes | Cycles | Max depth |
|---|---|---|---|---|---|---|---|---|
| `ansible-bom` | **syft 1.36.0** | 153 | 70 | yes | **no** | 48 | **3** | 12 |
| `cdxgen` tree | **syft 1.36.0** | 9,314 | 488 | yes | **no** | 288 | **31** | 17 |
| `ansible-bom` | **cdxgen** | 40 | 41 | yes | **yes** | 23 | 0 | 1 |

Read that table carefully, because three separate things are wrong with the naive design:

- **The declared root is not a node in the graph** for either syft BOM. `metadata.component.bom-ref`
  is a content hash (`38f5aa7af27a583d`); the graph is keyed by purl. Walking from the declared root
  yields **zero** components. A tool that does the obvious thing prints an empty tree and looks broken.
- **Coverage varies by an order of magnitude between generators.** syft entered 70 of 153 components
  into the graph (46%); on the larger tree, 488 of 9,314 (**5%**). cdxgen entered 41 for 40 (~100%)
  but flat, at depth 1. Neither is *wrong* — this is exactly the six-SBOM-types distinction the
  `software-supply-chain-landscape` sibling records: *two SBOMs of one artifact can disagree without
  either being wrong*.
- **It is emphatically not a tree.** 48 and 288 shared nodes respectively.

  *Mitigation*: derive roots as **in-degree-0 nodes** rather than trusting `metadata.component`
  (the syft BOM has exactly **2** such nodes — a workable top level, like `ls` at a filesystem root).
  Report coverage on every `tree` invocation. **Contingency**: if coverage proves uselessly low across
  generators, the tool pivots to `ls`-over-components with graph navigation as a bonus where present —
  still useful, less differentiated.

  *One thing that is reassuring*: **referential integrity was perfect** in every file measured —
  all 123 syft `dependsOn` targets and all 38 cdxgen ones resolve to real components. The graph is
  well-formed; it is merely rooted differently and sparsely populated.

#### POC-2 — the OBOM/HBOM corpus (run 2026-09-10, 23 real BOMs)

Measured over a real corpus of cdxgen-generated BOMs from a private infrastructure estate (host
identifiers withheld by construction — the script emits shape only, and the anonymisation was
verified against a planted known-bad value). Reproduce with
`evidence/poc2-corpus-shape.py <out.json> <corpus-dir>`; raw output in
`evidence/poc2-bom-corpus-2026-09-10.json`.

| Kind | n | Components (range) | With a dependency graph | Root in graph | Cycles | Max depth |
|---|---|---|---|---|---|---|
| **OBOM** | 14 | 2,107 – 6,050 | **0 of 14** | 0 | 0 | **0** |
| **HBOM** | 4 | 29 – 81 | 4 of 4 | 4 | 0 | **1** |
| Image SBOM | 5 | 185 – 5,704 | 5 of 5 | **5** | 1 | 5 – 11 |

**This resolves the open question and changes the design.**

- **An OBOM has no dependency graph at all.** Not sparse — *absent*, in all fourteen files, across
  thousands of components each. `tree` is not "less useful" for an OBOM; it is **meaningless**. This
  is coherent with what the landscape bundle records: an OBOM is a *deployment* inventory — OS,
  hardware, configuration — and there is no dependency relation to express.
- **An HBOM is one level deep by construction.** Exactly one dependency entry, rooted, depth 1:
  a host with devices hanging off it. That is an `ls` shape, not a `tree` shape.
- **Container-image SBOMs are the best-shaped BOMs in the corpus** — root genuinely in the graph,
  99–100% of components reachable (one outlier at 30%), depth 5–11, with cycles present in one.
  **This is where `tree` earns its place**, and it is a stronger case than the directory scans of
  POC-1.
- **`cryptographic-asset` components appear inside OBOMs and image SBOMs** (260 and 271
  respectively) — CBOM content embedded in another BOM type, which `--type` filtering exposes for free.

*Consequence*: the tool must **detect the shape and say so**, rather than assume one. A BOM with no
`dependencies` array is a legitimate, common document, not a broken one.

**2. R2 — cycles are present in ordinary Go SBOMs**

3 cycles in a 153-component BOM; 31 in a 9,314-component one. A naive recursive walk hangs or blows
the stack on the first real file it meets. *Mitigation*: visited-set from the first commit, with a
rendered back-reference (`-> already shown`) rather than silent truncation, and the measured cyclic
BOMs committed as fixtures. **Contingency**: none needed — this is solved computer science, it just
has to be done on day one rather than retrofitted.

**3. R7 — being confidently wrong**

The failure mode that would make the tool worse than jq: showing a clean 3-node tree for a BOM with
9,314 components, with nothing saying that 95% of them are not in the graph. *Mitigation*: coverage
is a **correctness guarantee**, not a `--verbose` flag — `tree` states reached/total, and a synthetic
root is labelled as synthetic. **Contingency**: none; this is non-negotiable, and it is the argument
for the tool over `jq` in the first place.

### Risk monitoring

Reviewed at each release. R1's measurement script is committed and re-runnable, so drift in generator
behaviour is detectable rather than assumed stable.

---

## K — Knowledge Assessment

### What we know (validated 2026-09-10)

- **`lsbom(8)` exists on macOS** — `/usr/bin/lsbom`, 136,416 bytes. Verified by `man` and `ls -l`.
- **`lsxbom` is free** — absent from `PATH`, MacPorts, `man -w`, and GitHub repository search (0 results).
- **CycloneDX dependency graphs are DAGs with cycles in practice** — measured above, twice, and
  cross-checked with `jq` independently of the analysis script.
- **The declared root is frequently not in the graph** — measured in both syft BOMs.
- **Referential integrity holds** — every `dependsOn` target resolved to a component in all three files.
- **`cyclonedx-go` covers spec 1.0–1.7, JSON and XML**, Apache-2.0.
- **gojq is pure Go and embeddable, and does not preserve key order** (no `keys_unsorted`, no `-S`).
- **`sbom-utility` is Go, Apache-2.0**, and already does SQL-ish query and list.
- **OBOMs carry no dependency graph** — 0 of 14 real files (POC-2). **HBOMs are depth-1** — 4 of 4.
- **Container-image SBOMs are well-rooted and near-fully reachable** — 5 of 5, 99–100% (POC-2).
- **cdxgen emits spec 1.7** consistently across all 23 corpus files.
- **Local toolchain is ready**: go1.27.1, syft 1.36.0, cdxgen, grype, jq 1.8.2, cosign.

### What we assume (needs validation)

| Assumption | Basis | How to validate | Priority |
|---|---|---|---|
| `ls`/`tree` metaphor is genuinely more usable than a query language | Ergonomic argument, untested on a second person | Build v0.1, use it for a week on real BOMs | **High** |
| An OBOM/HBOM is worth rendering as a tree at all | Untested — no real sample measured | **POC-2 below** | **High** |
| <2 s on a 10 MB BOM is achievable in Go | Plausible; unmeasured | Benchmark once parsing exists | Medium |
| XML input works for free via `cyclonedx-go` | Library claims it; untested here | One fixture | Low |

### Knowledge gaps (research needed)

| Gap | Impact if unfilled | Research approach | Owner |
|---|---|---|---|
| ~~No real OBOM or HBOM has been measured~~ — **CLOSED by POC-2, 2026-09-10** | Was: two of three types unvalidated. Answered: `tree` is meaningless for an OBOM and trivial for an HBOM | Done — 23-file corpus measured | Author |
| Whether a graphless BOM should make `tree` **fail**, warn, or fall back to `ls` | Decides the tool's most common interaction with the most common BOM type in the corpus | Prototype all three against the OBOM fixtures | Author |
| Whether syft's sparse graph is a defect or by design | Decides whether an upstream issue is warranted, and how loudly to report coverage | Read syft's CycloneDX encoder docs/issues | Author |
| Whether a synthetic root reads as helpful or confusing | Core UX decision | Prototype both, use for a week | Author |
| Real-world spec-version spread (1.4 vs 1.6 vs 1.7) | Affects test matrix size | Sample BOMs across generators | Author |

### Upstream Acceptance gaps

| Upstream | Requirement to confirm | How to check | Blocks the plan? |
|---|---|---|---|
| syft / cdxgen | DCO vs CLA; **AI-contribution policy**; inbound licence | Read `CONTRIBUTING`, the CoC, security page, and the foundation — the five locations, not just `CONTRIBUTING` | **No** — only gates *filing an issue*, not building |

### Domain expertise

- **CycloneDX schema**: strong → adequate. The `software-supply-chain-landscape` sibling is the
  sourced corpus, with `bom-types/` covering all three target types.
- **Go CLI construction**: adequate → adequate. `ansible-bom` is the in-workspace precedent.
- **Graph rendering in a terminal**: **thin**. Cycle-safe DAG rendering with back-references is the
  one genuinely new piece of engineering.

### Proof-of-concept needs

| POC | Purpose | Success criteria | Effort |
|---|---|---|---|
| **POC-1** *(done, 2026-09-10)* | Establish real BOM graph shape | Cycles, shared nodes and root behaviour quantified across two generators | Done — `evidence/` |
| **POC-2** *(done, 2026-09-10)* | Measure a real **OBOM** and **HBOM** | **Done — 23 files.** `tree` does **not** suit an OBOM (no graph, 0/14) or an HBOM (depth 1). Image SBOMs are the strong `tree` case | Done — `evidence/` |
| **POC-3** | Cycle-safe DAG walk over the measured fixtures | Terminates; renders back-references; coverage line correct | ~half a day |
| **POC-4** | Parse + walk the 10 MB fixture under 2 s | Benchmark passes | ~2 h |
| **POC-5** *(done)* | What relaxing "no cgo" costs | **Done.** Self-contained binary survives cgo; cross-compilation is the real cost | `evidence/poc5-*` |
| **POC-6** *(done)* | Backend survey | **Done.** DuckDB embed and zero-ingest, `zig cc` cross-compilation, build tags, the Cypher survey, the Ladybug↔DuckDB bridge | `evidence/poc6-*` |
| **POC-7** *(done)* | BOM identity and OBOM navigation | **Done.** Discrimination rule, the custom-lifecycle trap, 40-category vocabulary | `evidence/poc7-*` |
| **POC-8** *(done)* | Measure a **merged host view** | **Done.** It has a graph — 33 edges, real hardware→runtime links — reaching **0.6%** of 4,952 components. Found a root-resolution bug | `evidence/poc8-*` |

⚠ **POC-3 and POC-4 remain genuinely pending** — they need code that does not exist yet. They are
Phase 1 acceptance criteria in the roadmap, not stale entries.

---

## SPARK Synthesis

### Viability assessment

| Dimension | Score (1–5) | Notes |
|---|---|---|
| Stakeholder Alignment | 4 | Single primary user who is also the author — clear, if narrow |
| Problem Clarity | 5 | Sharpened by measurement: the gap is graph navigation, not query |
| Solution Feasibility | 4 | Libraries mature, toolchain present; the DAG walk is the real work |
| Risk Manageability | 3 | R1/R2 are measured-high, but both have concrete mitigations |
| Knowledge Readiness | 3 | Strong on SBOM, **untested on OBOM/HBOM** — POC-2 closes it |
| **Overall** | **3.8** | |

### Recommendation

**Proceed with conditions.**

The empty niche is real and narrow: `sbom-utility` already owns query in the same language under the
same org, so the *only* defensible reason for `lsxbom` to exist is terminal-native navigation of the
dependency graph. That is worth building — but the measurements show the graph is sparser, less
rooted and more cyclic than the `ls`/`tree` metaphor assumes, so the project's real content is
**handling those honestly**, not the CLI surface.

**POC-2 sharpened this further.** Across 23 real BOMs, the dependency graph is *absent* in every
OBOM and *trivial* in every HBOM; it is rich only in container-image SBOMs. So `ls` — with `--type`
filtering over a flat inventory of thousands of components — is the command that carries the tool,
and `tree` is the one that distinguishes it where a graph exists. Building `tree` first, on the
assumption that BOMs are trees, would have produced a tool that prints nothing for the most common
document in the corpus.

### Conditions for success

1. **Graph navigation is the core.** If `query` becomes the headline, the honest advice to a user is
   "use sbom-utility" and the project should stop.
2. **Coverage reporting is a correctness guarantee, not a flag.** Never render a partial graph as complete.
3. **Cycle-safety and synthetic roots from commit one**, with the measured cyclic BOMs as fixtures.
4. **`explore`/TUI stays out of v0.1**, until the graph model has been used in anger.
5. **POC-2 runs before the CLI surface is fixed** — if `tree` is wrong for an OBOM, the command
   vocabulary should learn that before it is public.

### Immediate next steps

| # | Action | Owner | When |
|---|---|---|---|
| 1 | POC-2 — generate and measure a real OBOM and HBOM | Author | Before bootstrap |
| 2 | `bootstrap-project` at **t0** (or t1 if the ADR log is wanted immediately) | Author + AI | Next |
| 3 | ADR-0001 record-ADRs; **ADR-0002 "the BOM dependency graph is a DAG, and how we render it"** — the decision this analysis exists to inform | Author + AI | With bootstrap |
| 4 | Commit the measured BOMs as test fixtures | Author | Sprint 1 |
| 5 | POC-3 — cycle-safe walk | Author | Sprint 1 |

### Open questions for stakeholder review

1. ~~**Tier — t0 or t1?**~~ — **DECIDED: t1**, bootstrapped 2026-09-10. A roadmap was later adopted
   as a deliberate deviation without taking the rest of t2; see `CLAUDE.md`.
2. ~~**Distribution intent.**~~ — **DECIDED: Private, `ships-artifacts: no`.** Public is defensible
   later and runs through the `public-release` gate with its own ADR.
3. ~~**Does `lsxbom` render an OBOM at all?**~~ — **ANSWERED by POC-2.** It renders one with `ls`,
   never with `tree`. All three types stay in v0.1, because `ls` covers all three; `tree` is scoped
   to BOMs that carry a graph, which in this corpus means container-image SBOMs and directory scans.
   The remaining decision is what `tree` *does* when asked to walk a graphless BOM (see Knowledge gaps).
4. ~~**Is a synthetic root acceptable UX?**~~ — **ANSWERED by [ADR-0004](../adr/0004-the-dependency-graph-is-a-dag.md)**:
   roots are derived from in-degree-0 nodes and **labelled as synthetic**. The tool neither refuses
   nor silently invents. ADR-0006 softens the question further — a column view shows roots as a
   first column rather than needing a single one.

---

*SPARK conducted with AI assistance (Claude Opus 5). All quantitative claims are reproducible —
see the indexed corpus in [`evidence/README.md`](evidence/README.md), which lists every POC, what it
established, and how to re-run it.*
