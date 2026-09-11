#!/usr/bin/env python3
"""Generate the lsxbom fixture corpus.

Every fixture isolates ONE shape the traversal core must handle, and each is small
enough to reason about by hand. The corpus is generated rather than hand-written so
it is reviewable as a diff and regenerable; the output is committed so tests are
deterministic and need no toolchain.

NOTHING HERE IS DERIVED FROM A REAL ESTATE. The shapes were measured against real
BOMs (see docs/inception/evidence/) but every value below is synthetic, so a fixture
can never leak a hostname, path or address.

Usage: generate.py [outdir]        (default: the directory this script lives in)
"""
import json, pathlib, sys

SPEC = "1.6"


def bom(spec=SPEC, root=None, components=None, dependencies=None, omit_deps=False,
        metadata_extra=None, omit_components=False, top_level=None):
    doc = {
        "bomFormat": "CycloneDX",
        "specVersion": spec,
        "version": 1,
        "metadata": {"component": root} if root else {},
    }
    if not omit_components:
        doc["components"] = components or []
    if metadata_extra:
        doc["metadata"].update(metadata_extra)
    if not omit_deps:
        doc["dependencies"] = dependencies or []
    # `vulnerabilities` and `services` are siblings of `components`, not children of
    # it: a document may carry them INSTEAD of an inventory.
    if top_level:
        doc.update(top_level)
    return doc


def vuln(cve, ref):
    return {
        "bom-ref": f"vuln-{cve}",
        "id": cve,
        "affects": [{"ref": ref}],
        "analysis": {"state": "not_affected", "justification": "code_not_reachable"},
    }


def vx(vid, *refs, state=None, justification=None, severity=None, score=None, ref=None):
    """One vulnerability record. Every argument is written AS GIVEN, so a fixture can
    carry values the schema forbids when that is the point of the fixture."""
    v = {"bom-ref": ref or f"vuln-{vid}", "id": vid}
    if refs:
        v["affects"] = [{"ref": r} for r in refs]
    analysis = {k: val for k, val in (("state", state), ("justification", justification)) if val}
    if analysis:
        v["analysis"] = analysis
    if severity:
        rating = {"severity": severity, "method": "CVSSv31"}
        if score is not None:
            rating["score"] = score
        v["ratings"] = [rating]
    return v


def lib(name, ref=None, version="1.0.0", purl=True, ctype="library", props=None):
    ref = ref or f"pkg:generic/{name}@{version}"
    c = {"bom-ref": ref, "type": ctype, "name": name, "version": version}
    if purl:
        c["purl"] = f"pkg:generic/{name}@{version}"
    if props:
        c["properties"] = props
    return c


def dep(ref, *targets):
    return {"ref": ref, "dependsOn": list(targets)}


def r(name):
    return f"pkg:generic/{name}@1.0.0"


FIXTURES = {}


def fixture(name, why, asserts, doc):
    FIXTURES[name] = {"why": why, "asserts": asserts, "doc": doc}


# ---------------------------------------------------------------- happy path
fixture(
    "tree-simple",
    "A plain tree. The only fixture where the naive implementation is correct — "
    "so it proves the renderer works, not that it is careful.",
    {"is_tree": True, "cycles_found": 0, "root_in_graph": True},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("app"), lib("a"), lib("b"), lib("c")],
        dependencies=[dep(r("app"), r("a"), r("b")), dep(r("a"), r("c")), dep(r("b")), dep(r("c"))],
    ),
)

# ---------------------------------------------------------------- DAG, not tree
fixture(
    "diamond",
    "app -> a,b -> shared. A shared node with two parents: the reason `tree` must "
    "decide between repeating a subtree and back-referencing it. Measured in every "
    "real SBOM examined.",
    {"is_tree": False, "shared_nodes": 1, "cycles_found": 0},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("app"), lib("a"), lib("b"), lib("shared")],
        dependencies=[dep(r("app"), r("a"), r("b")), dep(r("a"), r("shared")),
                      dep(r("b"), r("shared")), dep(r("shared"))],
    ),
)

# ---------------------------------------------------------------- cycles
fixture(
    "cycle-direct",
    "x <-> y. The smallest cycle. A naive recursive walk does not terminate here.",
    {"cycles_found": 1, "is_tree": False},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("app"), lib("x"), lib("y")],
        dependencies=[dep(r("app"), r("x")), dep(r("x"), r("y")), dep(r("y"), r("x"))],
    ),
)

fixture(
    "cycle-self",
    "A component depending on itself. Degenerate but legal, and it breaks a "
    "visited-set implementation that marks AFTER recursing rather than before.",
    {"cycles_found": 1},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("app"), lib("loop")],
        dependencies=[dep(r("app"), r("loop")), dep(r("loop"), r("loop"))],
    ),
)

