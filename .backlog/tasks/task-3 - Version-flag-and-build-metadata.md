---
id: TASK-3
title: Version flag and build metadata
status: To Do
assignee: []
created_date: '2026-10-01 15:18'
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
