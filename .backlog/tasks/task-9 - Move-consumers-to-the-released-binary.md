---
id: TASK-9
title: Move consumers to the released binary
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:19'
updated_date: '2026-10-02 03:12'
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

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Inventory canadian-ham, ArkhamHorror, and dotfiles references to old/local backlog-sync binaries and launchd agents.
2. Update each consumer to use the Homebrew-installed `/opt/homebrew/bin/backlog-sync`; remove the canadian-ham `tools/backlog-sync` copy and point docs to `djensenius/backlog-sync`.
3. Update dotfiles install.sh to install both backlog-md and backlog-sync from the owner tap/Homebrew as appropriate.
4. Install/update consumer launchd agents, run each once and then again as a no-op, and record objective evidence.
5. Open/merge required PRs in affected repositories, then finalize TASK-9 in backlog-sync with links and validation output.
<!-- SECTION:PLAN:END -->