fixture(
    "cycle-deep",
    "A four-node cycle reached through two acyclic hops. The cycle is not adjacent "
    "to the root, so a check that only looks one level down misses it.",
    {"cycles_found": 1, "is_tree": False},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("app"), lib("a"), lib("p"), lib("q"), lib("s"), lib("t")],
        dependencies=[dep(r("app"), r("a")), dep(r("a"), r("p")), dep(r("p"), r("q")),
                      dep(r("q"), r("s")), dep(r("s"), r("t")), dep(r("t"), r("p"))],
    ),
)

fixture(
    "cycle-semantics",
    "THE fixture ADR-0004 rests on. x -> y -> x, plus a host-like entry point. "
    "Cypher relationship-uniqueness reports x as reachable from x (x->y->x is two "
    "distinct edges); node-uniqueness does not. Both were reproduced against real "
    "engines. lsxbom picks node-uniqueness; this fixture pins that choice.",
    {"cycles_found": 1},
    bom(
        root=lib("entry", ref=r("entry")),
        components=[lib("entry"), lib("x"), lib("y"), lib("unreachable")],
        dependencies=[dep(r("entry"), r("x")), dep(r("x"), r("y")), dep(r("y"), r("x")),
                      dep(r("unreachable"))],
    ),
)

# ---------------------------------------------------------------- rooting
fixture(
    "rootless",
    "The syft shape, and the one that makes a naive tool print nothing: the declared "
    "metadata.component.bom-ref is a content hash while the graph is keyed by purl, "
    "so walking from the declared root reaches ZERO components.",
    {"root_declared": True, "root_in_graph": False, "components_reachable_from_root": 0},
    bom(
        root={"bom-ref": "38f5aa7af27a583d", "type": "file", "name": "some-project"},
        components=[lib("a"), lib("b"), lib("c")],
        dependencies=[dep(r("a"), r("b")), dep(r("b"), r("c")), dep(r("c"))],
    ),
)

fixture(
    "multi-root",
    "Three in-degree-0 nodes and no usable declared root. Derived roots are therefore "
    "plural, and the renderer must show a forest rather than pick one arbitrarily.",
    {"root_in_graph": False, "indegree_zero": 3},
    bom(
        components=[lib("r1"), lib("r2"), lib("r3"), lib("shared")],
        dependencies=[dep(r("r1"), r("shared")), dep(r("r2"), r("shared")),
                      dep(r("r3"), r("shared")), dep(r("shared"))],
    ),
)

# ---------------------------------------------------------------- coverage
fixture(
    "partial-coverage",
    "Ten components, three in the dependency graph. Rendering the three as if they "
    "were the BOM is the confidently-wrong failure ADR-0004 forbids; coverage must "
    "be reported.",
    {"components": 10, "in_graph_components": 3},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("app")] + [lib(f"c{i}") for i in range(1, 10)],
        dependencies=[dep(r("app"), r("c1")), dep(r("c1"), r("c2")), dep(r("c2"))],
    ),
)

# ---------------------------------------------------------------- no graph at all
fixture(
    "no-dependencies-empty",
    "`dependencies: []` PRESENT AND EMPTY — the OBOM shape, in 14 of 14 real files. "
    "The document is stating it has no relations. `tree` must say so and point at "
    "`ls`, not print an empty result.",
    {"has_dependencies_key": True, "dependency_entries": 0},
    bom(
        root=lib("host-os", ref=r("host-os"), ctype="operating-system"),
        components=[lib(f"pkg{i}") for i in range(1, 6)],
        dependencies=[],
        metadata_extra={"lifecycles": [{"phase": "pre-build"}, {"phase": "operations"}]},
    ),
)

fixture(
    "no-dependencies-absent",
    "The `dependencies` key MISSING ENTIRELY. Distinct from the empty array above: "
    "absent means the generator said nothing, empty means the document asserts there "
    "are no relations. A tool conflating the two reports a claim the BOM never made.",
    {"has_dependencies_key": False},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("a"), lib("b")],
        omit_deps=True,
    ),
)

