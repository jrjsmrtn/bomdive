#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
#
# SPDX-License-Identifier: Apache-2.0

"""Could a VEX be navigated in the column view, and would its links lead anywhere?

WHY. bomdive navigates components. Before deciding to browse vulnerability records
(ADR-0009), this measures whether VEX documents have the shape the column view needs —
something to group by, something to descend into — and whether their `affects[].ref`
references resolve.

Two shapes are measured separately, because they turned out to be different worlds:

  STANDALONE VEX  vulnerabilities and NO components. Every sample in the public corpora is
                  this shape, and every one is an illustrative use case.
  EMBEDDED VEX    vulnerabilities AND components in one document. The public corpora hold
                  none; a Dependency-Track 5.1.0 VEX export is this shape.

For each: vulnerabilities per document, analysis states, severities, how references
resolve (a LOCAL ref must name a bom-ref in the same document; a BOM-LINK
`urn:cdx:<serial>/<version>#<bom-ref>` must name a corpus document at that version holding
that bom-ref), and, for embedded VEX, whether the components are an inventory or only the
components some vulnerability affects.

A BY-SOURCE section then reports, per corpus sub-directory: the generator named in
`metadata.tools`, reference forms (bom-ref, purl, BOM-Link), values outside the schema's
own enums, and which first column each document gets under ADR-0009 — both the rule first
accepted (group by analysis state whenever any vulnerability carries one) and the amended
rule (a column must split: state if that gives two or more groups, else severity if THAT
does, else state). A first column holding one group is not an axis.

Counts only, never identifiers, so it is safe over a private corpus.

    python3 docs/inception/evidence/poc11-vex-navigability.py <corpus-dir>

With --links DIR [DIR ...] it instead measures what a BOM-Link RESOLVES AGAINST: how
many (serialNumber, version) pairs more than one document claims, across every corpus
named, and whether those documents are byte-identical copies or different content.
A BOM-Link identifies its target by serial and version alone, so a pair claimed by two
different documents cannot be resolved without guessing.

    python3 ... --links .corpora-cache .corpora-cache-vex

With --directory DIR it measures what `bomdive browse DIR` would resolve against (ADR-0009
[D]): the files directly in DIR as ONE set, and how many BOM-Links would read *linked,
ambiguous* because documents with different content claim their target.

    python3 ... --directory .corpora-cache/examples

<corpus-dir> holds one sub-directory per source, each holding *.json documents.
"""
import collections
import glob
import json
import os
import pathlib
import sys

SEVERITY_ORDER = ["critical", "high", "medium", "low", "info", "none", "unknown"]


def refs_in(doc):
    """Every bom-ref a document defines: its root, components and services, nested."""
    out = set()
    root = (doc.get("metadata") or {}).get("component") or {}
    if root.get("bom-ref"):
        out.add(root["bom-ref"])
    stack = list(doc.get("components") or []) + list(doc.get("services") or [])
    while stack:
        x = stack.pop()
        if x.get("bom-ref"):
            out.add(x["bom-ref"])
        stack += (x.get("components") or []) + (x.get("services") or [])
    return out


def top_severity(vuln):
    """The most severe rating a vulnerability carries, or "(no rating)"."""
    sev = [r.get("severity") for r in vuln.get("ratings") or [] if r.get("severity")]
    if not sev:
        return "(no rating)"
    return min(sev, key=lambda s: SEVERITY_ORDER.index(s) if s in SEVERITY_ORDER else 99)


