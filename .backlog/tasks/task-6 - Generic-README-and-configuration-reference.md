---
id: TASK-6
title: Generic README and configuration reference
status: To Do
assignee: []
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 16:03'
labels:
  - docs
milestone: m-0
dependencies:
  - TASK-4
  - TASK-10
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The README was written while the tool lived inside canadian-ham and still uses that repo's paths. New consumers (e.g. ArkhamHorror and the owner's other projects) need a standalone guide.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The README covers installation (Homebrew, go install, release archives), GitHub prerequisites (gh scopes incl. project, creating the Project and Status options), the full config reference with a single-repo and a multi-repo example, inbox modes, dry runs, launchd per config, and the cross-branch behaviour of the backlog CLI
- [ ] #2 No consumer-specific absolute paths remain except in clearly labelled examples
- [ ] #3 Markdown links resolve
<!-- AC:END -->