# ---------------------------------------------------------------- OBOM / HBOM shapes
OSQ = "cdx:osquery:category"
fixture(
    "obom-categories",
    "The real OBOM navigation axis: no graph, and every component carrying "
    "cdx:osquery:category. This is what `ls` groups by when there is nothing to walk. "
    "Categories are platform-dependent in the wild, so the fixture mixes two.",
    {"dependency_entries": 0, "osquery_categories": 4, "has_unsorted_properties": True},
    bom(
        spec="1.7",
        root=lib("host-os", ref=r("host-os"), ctype="operating-system"),
        components=[
            lib("svc-alpha", ref="osquery:launchd_services:data:svc-alpha", purl=False,
                ctype="application", props=[{"name": OSQ, "value": "launchd_services"}]),
            lib("svc-beta", ref="osquery:launchd_services:data:svc-beta", purl=False,
                ctype="application", props=[{"name": OSQ, "value": "launchd_services"}]),
            # Property order here is DELIBERATELY NOT ALPHABETICAL: "ca" sorts before
            # "cdx:osquery:category" but appears last. Without that, a renderer that
            # sorts properties is indistinguishable from one preserving document
            # order, and mutation testing showed exactly that blind spot.
            lib("cert-one", ref="osquery:certificates:data:cert-one", purl=False,
                ctype="data", props=[{"name": OSQ, "value": "certificates"},
                                     {"name": "not_valid_after", "value": "2027-01-01"},
                                     {"name": "ca", "value": "true"}]),
            lib("port-8080", ref="osquery:listening_ports:data:port-8080", purl=False,
                ctype="data", props=[{"name": OSQ, "value": "listening_ports"}]),
            lib("unit-gamma", ref="osquery:systemd_units:data:unit-gamma", purl=False,
                ctype="application", props=[{"name": OSQ, "value": "systemd_units"}]),
        ],
        dependencies=[],
        metadata_extra={"lifecycles": [{"phase": "pre-build"}, {"phase": "operations"}]},
    ),
)

fixture(
    "hbom-depth1",
    "The HBOM shape: one dependency entry, rooted, depth 1 — a host with devices "
    "hanging off it. An `ls` shape, not a `tree` shape.",
    {"max_depth_from_root": 1, "root_in_graph": True},
    bom(
        spec="1.7",
        root=lib("host-device", ref=r("host-device"), ctype="device"),
        components=[lib("host-device", ctype="device"), lib("nic-0", ctype="device"),
                    lib("disk-0", ctype="device"), lib("fw-0", ctype="firmware")],
        dependencies=[dep(r("host-device"), r("nic-0"), r("disk-0"), r("fw-0"))],
        metadata_extra={"lifecycles": [{"phase": "operations"}]},
    ),
)

# ---------------------------------------------------------------- identity
fixture(
    "duplicate-names",
    "Three components sharing a name at different versions, plus two sharing a name "
    "with no version overlap. Measured in a real OBOM: bom-refs were unique across "
    "3,110 components while names were not. Anything keyed on name collides here.",
    {"unique_names_lt_components": True},
    bom(
        root=lib("app", ref=r("app")),
        components=[
            lib("app"),
            lib("dup", ref="pkg:generic/dup@1.0.0", version="1.0.0"),
            lib("dup", ref="pkg:generic/dup@2.0.0", version="2.0.0"),
            lib("dup", ref="pkg:generic/dup@3.0.0", version="3.0.0"),
        ],
        dependencies=[dep(r("app"), "pkg:generic/dup@1.0.0", "pkg:generic/dup@2.0.0",
                          "pkg:generic/dup@3.0.0")],
    ),
)

fixture(
    "purl-less",
    "Components with no purl at all — 28-40% of a real OBOM. bom-ref is the only key "
    "that works; a purl-keyed index silently drops these.",
    {"purl_less_components": 3},
    bom(
        root=lib("host-os", ref=r("host-os"), ctype="operating-system"),
        components=[
            lib("has-purl"),
            lib("no-purl-1", ref="osquery:gatekeeper:data:no-purl-1", purl=False, ctype="data"),
            lib("no-purl-2", ref="osquery:certificates:data:no-purl-2", purl=False, ctype="data"),
            lib("no-purl-3", ref="osquery:alf:data:no-purl-3", purl=False, ctype="data"),
        ],
        dependencies=[dep(r("host-os"), r("has-purl"))],
    ),
)

fixture(
    "root-outside-components",
    "The declared root DRIVES the dependency graph but is not listed in "
    "`components` — legitimate, because metadata.component is the SUBJECT of the "
    "document rather than a member of its inventory. Measured in a real merged "
    "host view (POC-8), where it left the tree with no roots at all on a document "
    "carrying 33 edges.",
    {"root_declared": True, "root_in_graph": True, "components": 3},
    bom(
        root={"bom-ref": "host:machine-1", "type": "device", "name": "the-host"},
        components=[lib("nic-0", ctype="device"), lib("disk-0", ctype="device"),
                    lib("svc-a", ctype="application")],
        dependencies=[dep("host:machine-1", r("nic-0"), r("disk-0")),
                      dep(r("nic-0"), r("svc-a"))],
    ),
)

