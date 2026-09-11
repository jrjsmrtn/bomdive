# 9. Browse the vulnerability records in a CycloneDX document

Date: 2026-09-11

## Status

**Proposed.** Two decisions belonged to the maintainer — `CLAUDE.md` makes traversal semantics
human-led — and **both were settled on 2026-09-11, as recommended**:

- **[A]** lsxbom browses records other than components, starting with vulnerabilities: **yes**;
- **[B]** a linked document is loaded when it is **named on the command line**.

**[C] was carried out, and it changed the design.** The one tool within reach, Dependency-Track
5.1.0, does not produce a standalone VEX: its export is VEX **embedded** in an SBOM. Section 1 was
revised for that, as [C] required. **This ADR becomes Accepted when the maintainer accepts the
revised section 1.**

Extends [ADR-0006](0006-column-view-as-a-first-class-renderer.md) and
[ADR-0007](0007-column-view-open-questions.md). It reverses nothing in either.

## Context

### What happens today

lsxbom navigates components. A standalone VEX has none: it is a CycloneDX document, but not a bill
of materials. Since `8667317` it is labelled correctly and explains why it shows nothing. It still
shows nothing — `browse` opens on an empty column.

### What a VEX contains

Read from the CycloneDX 1.6 schema, not from memory:

- **`vulnerabilities[]`** — each record has `id`, `source`, `ratings`, `cwes`, `description`,
  `detail`, `recommendation`, `analysis`, `affects`, and more.
- **`analysis.state`** — one of six values. The schema defines them:

  | State | Schema definition |
  |---|---|
  | `exploitable` | The vulnerability may be directly or indirectly exploitable. |
  | `in_triage` | The vulnerability is being investigated. |
  | `false_positive` | The vulnerability is not specific to the component or service and was falsely identified or associated. |
  | `not_affected` | The component or service is not affected by the vulnerability. Justification should be specified for all not_affected cases. |
  | `resolved` | The vulnerability has been remediated. |
  | `resolved_with_pedigree` | The vulnerability has been remediated and evidence of the changes are provided in the affected components pedigree containing verifiable commit history and/or diff(s). |

- **`affects[].ref`** — "References a component or service by the objects bom-ref". It has two
  forms: a local `bom-ref`, or a **BOM-Link Element**, `urn:cdx:<serial-number>/<version>#<bom-ref>`.
  A BOM-Link points into **another document**, which it identifies by `serialNumber` and `version`.

### Measured

From `docs/inception/evidence/poc11-vex-navigability.py`, over the 137 CycloneDX JSON documents in
the public corpora:

| | |
|---|---|
| standalone VEX documents | **25 of 137** (18%) |
| vulnerabilities per document | 1–19 |
| `affects[]` per vulnerability | 0–3 |
| `analysis.state` | `not_affected` 82, `exploitable` 15, `in_triage` 8, `resolved` 7 |
| local refs that resolve in their own document | **76 of 76** |
| BOM-Link refs that resolve to a component in another corpus document | **79 of 80** |
| SBOMs with VEX embedded in them | 0 |

The shape fits the column view: a few groups (the states), a list in each (the vulnerabilities),
and one short level below (what each affects). It is the same shape as the OBOM axis ADR-0006
adopted — category → component → properties.

⚠ **All 25 samples come from one source, and no tool produced any of them.** Every one is in the
`sbom-examples` corpus — 11 CISA use cases, 13 other use cases, 1 generic example — and none
carries `metadata.tools`. They were written to *illustrate* VEX. What a generator actually emits
is unmeasured. The shape above is the specification's intent, not observed practice. See **[C]**.

### Measured again, on tool output

Precondition [C] was carried out on 2026-09-11. Nine VEX exports from a Dependency-Track 5.1.0
instance were measured with the same script. They were exported read-only from a private portfolio,
with a key whose team holds only `VIEW_PORTFOLIO` and `VULNERABILITY_ANALYSIS_READ`. Only their
shape is recorded: no document, name or identifier is committed.

| | Use cases (public corpora) | Tool output (Dependency-Track 5.1.0) |
|---|---|---|
| shape | standalone — no components | **embedded** — components *and* vulnerabilities |
| documents | 25 | 9 (a tenth export failed server-side, HTTP 500, twice) |
| vulnerabilities per document | 1–19 | **3–1,505** |
| `affects[]` per vulnerability | 0–3 | 1–2 |
| most vulnerabilities on one target | 19 | **1,134** |
| `analysis.state` | four states across 112 vulnerabilities | **none** — all 3,560 carry no analysis |
| most severe rating | 86 of 112 unrated or `none` | **every one rated**: high 1,584, medium 1,577, critical 257, low 102, unknown 40 |
| references | 76 local, 80 BOM-Link | 3,672 local, **no BOM-Links** |
| components | — | **only the affected ones**: 183 of 183, in all 9 documents |
| `dependencies` graph | none | none |

What this changes:

- **Tools produce embedded VEX.** A standalone VEX has so far appeared only in illustrative use
  cases.
