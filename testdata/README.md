# Fixture corpus

Each fixture isolates **one** shape the traversal core must handle, and each is small enough to
reason about by hand. ADR-0002 makes these the tests that come *first*: every shape here broke a
naive implementation during the inception analysis, and a corpus of well-formed trees would have
tested nothing that matters.

**What each fixture proves is in [`manifest.json`](manifest.json), not restated here** — a
hand-written table beside a machine-readable one is two sources of truth with only one that
updates. The manifest carries, per fixture, a `why` and the `asserts` that pin it.

## Nothing here comes from a real estate

The *shapes* were measured against real BOMs (see `../docs/inception/evidence/`), but every value
is synthetic and generated. A fixture therefore cannot leak a hostname, path or address, and the
corpus can be committed and published without review.

## The three commands

```bash
python3 testdata/generate.py         # regenerate the fixtures (deterministic)
python3 testdata/verify.py           # assert each fixture CONTAINS what its manifest claims
python3 testdata/validate-schema.py  # validate each against its own official CycloneDX schema
```

`verify.py` is the one that matters most. **A fixture named `cycle-direct` that contains no cycle
tests nothing, and nothing would say so** — the suite would pass, the traversal core would be
unproven, and the failure would surface against a real BOM instead. The manifest is a claim;
`verify.py` is the check on it. It self-tests three ways:

```bash
python3 testdata/verify.py --self-test   # detects a cycle / no false positive / catches a mismatch
```

## Schema validation does not use cdx-validate

The installed `cdx-validate` refuses two of the versions in this corpus:

```
Unsupported CycloneDX specVersion '1.4'. Supported versions are 1.6, 1.7, 2.0.
```

`cyclonedx-go` covers **1.0–1.7**, so lsxbom must handle versions that validator will not check.
`validate-schema.py` fetches the official schema for each fixture's own `specVersion` from the
CycloneDX specification repository instead, and all 19 fixtures — 1.4 and 1.5 included — are valid
against it.

⚠ **Its "2.0" is not a released spec.** The specification repository holds schemas up to
`bom-1.7.schema.json` and its newest tag is `1.7.1`; there is no `bom-2.0.schema.json`. Checked
2026-09-10. Do not add a 2.0 fixture on the strength of that message.

⚠ **It needs the network on first run** and caches into `.schema-cache/` (git-ignored). When it
cannot fetch, it exits **2 and says so** rather than skipping — an unrun schema check reports a
success it has not earned.

## Adding a fixture

1. Add a `fixture(...)` call in `generate.py` with a `why` that says what it would catch, and
   `asserts` that pin the property.
2. Run all three commands above.
3. If `verify.py` fails, the fixture does not contain what you think it does — fix the fixture,
   not the claim.