def measure(docs, by_serial):
    """Shape of a set of documents that all carry vulnerabilities."""
    per_doc, per_vuln = [], []
    states, severities, roots = collections.Counter(), collections.Counter(), collections.Counter()
    local, link = collections.Counter(), collections.Counter()
    comps_total = comps_affected = docs_only_affected = with_deps = 0
    max_vulns_per_component = 0
    for d in docs:
        own = refs_in(d)
        vulns = d["vulnerabilities"]
        per_doc.append(len(vulns))
        roots[((d.get("metadata") or {}).get("component") or {}).get("type", "(no root)")] += 1
        with_deps += bool(d.get("dependencies"))
        targets = collections.Counter()
        for v in vulns:
            states[(v.get("analysis") or {}).get("state", "(no analysis)")] += 1
            severities[top_severity(v)] += 1
            affects = v.get("affects") or []
            per_vuln.append(len(affects))
            for a in affects:
                ref = a.get("ref", "")
                targets[ref] += 1
                if not ref.startswith("urn:cdx:"):
                    # A purl that names no bom-ref here identifies a PACKAGE, not a missing
                    # component: ADR-0009 section 2 gives it its own state rather than
                    # counting it as dangling.
                    if ref in own:
                        local["resolves"] += 1
                    elif ref.startswith("pkg:"):
                        local["purl — names a package, not a component here"] += 1
                    else:
                        local["names nothing"] += 1
                    continue
                body, _, fragment = ref[len("urn:cdx:"):].partition("#")
                serial, _, version = body.partition("/")
                candidates = by_serial.get(serial.lower(), [])
                if not candidates:
                    link["target document not in corpus"] += 1
                    continue
                exact = [p for v_, p in candidates if v_ == version]
                if not exact:
                    link["serial found, version differs"] += 1
                elif fragment in refs_in(exact[0]):
                    link["resolves"] += 1
                else:
                    link["target found, bom-ref missing"] += 1
        comps = {c.get("bom-ref") for c in d.get("components") or []}
        comps_total += len(comps)
        comps_affected += len(comps & set(targets))
        docs_only_affected += bool(comps) and comps <= set(targets)
        if targets:
            max_vulns_per_component = max(max_vulns_per_component, max(targets.values()))
    return {
        "documents": len(docs),
        "vulnerabilities per document": f"{min(per_doc)}–{max(per_doc)}" if per_doc else "-",
        "vulnerabilities, total": sum(per_doc),
        "affects[] per vulnerability": f"{min(per_vuln)}–{max(per_vuln)}" if per_vuln else "-",
        "most vulnerabilities on one target": max_vulns_per_component,
        "analysis.state": dict(states.most_common()),
        "most severe rating": dict(severities.most_common()),
        "root component type": dict(roots.most_common()),
        "documents with a `dependencies` graph": with_deps,
        "components, total": comps_total,
        "components some vulnerability affects": comps_affected,
        "documents whose components are ONLY the affected ones": docs_only_affected,
        "non-BOM-Link refs": dict(local),
        "BOM-Link refs": dict(link),
    }


def schema_enums():
    """The schema's own enums, read from the repository's schema cache — never typed in."""
    here = pathlib.Path(__file__).resolve()
    path = here.parents[3] / "testdata" / ".schema-cache" / "bom-1.6.schema.json"
    if not path.exists():
        return None
    d = json.load(open(path))["definitions"]
    return {
        "state": set(d["impactAnalysisState"]["enum"]),
        "justification": set(d["impactAnalysisJustification"]["enum"]),
        "severity": set(d["severity"]["enum"]),
    }


def groups(vulns, axis):
    """The first-column groups a document yields on one axis. A severity spelled in another
    case is grouped with the schema value it names; an unknown value is its own group."""
    out = set()
    for v in vulns:
        if axis == "state":
            out.add((v.get("analysis") or {}).get("state", "(no analysis)"))
        else:
            sev = top_severity(v)
            out.add(sev.lower() if sev.lower() in SEVERITY_ORDER else sev)
    return out


def first_column(vulns):
    """(accepted rule, amended rule) for one document, as "axis:groups"."""
    by_state, by_sev = groups(vulns, "state"), groups(vulns, "severity")
    has_analysis = any(v.get("analysis", {}).get("state") for v in vulns)
    accepted = f"state:{len(by_state)}" if has_analysis else f"severity:{len(by_sev)}"
    if len(by_state) >= 2:
        amended = f"state:{len(by_state)}"
    elif len(by_sev) >= 2:
        amended = f"severity:{len(by_sev)}"
    else:
        amended = f"state:{len(by_state)}"
    return accepted, amended