# ------------------------------------------------- documents with no inventory
fixture(
    "vex-standalone",
    "A standalone VEX: vulnerability records and NO components. CycloneDX carries "
    "VEX either embedded in a BOM or standalone, where the document asserts which "
    "vulnerabilities affect a product described ELSEWHERE. 25 of 137 public corpus "
    "documents are this shape (POC-10), and every one of them was identified as an "
    "SBOM and drawn as an empty component list.",
    {"components": 0, "vulnerabilities": 1, "services": 0},
    bom(
        spec="1.4",
        root={"bom-ref": "product-XYZ", "type": "application", "name": "XYZ"},
        omit_components=True,
        omit_deps=True,
        top_level={"vulnerabilities": [vuln("CVE-2020-25649", "product-XYZ")]},
    ),
)

fixture(
    "sbom-with-vex",
    "Components AND vulnerabilities in one document — VEX embedded in an SBOM. It "
    "must stay an SBOM: the absence of components is what makes a VEX standalone, "
    "so a rule keying on `vulnerabilities` alone would misclassify this.",
    {"components": 2, "vulnerabilities": 1},
    bom(
        spec="1.6",
        root=lib("app", ref=r("app")),
        components=[lib("app"), lib("a")],
        dependencies=[dep(r("app"), r("a"))],
        top_level={"vulnerabilities": [vuln("CVE-2021-44228", r("a"))]},
    ),
)

fixture(
    "services-only",
    "A document carrying services and no components — the SaaSBOM shape. lsxbom "
    "navigates components, so it has nothing to list here and must SAY so rather "
    "than drawing an empty pane.",
    {"components": 0, "services": 2, "vulnerabilities": 0},
    bom(
        spec="1.6",
        root={"bom-ref": "svc-root", "type": "application", "name": "the-platform"},
        omit_components=True,
        omit_deps=True,
        top_level={"services": [
            {"bom-ref": "svc-a", "name": "auth", "endpoints": ["https://auth.example.com/v1"]},
            {"bom-ref": "svc-b", "name": "billing", "endpoints": ["https://billing.example.com/v1"]},
        ]},
    ),
)

fixture(
    "metadata-only",
    "Nothing but metadata: no components, no vulnerabilities, no services. "
    "Schema-valid — the document names a product without inventorying it — and 14 "
    "of 137 public corpus documents are exactly this (POC-10), being the product "
    "half of a CISA VEX use case.",
    {"components": 0, "vulnerabilities": 0, "services": 0},
    bom(
        spec="1.6",
        root={"bom-ref": "product-only", "type": "application", "name": "just-a-name"},
        omit_components=True,
        omit_deps=True,
    ),
)

fixture(
    "attestation-only",
    "A CycloneDX Attestations (CDXA) document: `declarations` and nothing else — no "
    "components, and in the spec's own sample no metadata either. It attests "
    "conformance to a standard; it inventories nothing, so it is not a bill of "
    "materials. Shape taken from the specification's conformance corpus "
    "(`valid-attestation-1.6.json`), contents synthetic.",
    {"components": 0, "attestations": 1, "standards": 0},
    {
        "bomFormat": "CycloneDX",
        "specVersion": "1.6",
        "version": 1,
        "declarations": {
            "assessors": [
                {"bom-ref": "assessor-1", "thirdParty": True,
                 "organization": {"name": "Example Assessors"}},
            ],
            "attestations": [
                {"summary": "Attestation for the example standard",
                 "assessor": "assessor-1",
                 "map": [{"requirement": "req-1",
                          "claims": ["claim-1"],
                          "conformance": {"score": 1.0}}]},
            ],
            "claims": [
                {"bom-ref": "claim-1", "target": "product-only",
                 "predicate": "The product meets requirement 1.",
                 "evidence": ["evidence-1"]},
            ],
            "evidence": [
                {"bom-ref": "evidence-1", "propertyName": "example:evidence",
                 "description": "Synthetic evidence for a synthetic claim."},
            ],
        },
    },
)

fixture(
    "definitions-only",
    "A document whose whole payload is `definitions.standards` — it DEFINES a "
    "standard for others to attest against, rather than inventorying anything. "
    "Unmeasured in every corpus available, including the specification's own "
    "conformance suite, so this fixture is built from the schema rather than from "
    "a sample; that is recorded here because it is a weaker provenance than every "
    "other fixture in this corpus.",
    {"components": 0, "attestations": 0, "standards": 1},
    {
        "bomFormat": "CycloneDX",
        "specVersion": "1.6",
        "version": 1,
        "definitions": {
            "standards": [
                {"bom-ref": "standard-1",
                 "name": "Example Baseline",
                 "version": "1.0",
                 "description": "A synthetic standard, for testing identification only.",
                 "owner": "Example Standards Body",
                 "requirements": [
                     {"bom-ref": "req-1", "identifier": "EB-1", "title": "First requirement",
                      "text": "The product shall do the thing."},
                 ]},
            ],
        },
    },
)