- **On untriaged tool output the analysis-state axis is degenerate** — one group of up to 1,505
  entries. Nothing in that portfolio had been triaged. A triaged one would carry states, but none
  has been measured.
- **Severity is complete exactly where analysis is absent**, and absent exactly where analysis is
  present. Neither axis serves both worlds alone.
- **Cross-document links appear only in the use cases.**
- **One component can carry 1,134 vulnerabilities.**

### Scope

The SPARK analysis excludes **vulnerability lookup** — matching components against a vulnerability
database. The roadmap excludes **scanning**. This ADR proposes neither. It reads vulnerability
records that are already written in a file the user supplies, with no database and no network.
That is navigation, which is what this tool is for.

It is also not a query feature. `CLAUDE.md` says lsxbom is not a query tool, and that a `query`
headline would mean the project should stop. Grouping by `analysis.state` is `ls`-shaped, like
`--by-category`. There is no expression language.

It does widen the tool, from "navigates components" to "navigates the records in a CycloneDX
document". That is decision **[A]**.

## Decision

### [A] SETTLED — browse records other than components, starting with vulnerabilities

**Decided 2026-09-11: yes**, as recommended. The reasoning, as proposed:

- Without it, 25 of 137 corpus documents open on an empty pane. The correctness obligations exist
  to stop exactly that: a view that shows nothing reads as a tool that failed.
- The measured shape fits the column model. It needs no new interaction.
- The links resolve, so the value continues into the BOMs lsxbom already navigates.

**Against:** this is the first time lsxbom shows something that is not a component, and each
further record type — services, attestations, definitions — will ask for the same. This ADR admits
**vulnerabilities only**. Each further type must meet an evidence bar and get its own decision; see
*Not in this ADR*.

### 1. The columns — revised after [C]

One view, two shapes. The document decides which applies.

**A standalone VEX** — vulnerabilities, no components:

| Column | Lists | Each entry shows |
|---|---|---|
| 1 | the groups — see *The first column* below | the group and its count |
| 2 | the vulnerabilities in the selected group | `id`, for example `CVE-2021-44228` |
| 3 | what the selected vulnerability affects | the target's name if resolved, otherwise the ref |
| detail | the fields of the selection | see below |

**VEX embedded in an SBOM** — components and vulnerabilities, the shape tools produce. The view
opens on the SBOM as it does today, and adds the vulnerability axis in both directions, inside the
one document:

- **from a component, descend to the vulnerabilities that affect it.** That answers "which of my
  components are exploitable?", which this ADR had deferred behind [B]. For embedded VEX it needs
  nothing but the document.
- **from the grouped vulnerabilities, descend to what each affects**, as in the standalone view.

Switching between the component axis and the vulnerability axis needs its own key. `⇥` already
reverses dependency edges, which is a different operation. The binding is an implementation detail
and gets reviewed like any other.

**The first column groups by `analysis.state` when any vulnerability in the document carries
one, and by most severe rating otherwise.**

- Analysis states are the answer a VEX exists to give, so they win whenever they are present.
- On untriaged tool output they are absent from all 3,560 measured vulnerabilities. Grouping by
  them there yields one group of up to 1,505 entries — the degenerate axis lsxbom already refuses
  for OBOM categories.
- Every one of those 3,560 vulnerabilities is rated, so severity is a complete axis there. The use
  cases are the opposite: 86 of their 112 vulnerabilities are unrated or `none`.
- The view says which axis it chose, and `?` explains why.

Both axes are discovered from the document, not hardcoded — the rule POC-7 set for categories.
States sort alphabetically, as categories do. Severities follow the schema's own order, most severe
first: `critical`, `high`, `medium`, `low`, `info`, `none`, `unknown`. A vulnerability that cannot be placed goes in a group of its own, listed last:
`(no analysis)` or `(no rating)`. Counting it as `not_affected` or `low` would put words in the
document's mouth.

**Scale.** One column must hold up to 1,505 vulnerabilities, and one component's list up to 1,134.
The column view already lists the 4,889 components of a measured OBOM, and `/` filters any column.

**The detail pane** for a vulnerability shows `id`, `source`, `ratings` (severity, score, method),
`cwes`, `description`, `detail`, `recommendation`, the `analysis` (state, justification, response,
detail) and how many entries it affects. For an affected entry it shows the ref, how it resolved
(section 2), the listed `versions` with their status, and — when resolved — the component's own
detail.

### 2. A reference that does not resolve is shown, never dropped

This matches ADR-0004's treatment of a dangling `dependsOn`. Every affected entry is in exactly one
of four states, and the view says which:

| State | Meaning |
|---|---|
| **resolved** | names a `bom-ref` in this document, or in a linked document that was loaded |
| **linked, not loaded** | a BOM-Link whose target document was not supplied. Not an error: the VEX correctly points elsewhere |
| **linked, version differs** | the target's `serialNumber` was supplied, at a different `version`. Not resolved — the view does not guess |
| **names nothing** | a local ref with no match, or a loaded target without that `bom-ref`. Dangling. 1 of 80 in the corpus |

