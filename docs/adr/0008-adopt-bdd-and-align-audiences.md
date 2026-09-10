# 8. Adopt BDD with Gherkin, and align audiences with ansible-bom

Date: 2026-09-10

## Status

Accepted. Amends [ADR-0002](0002-adopt-development-best-practices.md), which is silent on BDD.

## Context

The `bootstrap-project` house template makes BDD a default for user-facing projects — Web, GUI,
TUI, CLI — and lsxbom is two of those. The audience registry had *already* named BDD features
(`user-ls`, `user-tree`, `api-json`) that did not exist, so the project carried a stated
commitment nothing met. That is the same shape as three other promises this project has found
unenforced: a coverage floor, an evidence-file rule, and a tier trigger.

⚠ **A recommendation against BDD was made and overruled, and the record should say so.** The
argument was that Gherkin's value is a shared language with non-technical stakeholders, that there
are none here, that the Go tests already read as specifications, and that **Gherkin adds no
assertion power** — every real defect in this project was found by mutation testing, including three
tests that ran the right lines and asserted the wrong thing. That argument still stands on its own
terms. It was outweighed by consistency with the house template and with the sibling, which is a
portfolio-level concern the argument did not weigh.

## Decision

### 1. BDD with Gherkin, via godog

Feature files in `features/`, step definitions in `features/steps/`.

**Steps drive `cli.Run` with in-memory streams** — the same path the smoke tests use and the same
path a terminal uses. No subprocess, so a feature runs at unit-test speed and a failure points at
real code rather than at a shell.

**`Strict: true`.** An undefined or pending step **fails**. Verified by planting one: without
strict, a scenario nobody implemented reads as a pass, which is the worst possible property for a
document meant to describe behaviour.

### 2. Features are scoped to what BDD is actually for

Scenarios cover the **user-facing contract** — what a person or a pipeline sees. They do **not**
restate the unit tests. The DAG traversal semantics, cycle handling and identity rules stay in
table-driven Go tests, where mutation testing can reach them.

### 3. Audiences align with `ansible-bom`

The registry adopts that project's **A1–A7 IDs unchanged**, so an audience ID names the same person
in both repositories: a packager is a packager whichever sibling they are holding. The two tools are
opposite ends of one pipeline — one *produces* a BOM for these people, the other lets them *read*
one — so the audiences transfer intact and only the verb changes.

⚠ **One thing genuinely shifts.** A4 (SecOps) and A5 (SBOM toolchain) are *Integration* audiences
for a generator, because they consume its output. For a viewer, **A4 reads BOMs at a terminal and is
Primary**; A5 remains Integration as the `--json` consumer. Recorded because a reader comparing the
two registries will notice the difference and should find it explained rather than look like drift.

### 4. Traceability is enforced, not asserted

`scripts/check-audience-tags.py` checks **both directions**: every feature carries an
`@audience:AN` tag naming an ID the registry defines, and every audience the coverage matrix marks
as owed BDD has at least one scenario. Self-tested against all three failure modes.

Without it this would be a fourth unenforced promise, in the same document that has already carried
one.

## Consequences

**Positive**: the user-facing contract is written in a form a non-programmer can read and a machine
can run; audience IDs mean one thing across the workspace; the registry's traceability claim is now
true.

**Negative**: godog is a new dependency, and the features duplicate coverage the smoke tests already
had — accepted deliberately, since they are read by different people for different reasons.

⚠ **BDD is not where correctness is proven here.** Fourteen scenarios describe the contract; the
defects are caught by mutation testing over the unit tests. Anyone reading the features as the test
suite will overestimate what they guarantee.

## References

- [`ansible-bom`'s registry](https://github.com/jrjsmrtn/ansible-bom/blob/develop/docs/reference/audience-registry.md) — the IDs this adopts
- `docs/reference/audience-registry.md` · `scripts/check-audience-tags.py`