# ------------------------------------------------------ vulnerability records
fixture(
    "vex-embedded-states",
    "Embedded VEX that has been TRIAGED: three analysis states across four "
    "vulnerabilities, so the vulnerability axis groups by state. Component a carries "
    "two vulnerabilities, so component -> vulnerabilities has more than one answer.",
    {"components": 3, "vulnerabilities": 4},
    bom(
        root=lib("app", ref=r("app"), ctype="application"),
        components=[lib("a"), lib("b"), lib("c")],
        omit_deps=True,
        top_level={"vulnerabilities": [
            vx("CVE-2024-0001", r("a"), state="exploitable", severity="critical", score=9.8),
            vx("CVE-2024-0002", r("a"), r("b"), state="not_affected",
               justification="code_not_reachable", severity="high", score=7.5),
            vx("CVE-2024-0003", r("c"), state="resolved", severity="medium", score=5.3),
            vx("CVE-2024-0004", r("b"), state="not_affected",
               justification="code_not_present", severity="low", score=3.1),
        ]},
    ),
)

fixture(
    "vex-embedded-untriaged",
    "Embedded VEX with NO analysis on any vulnerability, the shape a Dependency-Track "
    "5.1.0 export of an untriaged project has (POC-11): every record rated, none "
    "analysed, only the affected components present. Grouping by state would give one "
    "group, so the vulnerability axis groups by severity.",
    {"components": 2, "vulnerabilities": 4},
    bom(
        root=lib("app", ref=r("app"), ctype="application"),
        components=[lib("a"), lib("b")],
        omit_deps=True,
        top_level={"vulnerabilities": [
            vx("CVE-2024-1001", r("a"), severity="high", score=8.1),
            vx("CVE-2024-1002", r("a"), severity="medium", score=6.5),
            vx("CVE-2024-1003", r("b"), severity="critical", score=9.1),
            vx("CVE-2024-1004", r("b"), severity="high", score=7.2),
        ]},
    ),
)

fixture(
    "vex-uniform",
    "Every vulnerability in the same state AND the same severity, so neither axis "
    "splits the document. 128 of 169 real public documents measured are like this "
    "(POC-11). The view must open on the vulnerabilities, with no grouping column "
    "holding a single group.",
    {"components": 1, "vulnerabilities": 3},
    bom(
        root=lib("app", ref=r("app"), ctype="application"),
        components=[lib("a")],
        omit_deps=True,
        top_level={"vulnerabilities": [
            vx("CVE-2024-2001", r("a"), state="not_affected", justification="code_not_reachable", severity="high"),
            vx("CVE-2024-2002", r("a"), state="not_affected", justification="code_not_reachable", severity="high"),
            vx("CVE-2024-2003", r("a"), state="not_affected", justification="code_not_reachable", severity="high"),
        ]},
    ),
)

fixture(
    "vex-refs",
    "One vulnerability per way an `affects` reference can resolve (ADR-0009 section 2): "
    "a local bom-ref; a purl that IS a component's bom-ref; a purl with no version that "
    "names no component; a BOM-Link back into this very document; the same link at a "
    "different version; a BOM-Link to a document not supplied; a string that names "
    "nothing. Plus one vulnerability with no `affects` at all, as 49 real documents "
    "have (POC-11), and one naming the SAME component twice — by bom-ref and by a "
    "BOM-Link — which must still be listed once.",
    {"components": 2, "vulnerabilities": 9},
    bom(
        root=lib("app", ref=r("app"), ctype="application"),
        components=[lib("a"), lib("b", ref="pkg:npm/b@2.0.0")],
        omit_deps=True,
        top_level={
            "serialNumber": "urn:uuid:11111111-2222-4333-8444-555555555555",
            "vulnerabilities": [
                vx("CVE-2024-3001", r("a"), severity="high"),
                vx("CVE-2024-3002", "pkg:npm/b@2.0.0", severity="high"),
                vx("CVE-2024-3003", "pkg:maven/org.example/lib", severity="high"),
                vx("CVE-2024-3004", "urn:cdx:11111111-2222-4333-8444-555555555555/1#" + r("a"), severity="high"),
                vx("CVE-2024-3005", "urn:cdx:11111111-2222-4333-8444-555555555555/2#" + r("a"), severity="high"),
                vx("CVE-2024-3006", "urn:cdx:aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee/1#x", severity="high"),
                vx("CVE-2024-3007", "no-such-ref", severity="high"),
                vx("CVE-2024-3008", severity="high"),
                vx("CVE-2024-3009", r("a"), "urn:cdx:11111111-2222-4333-8444-555555555555/1#" + r("a"),
                   severity="high"),
            ],
        },
    ),
)

