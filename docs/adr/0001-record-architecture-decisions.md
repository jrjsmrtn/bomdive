# 1. Record Architecture Decisions

Date: 2026-09-10

## Status

Accepted

## Context

lsxbom requires a systematic way to record significant technical decisions. The project began with
a SPARK analysis whose measurements **overturned the initial design twice**, and that kind of
reversal is exactly what gets lost between sessions if it is not written down at decision time.

This matters more than usual here because the project is AI-assisted and single-contributor: there
is no second person whose memory serves as a backup for why something was done.

## Decision

We will use Architecture Decision Records (ADRs) to document significant decisions.

**Location**: `docs/adr/`.

**Format**: Michael Nygard's — Title, Status, Context, Decision, Consequences. Titles use the
adr-tools format `# N. Title`, which Structurizr's `!adrs` importer requires.

**Numbering**: sequential four-digit, no gaps. ADR-0004 is *not* Operations here — this is a
Development-category project, so that slot carries the first project-specific decision instead.

**What warrants an ADR**: technology choices, traversal and rendering semantics, anything that
would be costly to reverse, and **any decision a measurement forced**. That last clause is
project-specific and deliberate.

**Evidence is not duplicated into ADRs.** Where a decision rests on a measurement, the ADR links
to the re-runnable script in `docs/inception/evidence/` rather than quoting a number that will
drift. A number copied into prose has two sources of truth and only one of them updates.

## Consequences

**Positive**: rationale survives the session it was formed in; settled questions stay settled;
a measurement-driven reversal is visible as such.

**Negative**: overhead per decision; ADRs go stale if superseded records are not marked.

## References

- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) — Michael Nygard
- [ADR GitHub Organization](https://adr.github.io/)