*Linked, not loaded* is deliberately separate from *names nothing*. In the first case the document
is correct; lsxbom was just not given the document it points to.

**A resolution count is reported, as coverage is** — for example `affects: 79 of 80 resolved`.
Showing only the resolved half without saying so would render a partial answer as complete, which
`CLAUDE.md` forbids.

### [B] SETTLED — how a linked document is loaded

80 of the 156 `affects` references in the corpus are BOM-Links. They resolve only if lsxbom has the
target document.

| Option | How | For | Against |
|---|---|---|---|
| **(a) named** | `lsxbom browse app.vex.json app.cdx.json` loads every argument; links resolve among them | explicit; reads only files the user named; deterministic | the user must know which BOM a VEX points to |
| (b) discovered | lsxbom searches the VEX's directory for the linked `serialNumber` | no need to know the file | reads files nobody named; the result depends on what happens to be in the directory; slow on a large one |
| (c) both | (a), plus (b) behind a flag | — | two mechanisms to test and explain |

**Decided 2026-09-11: (a), named on the command line**, as recommended. Discovery can be added later behind a flag without changing (a). The
reverse is not true: once a tool reads unnamed files by default, it cannot easily stop. Under (a),
a BOM-Link to a document that was not supplied shows as *linked, not loaded* and names the serial
number, so the user learns which file to add.

**Identity:** a document matches a BOM-Link by `serialNumber` **and** `version`, exactly as the
link encodes them. The same serial at a different version is not a match.

### [C] Precondition — measure a VEX that a tool produced: carried out 2026-09-11

**Met in purpose, not in letter.** [C] asked for a *standalone* VEX from a tool. The one tool
within reach does not produce one: a Dependency-Track 5.1.0 VEX export is VEX embedded in an SBOM.
Nine exports were measured, above. Their shape differs from the use cases on both axes [C] named —
no `analysis`, and a single target with 1,134 vulnerabilities — so, as [C] required, section 1 was
revised before anything was built.

⚠ **Still unmeasured: a standalone VEX from a tool, and a triaged portfolio.** Every export came
from one untriaged portfolio, so the analysis-state axis has only ever been seen in illustrative
use cases.

### 3. What this ADR does not change

- **ADR-0006's contract:** the model expands one level at a time, from an arbitrary node. The
  column model's entries generalise from `bom.Node` to something that can also be a state or a
  vulnerability. The model already has one entry that is not a component: the category row an OBOM
  opens on.
- **`bom.Graph`** stays a graph of components, and `Coverage` stays about components.
- **`ls` and `tree`** are unchanged. An `ls --by-state`, following `--by-category`, is a likely
  follow-up and needs its own decision.
- **Direction (ADR-0007):** forward is state → vulnerability → affected. **Reverse — from a
  component to the VEX statements about it — answers the question a BOM reader most wants
  answered: which of my components are exploitable?** For a standalone VEX it needs both
  documents loaded, so it depends on [B] and follows later. For embedded VEX — the shape tools
  produce — it needs only the document, and it is part of section 1.

## Not in this ADR, and what would bring each in

| Record type | Evidence today | Trigger for its own decision |
|---|---|---|
| services (SaaSBOM) | 1 sample, nesting depth 2 | a second sample, produced by a tool |
| attestations (`declarations`) | 1 sample, which lsxbom cannot load (CycloneDX/cyclonedx-go#275) | the upstream fix |
| definitions | 0 samples | any sample |

## Consequences

**Positive**

- The 25 corpus VEX documents stop opening on an empty pane.
- The VEX that tools actually produce becomes browsable, by component and by vulnerability.
- lsxbom can answer a question no current command answers: which components a VEX statement is
  about, across documents.
- The four resolution states and the count keep the correctness obligations. Nothing partial is
  rendered as complete.

**Negative**

- lsxbom reads more than one document for the first time. That brings cross-document identity
  (`serialNumber` + `version`), argument handling for several files, and a new kind of failure: a
  linked file at the wrong version.
- The column model must generalise beyond components. Every view feature — filter, detail, the
  descend arrow, the `?` explanation — must handle the new entries.
- It sets a precedent: each further record type will ask to be browsable. The table above states
  the evidence each must bring.

**Risk**

- The analysis-state axis rests on illustrative documents only: every tool export measured was
  untriaged. Falling back to severity is what keeps the view useful if that never changes.

## References

- POC-11 — `docs/inception/evidence/poc11-vex-navigability.py` and
  `docs/inception/evidence/poc11-vex-navigability-2026-09-11.md`
- POC-10 — `docs/inception/evidence/poc10-empty-documents-2026-09-11.md`
- CycloneDX BOM-Link — <https://cyclonedx.org/capabilities/bomlink/>, cited by the schema's own
  description of `bomLinkElementType`
- [ADR-0004](0004-the-dependency-graph-is-a-dag.md), [ADR-0006](0006-column-view-as-a-first-class-renderer.md),
  [ADR-0007](0007-column-view-open-questions.md)
