<!--
SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
SPDX-License-Identifier: Apache-2.0
-->

# Security Policy

## Reporting a vulnerability

**Use GitHub's private vulnerability reporting** — the *Report a vulnerability* button under the
repository's Security tab. It opens a private advisory that only the maintainer can see.

If that is unavailable to you, email **jrjsmrtn@gmail.com** with `bomdive security` in the subject.

Please do not open a public issue for a vulnerability until it is fixed and released.

**What to expect**: an acknowledgement within 7 days, an assessment within 14, and a fix released
as soon as one exists. This is a single-maintainer project with no service-level agreement — those
are intentions, not guarantees. If you have had no reply in 14 days, assume the mail went astray
and try the private advisory route.

Coordinated disclosure is preferred. Tell us the timeline you intend to publish on, and if a fix
is going to take longer than that, we will say so rather than go quiet.

## Supported versions

| Version | Supported |
|---|---|
| 0.1.x | yes — the current line |
| < 0.1 | no |

Pre-1.0: only the latest release gets fixes, and the CLI surface may still move
([ADR-0002](docs/adr/0002-adopt-development-best-practices.md)).

## What is in scope

bomdive **reads files other people produced**, which is where its risk lives:

- **Parser and traversal defects** on a hostile or malformed CycloneDX document: a crash, a hang,
  unbounded memory, or a cycle that never terminates. In scope, and the most valuable class of
  report here.
- **Misrepresentation of a document's content** — showing a dependency relationship, a licence or
  a vulnerability state that the document does not assert, or silently hiding one it does. This is
  a correctness guarantee, not a nicety: a BOM reader that lies is worse than no reader.
- **Anything reached through a document's own fields** — a ref, a purl or a BOM-Link that escapes
  its intended use.

## What is not

- **The documents themselves.** A BOM that understates what it inventories is a problem with its
  generator. bomdive reports coverage so you can see it, and that is the extent of its claim.
- **Vulnerabilities in the components a BOM lists.** This tool shows you the records a document
  carries; it has no database and makes no network call. Use a scanner for that.
- **Dependency advisories** — those go through the normal dependency-update path, not here, unless
  one is reachable from parsing an untrusted document, in which case say so.

## What this tool does not do

No network access, no telemetry, no code execution from a document, no writes to the documents it
reads. If you find it doing any of those, that is a vulnerability by itself.
