---
id: TASK-3
title: Version flag and build metadata
status: In Progress
assignee:
  - '@pi'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 16:04'
labels:
  - cli
milestone: m-0
dependencies: []
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Release archives and Homebrew need `backlog-sync --version` to report what's installed, and GoReleaser injects version, commit and date at build time like gopod.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `backlog-sync --version` prints version, commit and date (defaults for local builds)
- [ ] #2 main.version, main.commit and main.date can be set with -ldflags -X
- [ ] #3 A test covers the version output
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add package-level version, commit, and date variables in main with local-build defaults suitable for -ldflags -X injection.
2. Extend CLI parsing so --version prints the three fields and exits before sync setup or config validation.
3. Add/adjust tests to exercise default version output and ldflags-settable variables without network or shelling to GitHub.
4. Run go test ./... and go vet ./..., then report exact output.
<!-- SECTION:PLAN:END -->
