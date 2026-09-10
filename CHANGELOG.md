# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Project bootstrapped at tier t1.
- SPARK analysis, with re-runnable evidence for every quantitative claim
  (`docs/inception/`).
- Audience registry (A1–A4), including the coverage-reporting obligation that binds both the
  human and the JSON surface.
- Watched trigger for a cgo-free graph backend via PGlite + Apache AGE, with a self-testing
  checker (`docs/roadmap/watched-pglite-age.md`, `scripts/check-pglite-trigger.py`).
- Foundation ADRs.

### Notes

- Renamed from `lsbom` before the first commit: macOS ships `lsbom(8)` for Installer `.bom`
  files — a collision in the same semantic space. `lsxbom` was verified free in `PATH`, MacPorts,
  `man`, and GitHub repository search on 2026-09-10.

[Unreleased]: https://github.com/jrjsmrtn/lsxbom/compare/v0.1.0...HEAD
