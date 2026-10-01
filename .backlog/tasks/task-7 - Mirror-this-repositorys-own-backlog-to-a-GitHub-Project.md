---
id: TASK-7
title: Mirror this repository's own backlog to a GitHub Project
status: To Do
assignee: []
created_date: '2026-10-01 15:18'
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
