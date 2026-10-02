---
id: TASK-7
title: Mirror this repository's own backlog to a GitHub Project
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-02 02:37'
labels:
  - repo
milestone: m-0
dependencies:
  - TASK-1
ordinal: 7000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
backlog-sync should dogfood itself: its own tasks should show on a GitHub Project like its consumers' tasks.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A GitHub Project (v2) for backlog-sync exists with Status options To Do / In Progress / Done
- [ ] #2 `.backlog-sync.json` is committed (defaultRepo djensenius/backlog-sync, inbox mode manual)
- [ ] #3 A launchd agent for this repo is installed with its own label, lock and log, and a real run plus a no-op second run are recorded in the notes
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify/create the djensenius backlog-sync GitHub Project v2 and confirm Status options To Do, In Progress, Done.
2. Commit this repository's `.backlog-sync.json` with defaultRepo `djensenius/backlog-sync`, manual inbox mode, release-safe labels/fields, and a repo-specific lock path.
3. Install a repo-specific launchd agent using the Homebrew-installed `backlog-sync`, with distinct label/log/lock paths.
4. Run a real sync and a second no-op sync, capture evidence in TASK-7 notes, then finalize through PR after checks/review.
<!-- SECTION:PLAN:END -->
