#!/usr/bin/env python3
"""Validate every fixture against the OFFICIAL CycloneDX JSON schema for its own specVersion.

WHY NOT cdx-validate. The installed cdx-validate refuses spec 1.4 and 1.5 outright —
"Unsupported CycloneDX specVersion '1.4'. Supported versions are 1.6, 1.7, 2.0." Since
cyclonedx-go covers 1.0-1.7, bomdive must handle versions that validator will not check,
so the corpus needs a validator that spans the same range the library does.

Schemas are fetched from the CycloneDX specification repo and cached under
testdata/.schema-cache/ (git-ignored). FAILS LOUDLY OFFLINE rather than skipping: a
schema check that silently does not run is the failure mode this corpus exists to avoid.

Usage: validate-schema.py [dir]
"""
import json, pathlib, sys, urllib.request, urllib.error

BASE = "https://raw.githubusercontent.com/CycloneDX/specification/master/schema"
# CycloneDX schemas $ref these siblings.
SIDECARS = ["spdx.schema.json", "jsf-0.82.schema.json"]


def fetch(cache: pathlib.Path, filename: str) -> dict:
    local = cache / filename
    if local.exists():
        return json.loads(local.read_text())
    url = f"{BASE}/{filename}"
    try:
        with urllib.request.urlopen(url, timeout=30) as r:
            data = r.read().decode()
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        print(f"CANNOT VALIDATE: failed to fetch {url} ({exc}).", file=sys.stderr)
        print("  This is a hard failure, not a skip — an unrun schema check "
              "reports success it has not earned.", file=sys.stderr)
        raise SystemExit(2)
    cache.mkdir(parents=True, exist_ok=True)
    local.write_text(data)
    return json.loads(data)


# Fixtures that must FAIL validation, each with the reason it exists. A fixture is only
# here because real documents carry what the schema forbids and bomdive must cope with
# it. The gate fails if one of these VALIDATES — its reason has gone — and if one no
# longer exists, so the list cannot rot into a place to hide a broken file.
EXPECTED_INVALID = {
    "vex-out-of-schema.cdx.json":
        "OpenVEX states and justifications and capitalised severities, as measured in "
        "real public VEX (POC-11); bomdive must show them as written",
}


def main():
    here = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else pathlib.Path(__file__).parent
    cache = here / ".schema-cache"

    try:
        from jsonschema import Draft7Validator
        from referencing import Registry, Resource
        from referencing.jsonschema import DRAFT7
    except ImportError:
        print("CANNOT VALIDATE: jsonschema/referencing not installed.", file=sys.stderr)
        raise SystemExit(2)

    fixtures = sorted(here.glob("*.cdx.json"))
    if not fixtures:
        print("no fixtures found", file=sys.stderr)
        raise SystemExit(2)

    # one shared registry of the sidecar schemas the CycloneDX schemas reference
    resources = []
    for s in SIDECARS:
        doc = fetch(cache, s)
        resources.append((s, Resource.from_contents(doc, default_specification=DRAFT7)))
    registry = Registry().with_resources(resources)

    bad = 0
    names = {f.name for f in fixtures}
    for missing in sorted(set(EXPECTED_INVALID) - names):
        bad += 1
        print(f"  STALE EXEMPTION {missing}: listed as expected-invalid but no such fixture")
    for f in fixtures:
        doc = json.loads(f.read_text())
        ver = doc.get("specVersion")
        schema = fetch(cache, f"bom-{ver}.schema.json")
        validator = Draft7Validator(schema, registry=registry)
        errors = sorted(validator.iter_errors(doc), key=lambda e: list(e.path))
        if f.name in EXPECTED_INVALID:
            if errors:
                print(f"  invalid {f.name} (spec {ver}) — EXPECTED: {EXPECTED_INVALID[f.name]}")
            else:
                bad += 1
                print(f"  UNEXPECTEDLY VALID {f.name}: its exemption no longer holds — remove it")
            continue
        if errors:
            bad += 1
            print(f"  INVALID {f.name} (spec {ver}):")
            for e in errors[:3]:
                loc = "/".join(str(p) for p in e.path) or "<root>"
                print(f"      {loc}: {e.message[:110]}")
        else:
            print(f"  valid   {f.name} (spec {ver})")
    expected = len(EXPECTED_INVALID)
    print(f"{len(fixtures) - expected - bad}/{len(fixtures) - expected} fixtures valid against their "
          f"own schema; {expected} deliberately invalid, and invalid")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