fixture(
    "vex-shared-ids",
    "Records that SHARE a vulnerability id within one document — 2,169 of 3,294 real "
    "public records do (POC-11), because a publisher may write one record per affected "
    "artifact. One id on two versions of a package and on a third component; one id "
    "recorded twice against the same component; one id that is unique; one id on two "
    "purls that name no component, as real VEX feeds write them. Shown by id alone they "
    "are identical rows.",
    {"components": 3, "vulnerabilities": 8},
    bom(
        root=lib("app", ref=r("app"), ctype="application"),
        components=[lib("jetty", version="1.0.0"), lib("jetty", version="2.0.0"), lib("c")],
        omit_deps=True,
        top_level={"vulnerabilities": [
            vx("CVE-2024-5001", "pkg:generic/jetty@1.0.0", severity="high", ref="vuln-5001-a"),
            vx("CVE-2024-5001", "pkg:generic/jetty@2.0.0", severity="high", ref="vuln-5001-b"),
            vx("CVE-2024-5001", r("c"), severity="high", ref="vuln-5001-c"),
            vx("CVE-2024-5002", r("c"), severity="high"),
            vx("CVE-2024-5003", r("c"), severity="high", ref="vuln-5003-a"),
            vx("CVE-2024-5003", r("c"), severity="high", ref="vuln-5003-b"),
            vx("CVE-2024-5004", "pkg:maven/org.example/lib-one", severity="high", ref="vuln-5004-a"),
            vx("CVE-2024-5004", "pkg:maven/org.example/lib-two", severity="high", ref="vuln-5004-b"),
        ]},
    ),
)

fixture(
    "vex-out-of-schema",
    "Values the CycloneDX schema forbids and real VEX carries (POC-11): an OpenVEX state "
    "(`under_investigation`), an OpenVEX justification (`vulnerable_code_not_present`), "
    "and severities in capitals. DELIBERATELY SCHEMA-INVALID — validate-schema.py "
    "requires it to fail validation. lsxbom must load it, show each value as written, "
    "and rank MEDIUM with medium.",
    {"components": 0, "vulnerabilities": 4},
    bom(
        root=lib("app", ref=r("app"), ctype="application"),
        omit_components=True,
        omit_deps=True,
        top_level={"vulnerabilities": [
            vx("CVE-2024-4001", "pkg:maven/org.example/one", state="under_investigation", severity="MEDIUM"),
            vx("CVE-2024-4002", "pkg:maven/org.example/two", state="not_affected",
               justification="vulnerable_code_not_present", severity="HIGH"),
            vx("CVE-2024-4003", "pkg:maven/org.example/three", state="not_affected",
               justification="code_not_reachable", severity="medium"),
            vx("CVE-2024-4004", "pkg:maven/org.example/four", state="not_affected",
               justification="code_not_reachable", severity="HIGH"),
        ]},
    ),
)

# ----------------------------------------- documents that link into each other
# ADR-0009 [B]: links resolve among the documents named on the command line. One VEX,
# and the four things a link into an SBOM can meet: the SBOM, a byte-identical copy of
# it, a DIFFERENT document claiming the same serial and version (9 such pairs were
# measured in the public corpora, POC-11), and the SBOM at another version.
LINK_SBOM = bom(
    root=lib("app", ref=r("app"), ctype="application"),
    components=[lib("a"), lib("b")],
    omit_deps=True,
    top_level={"serialNumber": "urn:uuid:22222222-3333-4444-8555-666666666666"},
)

fixture("link-sbom", "The SBOM a linked VEX points into: serial 22222222…, version 1.",
        {"components": 2}, LINK_SBOM)

fixture("link-sbom-copy",
        "A byte-identical copy of link-sbom, as the CISA use cases reuse one product BOM "
        "across cases. Two files, one document: links into it must still resolve.",
        {"components": 2}, json.loads(json.dumps(LINK_SBOM)))