def by_source(root):
    enums = schema_enums()
    totals = collections.Counter()
    shared_ids = {}
    print("\n== BY SOURCE")
    for src in sorted(os.listdir(root)):
        docs = []
        for path in sorted(glob.glob(os.path.join(root, src, "*.json"))):
            try:
                d = json.load(open(path))
            except Exception:
                continue
            if isinstance(d, dict) and d.get("bomFormat") == "CycloneDX" and d.get("vulnerabilities"):
                docs.append(d)
        if not docs:
            continue
        tools, forms, bad = collections.Counter(), collections.Counter(), collections.Counter()
        axis_acc, axis_amd = collections.Counter(), collections.Counter()
        shapes = collections.Counter()
        for d in docs:
            shapes["embedded" if d.get("components") else "standalone"] += 1
            t = (d.get("metadata") or {}).get("tools")
            if isinstance(t, dict):
                t = (t.get("components") or []) + (t.get("services") or [])
            for x in t or []:
                tools[f"{x.get('name')} {x.get('version') or ''}".strip()] += 1
            if not t:
                tools["(none recorded)"] += 1
            vulns = d["vulnerabilities"]
            for v in vulns:
                for a in v.get("affects") or []:
                    r = a.get("ref", "")
                    forms["purl" if r.startswith("pkg:") else "BOM-Link" if r.startswith("urn:cdx:")
                          else "bom-ref"] += 1
                if enums:
                    an = v.get("analysis") or {}
                    if an.get("state") and an["state"] not in enums["state"]:
                        bad[f"state={an['state']}"] += 1
                    if an.get("justification") and an["justification"] not in enums["justification"]:
                        bad[f"justification={an['justification']}"] += 1
                    for r in v.get("ratings") or []:
                        if r.get("severity") and r["severity"] not in enums["severity"]:
                            bad[f"severity={r['severity']}"] += 1
            # Records sharing an id with another record in the SAME document: a column
            # that shows only the id draws them as identical rows.
            ids = collections.Counter(v.get("id") or "" for v in vulns)
            dup = sum(n for i, n in ids.items() if n > 1)
            totals["records sharing their id in-document"] += dup
            shared_ids[src] = shared_ids.get(src, 0) + dup
            acc, amd = first_column(vulns)
            axis_acc[acc] += 1
            axis_amd[amd] += 1
            totals["documents with vulnerabilities"] += 1
            totals["vulnerability records"] += len(vulns)
            totals["accepted rule: one group"] += acc.endswith(":1")
            totals["amended rule: splits"] += int(amd.split(":")[1]) >= 2
            totals["amended rule: splits on neither axis"] += int(amd.split(":")[1]) < 2
        one_group = sum(n for k, n in axis_acc.items() if k.endswith(":1"))
        print(f"  {src}")
        print(f"    documents with vulnerabilities : {len(docs)}  {dict(shapes)}")
        print(f"    vulnerabilities                : {sum(len(d['vulnerabilities']) for d in docs)}")
        print(f"    generator (metadata.tools)     : {dict(tools.most_common())}")
        print(f"    affects[].ref forms            : {dict(forms)}")
        print(f"    out-of-schema values           : {dict(bad.most_common()) or 'none'}")
        print(f"    first column, accepted rule    : {dict(axis_acc.most_common())}  "
              f"(one group — no axis — in {one_group} of {len(docs)})")
        print(f"    first column, amended rule     : {dict(axis_amd.most_common())}")
        print(f"    records sharing their id       : {shared_ids.get(src, 0)} of "
              f"{sum(len(d['vulnerabilities']) for d in docs)} (with another record in the same document)")
    if totals:
        print("\n== ACROSS ALL SOURCES")
        for k, v in totals.items():
            print(f"  {k:<40s}: {v}")


def main(root: str) -> int:
    docs, unreadable = [], 0
    for path in sorted(glob.glob(os.path.join(root, "*", "*.json"))):
        try:
            d = json.load(open(path))
        except Exception:
            unreadable += 1
            continue
        if isinstance(d, dict) and d.get("bomFormat") == "CycloneDX":
            docs.append(d)

    by_serial = collections.defaultdict(list)  # uuid -> [(version, doc)]
    for d in docs:
        s = d.get("serialNumber") or ""
        if s.startswith("urn:uuid:"):
            by_serial[s[len("urn:uuid:"):].lower()].append((str(d.get("version")), d))

    standalone = [d for d in docs if not d.get("components") and d.get("vulnerabilities")]
    embedded = [d for d in docs if d.get("components") and d.get("vulnerabilities")]

    print(f"CycloneDX JSON documents : {len(docs)}   (unreadable files skipped: {unreadable})")
    for title, group in (("STANDALONE VEX", standalone), ("EMBEDDED VEX", embedded)):
        print(f"\n== {title}: {len(group)} document(s)")
        if not group:
            continue
        for key, value in measure(group, by_serial).items():
            if key == "documents":
                continue
            if title == "STANDALONE VEX" and key.startswith(("components", "documents whose")):
                continue
            print(f"  {key:<55s}: {value}")
    by_source(root)
    return 0


def links(roots):
    import hashlib
    claims = collections.defaultdict(list)  # (serial, version) -> [sha256]
    with_serial = 0
    for root in roots:
        for path in sorted(glob.glob(os.path.join(root, "*", "*.json"))):
            try:
                raw = open(path, "rb").read()
                d = json.loads(raw)
            except Exception:
                continue
            if not (isinstance(d, dict) and d.get("bomFormat") == "CycloneDX" and d.get("serialNumber")):
                continue
            with_serial += 1
            key = (d["serialNumber"].removeprefix("urn:uuid:").lower(), str(d.get("version")))
            claims[key].append(hashlib.sha256(raw).hexdigest())
    shared = {k: v for k, v in claims.items() if len(v) > 1}
    differ = {k: v for k, v in shared.items() if len(set(v)) > 1}
    print(f"CycloneDX JSON documents with a serialNumber : {with_serial}")
    print(f"(serial, version) pairs claimed by more than one document: {len(shared)}")
    print(f"  byte-identical copies                      : {len(shared) - len(differ)}")
    print(f"  DIFFERENT content under one serial/version : {len(differ)}")
    print(f"  largest: {max((len(v) for v in differ.values()), default=0)} documents under one pair")
    return 0


