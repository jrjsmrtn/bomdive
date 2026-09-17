# 10. Rename the tool to bomdive

Date: 2026-09-17

## Status

Accepted on 2026-09-17 by the maintainer, who chose the name. Supersedes the naming note in
`docs/inception/spark-analysis.md`, which is kept as the record of what was checked on 2026-09-10.

## Context

The project was `lsbom` for a few hours, then `lsxbom`
([the naming note](../inception/spark-analysis.md)): macOS ships `lsbom(8)` for Installer `.bom`
files, so the first name collided with a system command in the same semantic space, and the `x`
came from the landscape bundle's rule that *xBOM is not a format — it is the placeholder, written
with a literal `x`*.

`lsxbom` is accurate and awkward. It is hard to say, and the `x` needs the explanation above every
time. Two candidates were measured on 2026-09-17 before the name was settled.

**`xbom` is not available.** `safedep/xbom` (Apache-2.0, 37 stars, releases to v0.0.3, last pushed
2026-01-22) installs the `xbom` command — `brew install safedep/tap/xbom` — and is used as
`xbom generate --dir … --bom …`. It is a **generator**, which `CLAUDE.md` says this tool is
deliberately not, so sharing its command name would put two opposite tools on one word in the same
file format. The GitHub user `xbom` has also been taken since 2017. `xBOM` is in any case the
industry's generic term for the family, which makes it weak as an identity and hard to search for.

**`bill` is not available either**, and is worse as a name: taken on npm (3.2.6, unrelated),
PyPI and crates.io (0.4.3, an invoicing library), matched by 188,073 GitHub repository names, a
common human first name, and one letter from `bom`, the `kubernetes-sigs/bom` command (466 stars).

**`bomdive` is free everywhere checked**, on 2026-09-17:

| Namespace | State |
|---|---|
| GitHub repositories matching the name | 0 |
| GitHub user or organisation `bomdive` | free |
| Codeberg user | free |
| npm, PyPI, crates.io | free |
| Homebrew core formula, MacPorts port | free |
| a command in `PATH` on the development machine | none |

⚠ **Not checked, and stated so rather than implied:** trademarks; forges other than GitHub and
Codeberg; domain registration — `bomdive.dev`, `.io` and `.com` resolve to no address, which is not
the same as being unregistered.

## Decision

**The tool is `bomdive`.** The Go module is `github.com/jrjsmrtn/bomdive`, the command is
`bomdive`, and `ls`, `tree` and `browse` remain its subcommands.

The name borrows the recognition of `wagoodman/dive` (MIT, 54,570 stars), the Docker image layer
explorer: "dive" already reads as *explore this interactively in a terminal*, which is what
[ADR-0006](0006-column-view-as-a-first-class-renderer.md) made the design centre.

**Two costs, accepted knowingly:**

- **The name advertises the interactive half only.** `ls` is the universal command
  (`CLAUDE.md`), with a scriptable `--output json` surface, and "dive" says nothing about it.
  Keeping `bomdive ls` and `bomdive tree` as subcommands puts that half in every example. The
  alternative shortlisted name, `cdxls`, had the mirror flaw: it hides the browser.
- **`bom` is broader than what the tool reads.** It suggests any bill of materials, including
  SPDX, while [ADR-0003](0003-go-with-cyclonedx-go.md) makes CycloneDX the only input. A `cdx*`
  name could not overclaim that way. The gain in recognition was judged worth it.

## Consequences

- The module path changed, so every import changed with it. **No git remote is configured**, so
  there was no repository to rename on any forge or on the private server.
- **Every historical record was rewritten to the new name** — ADRs, evidence records, the
  CHANGELOG, the SPARK analysis. Two passages were deliberately left naming `lsxbom`, because they
  record *which name was checked on which date*: the CHANGELOG's naming note and the SPARK
  analysis' availability finding. The tool was called `lsxbom` from 2026-09-10 to 2026-09-17.
- The working directory, the installed binary and the tmux session name are outside the
  repository's control and are the maintainer's to move.
- A third rename would cost more than this one: it is cheap now because nothing is published, no
  remote exists, and there are no users. This is the last free move.