fixture("link-sbom-other",
        "A DIFFERENT document claiming link-sbom's serial and version — 9 such pairs exist "
        "in the public corpora (POC-11), 44 cdxgen files alone sharing the all-zero serial. "
        "Named together with link-sbom, a link into that serial is ambiguous.",
        {"components": 2},
        bom(root=lib("app", ref=r("app"), ctype="application"),
            components=[lib("a"), lib("c")], omit_deps=True,
            top_level={"serialNumber": "urn:uuid:22222222-3333-4444-8555-666666666666"}))

fixture("link-sbom-v2", "link-sbom's serial at version 2: a link asking for version 1 is not it.",
        {"components": 2},
        bom(root=lib("app", ref=r("app"), ctype="application"),
            components=[lib("a"), lib("b")], omit_deps=True,
            top_level={"serialNumber": "urn:uuid:22222222-3333-4444-8555-666666666666", "version": 2}))

fixture(
    "link-vex",
    "A standalone VEX whose every reference is a BOM-Link into link-sbom's serial: two "
    "components that exist there, one that does not, and one link into a document never "
    "supplied, plus one link to the SBOM's own SUBJECT — its metadata.component, which "
    "drives no dependency graph, and is what every CISA use-case link points at. What "
    "each resolves to depends on which documents are named with it.",
    {"components": 0, "vulnerabilities": 5},
    bom(
        root={"bom-ref": "product-vex", "type": "application", "name": "product"},
        omit_components=True,
        omit_deps=True,
        top_level={
            "serialNumber": "urn:uuid:77777777-8888-4999-8aaa-bbbbbbbbbbbb",
            "vulnerabilities": [
                vx("CVE-2024-6001", "urn:cdx:22222222-3333-4444-8555-666666666666/1#" + r("a"), state="exploitable", severity="critical"),
                vx("CVE-2024-6002", "urn:cdx:22222222-3333-4444-8555-666666666666/1#" + r("b"), state="not_affected",
                   justification="code_not_reachable", severity="high"),
                vx("CVE-2024-6003", "urn:cdx:22222222-3333-4444-8555-666666666666/1#" + r("nope"), severity="medium"),
                vx("CVE-2024-6004", "urn:cdx:cccccccc-dddd-4eee-8fff-000000000000/1#x", severity="low"),
                vx("CVE-2024-6005", "urn:cdx:22222222-3333-4444-8555-666666666666/1#" + r("app"), state="exploitable", severity="high"),
            ],
        },
    ),
)

fixture(
    "many-properties",
    "One component carrying 37 properties, the widest osquery category measured "
    "(POC-7: processes and launchd_services). The detail pane cannot show this in "
    "one screen, so it is the fixture that proves the pane scrolls AND says it has "
    "more — a pane cut off with no indicator looks complete.",
    {"max_properties": 37, "unnamed_components": 1},
    bom(
        spec="1.7",
        root=lib("host-os", ref=r("host-os"), ctype="operating-system"),
        components=[
            lib("wide", ref="osquery:processes:data:wide", purl=False, ctype="data",
                props=[{"name": OSQ, "value": "processes"}]
                      + [{"name": f"field_{i:02d}", "value": f"value-{i:02d}"}
                         for i in range(1, 37)]),
            lib("narrow", ref="osquery:alf:data:narrow", purl=False, ctype="data",
                props=[{"name": OSQ, "value": "alf"}]),
            # A component with NO name. Observed in a real HBOM device list, where
            # it rendered as an unselectable-looking blank row; bom-ref is the only
            # field guaranteed to be present.
            lib("", ref="osquery:alf:data:unnamed", purl=False, ctype="data",
                props=[{"name": OSQ, "value": "alf"}]),
        ],
        dependencies=[],
        metadata_extra={"lifecycles": [{"phase": "pre-build"}, {"phase": "operations"}]},
    ),
)

fixture(
    "long-values",
    "Few properties, but values long enough to WRAP. This is the shape that broke "
    "detail scrolling: a row-count estimate of two rows per field undercounts when "
    "the pane wraps, so the pane clamped to no-scroll and claimed everything was "
    "visible. Real OBOM values reach 70-plus characters in a pane around 55 wide.",
    {"max_value_length": 600},
    bom(
        spec="1.7",
        root=lib("host-os", ref=r("host-os"), ctype="operating-system"),
        components=[
            # ONE long property, deliberately. More fields would push the naive
            # two-rows-each estimate over the pane height by itself, and the
            # fixture would stop distinguishing the bug from the fix.
            lib("verbose", ref="osquery:processes:data:verbose", purl=False, ctype="data",
                props=[{"name": OSQ, "value": "processes"},
                       {"name": "cmdline", "value": "/very/long/path/segment/" * 25}]),
        ],
        dependencies=[],
        metadata_extra={"lifecycles": [{"phase": "pre-build"}, {"phase": "operations"}]},
    ),
)