def xml_root(raw):
    """(namespace-qualified tag, attributes) of an XML document's root element, or None.

    Reads with expat and stops AT the root element: nothing past it is parsed. Any DOCTYPE is
    refused outright — CycloneDX needs none, and with no DTD there are no entities, so neither
    external-entity nor entity-expansion attacks have anything to work with. The corpora are
    fetched from the internet, so this is untrusted input. Stdlib ElementTree parses the whole
    document and accepts internal DTDs, which is why it is not used here.
    """
    from xml.parsers import expat

    class Reached(Exception):
        pass

    root = {}

    def refuse_dtd(*_):
        raise ValueError("DTD refused")

    def start(tag, attrs):
        root["tag"], root["attrs"] = tag, attrs
        raise Reached

    p = expat.ParserCreate(namespace_separator=" ")
    p.StartDoctypeDeclHandler = refuse_dtd
    p.StartElementHandler = start
    try:
        p.Parse(raw, True)
    except Reached:
        return root["tag"], root["attrs"]
    except (expat.ExpatError, ValueError):
        return None
    return None


def directory(root):
    """What `bomdive browse DIR` would resolve against: the files DIRECTLY in DIR, as one set.

    Named files are a set the user asserted belongs together; a directory is whatever happens
    to be in it. This measures what that costs: how many BOM-Links land on a (serial, version)
    pair that documents with DIFFERENT content claim, and so read *linked, ambiguous*.

    The serial is compared without its `urn:uuid:` prefix, because a BOM-Link carries the bare
    UUID. A first ad-hoc count compared them with the prefix on one side, matched nothing, and
    reported 0 ambiguous links; the count of links whose target is present at all is printed
    as the control that exposes that mistake.

    CycloneDX XML counts as a member, because bomdive's Load reads it: a first version parsed
    JSON only and counted testdata's two XML fixtures as rejected. An XML document's serial
    and version are read from its root element, so it can be a link TARGET; links are read
    from JSON documents only, since no XML VEX has been measured.
    """
    import hashlib
    docs, rejected = [], 0
    for path in sorted(glob.glob(os.path.join(root, "*"))):
        if not os.path.isfile(path):
            continue
        raw = open(path, "rb").read()
        digest = hashlib.sha256(raw).hexdigest()
        if raw.lstrip(b"\xef\xbb\xbf \t\r\n").startswith(b"<"):
            found = xml_root(raw)
            if not found or not found[0].startswith("http://cyclonedx.org/schema/bom/"):
                rejected += 1
                continue
            docs.append((digest, {"serialNumber": found[1].get("serialNumber"),
                                  "version": found[1].get("version", "1")}))
            continue
        try:
            d = json.loads(raw)
        except Exception:
            rejected += 1
            continue
        if not (isinstance(d, dict) and d.get("bomFormat") == "CycloneDX"):
            rejected += 1
            continue
        docs.append((digest, d))
    claims = collections.defaultdict(set)  # (serial, version) -> {sha256}
    for digest, d in docs:
        if d.get("serialNumber"):
            key = (d["serialNumber"].removeprefix("urn:uuid:").lower(), str(d.get("version")))
            claims[key].add(digest)
    outcome, per_doc = collections.Counter(), []
    for _, d in docs:
        ambiguous = 0
        for v in d.get("vulnerabilities") or []:
            for a in v.get("affects") or []:
                ref = a.get("ref", "")
                if not ref.startswith("urn:cdx:"):
                    continue
                serial, _, version = ref[len("urn:cdx:"):].partition("#")[0].partition("/")
                found = claims.get((serial.lower(), version), set())
                outcome["BOM-Link refs"] += 1
                if not found:
                    outcome["no document at that serial and version"] += 1
                elif len(found) > 1:
                    outcome["AMBIGUOUS — claimed by documents with different content"] += 1
                    ambiguous += 1
                else:
                    outcome["target present, one content"] += 1
        if ambiguous:
            per_doc.append(ambiguous)
    shared = sum(1 for c in claims.values() if len(c) > 1)
    print(f"files directly in the directory : {len(docs) + rejected}")
    print(f"  CycloneDX, would load         : {len(docs)}")
    print(f"  rejected (not JSON/CycloneDX) : {rejected}")
    print(f"(serial, version) pairs with different content under them: {shared}")
    for k in ("BOM-Link refs", "target present, one content",
              "AMBIGUOUS — claimed by documents with different content",
              "no document at that serial and version"):
        print(f"  {k:<58s}: {outcome[k]}")
    print(f"documents with an ambiguous link: {len(per_doc)}; most from one document: "
          f"{max(per_doc, default=0)}")
    return 0


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--links":
        sys.exit(links(sys.argv[2:]))
    if len(sys.argv) > 2 and sys.argv[1] == "--directory":
        sys.exit(directory(sys.argv[2]))
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else ".corpora-cache"))
