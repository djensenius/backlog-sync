---
id: TASK-9
title: Move consumers to the released binary
status: To Do
assignee: []
created_date: '2026-10-01 15:19'
labels:
  - release
  - docs
milestone: m-0
dependencies:
  - TASK-5
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Until the first release, consumers run a binary built from canadian-ham's tools/backlog-sync. After the release they should install the tagged build, and the copy in canadian-ham should be removed.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 canadian-ham and ArkhamHorror launchd agents run the Homebrew-installed binary, and each records a real run plus a no-op second run
- [ ] #2 A canadian-ham PR removes tools/backlog-sync and points its docs at this repository
- [ ] #3 The owner's dotfiles install.sh installs backlog-md and backlog-sync
<!-- AC:END -->