# ---------------------------------------------------------------- error path
fixture(
    "dangling-ref",
    "A dependsOn target that matches no component. Referential integrity was PERFECT "
    "in every real BOM measured, so this is the error path rather than an observed "
    "shape — included because 'never seen it' is not 'cannot happen', and the tool "
    "must report it rather than crash or invent a node.",
    {"dangling_targets": 1},
    bom(
        root=lib("app", ref=r("app")),
        components=[lib("app"), lib("a")],
        dependencies=[dep(r("app"), r("a")), dep(r("a"), r("ghost"))],
    ),
)

# ---------------------------------------------------------------- spec versions
for v in ("1.4", "1.5", "1.7"):
    fixture(
        f"spec-{v.replace('.', '')}",
        f"The same trivial graph at spec {v}. Proves version autodetection and that "
        f"the parser is not pinned to one revision.",
        {"specVersion": v},
        bom(
            spec=v,
            root=lib("app", ref=r("app")),
            components=[lib("app"), lib("a")],
            dependencies=[dep(r("app"), r("a")), dep(r("a"))],
        ),
    )


# ---------------------------------------------------------------- XML encoding
#
# XML is not a second format so much as a second ENCODING: the same model, with
# identity carried by the root element's namespace instead of a `bomFormat` field.
# That difference is the whole reason these fixtures exist — a guard written
# against JSON silently rejects every valid XML BOM, which is exactly what
# happened before syft's corpus surfaced it.
XML_FIXTURES = {
    "xml-simple": (
        "1.6",
        "The ordinary case: a rooted graph in XML rather than JSON.",
        {"root": ("app", "1.0.0"), "components": [("app", "1.0.0"), ("a", "1.0.0")],
         "deps": [("app", ["a"]), ("a", [])]},
    ),
    "xml-no-metadata": (
        "1.1",
        "CycloneDX 1.1, which has NO metadata element at all — metadata.component "
        "arrived in 1.2. A reader that assumes a root component exists breaks here, "
        "and JSON fixtures cannot express this because CycloneDX JSON starts at 1.2.",
        {"root": None, "components": [("a", "1.0.0"), ("b", "2.0.0")], "deps": []},
    ),
}


def xml_doc(spec, spec_data):
    ns = f"http://cyclonedx.org/schema/bom/{spec}"
    out = ['<?xml version="1.0" encoding="UTF-8"?>',
           f'<bom xmlns="{ns}" serialNumber="urn:uuid:3e5d8b2a-30cd-409f-9519-558c53e542c5" version="1">']
    if spec_data["root"]:
        n, v = spec_data["root"]
        out += ['  <metadata>',
                f'    <component type="library" bom-ref="pkg:generic/{n}@{v}">',
                f'      <name>{n}</name>', f'      <version>{v}</version>',
                '    </component>', '  </metadata>']
    out.append('  <components>')
    for n, v in spec_data["components"]:
        out += [f'    <component type="library" bom-ref="pkg:generic/{n}@{v}">',
                f'      <name>{n}</name>', f'      <version>{v}</version>',
                f'      <purl>pkg:generic/{n}@{v}</purl>', '    </component>']
    out.append('  </components>')
    if spec_data["deps"]:
        out.append('  <dependencies>')
        for ref, on in spec_data["deps"]:
            r = f"pkg:generic/{ref}@1.0.0"
            if not on:
                out.append(f'    <dependency ref="{r}"/>')
            else:
                out.append(f'    <dependency ref="{r}">')
                for d in on:
                    out.append(f'      <dependency ref="pkg:generic/{d}@1.0.0"/>')
                out.append('    </dependency>')
        out.append('  </dependencies>')
    out.append('</bom>')
    return "\n".join(out) + "\n"


def main():
    out = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else pathlib.Path(__file__).parent)
    out.mkdir(parents=True, exist_ok=True)
    manifest = {}
    for name, spec in sorted(FIXTURES.items()):
        path = out / f"{name}.cdx.json"
        path.write_text(json.dumps(spec["doc"], indent=2, sort_keys=True) + "\n")
        manifest[name] = {"file": path.name, "why": spec["why"], "asserts": spec["asserts"]}
    for name, (spec, why, data) in sorted(XML_FIXTURES.items()):
        path = out / f"{name}.cdx.xml"
        path.write_text(xml_doc(spec, data))
        manifest[name] = {"file": path.name, "why": why,
                          "asserts": {"specVersion": spec, "encoding": "xml"}}
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
    print(f"{len(manifest)} fixtures + manifest.json -> {out}")


if __name__ == "__main__":
    main()
