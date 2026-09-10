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
        metadata_extra=None):
    doc = {
        "bomFormat": "CycloneDX",
        "specVersion": spec,
        "version": 1,
        "metadata": {"component": root} if root else {},
        "components": components or [],
    }
    if metadata_extra:
        doc["metadata"].update(metadata_extra)
    if not omit_deps:
        doc["dependencies"] = dependencies or []
    return doc


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
