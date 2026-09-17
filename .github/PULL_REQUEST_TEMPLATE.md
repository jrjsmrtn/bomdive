<!--
SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
SPDX-License-Identifier: Apache-2.0
-->

## What this changes

<!-- The behaviour, not the diff. Link the issue if there is one. -->

## Why

<!-- What forced the shape: the alternative you rejected, the constraint, what you tried that
     failed. This is the part that cannot be recovered from the tree later. -->

## Checklist

- [ ] `git commit -s` — every commit signed off ([DCO](https://developercertificate.org/));
      `git rebase --signoff <base>` fixes a branch
- [ ] `gofmt -l .` prints nothing, `go vet ./...` and `go test ./...` pass
- [ ] `./scripts/check-coverage.sh` — new code covered, every package still at 80% or above
- [ ] `reuse lint` passes (SPDX header on any new file)
- [ ] A test that fails without this change
- [ ] `CHANGELOG.md` updated under `[Unreleased]`
- [ ] Fixtures changed through `testdata/generate.py`, not by hand
- [ ] The commit messages claim no more than the commits contain
