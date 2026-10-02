---
id: TASK-9
title: Move consumers to the released binary
status: Done
assignee:
  - '@david'
created_date: '2026-10-01 15:19'
updated_date: '2026-10-02 03:36'
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
- [x] #1 canadian-ham and ArkhamHorror launchd agents run the Homebrew-installed binary, and each records a real run plus a no-op second run
- [x] #2 A canadian-ham PR removes tools/backlog-sync and points its docs at this repository
- [x] #3 The owner's dotfiles install.sh installs backlog-md and backlog-sync
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Inventory canadian-ham, ArkhamHorror, and dotfiles references to old/local backlog-sync binaries and launchd agents.
2. Update each consumer to use the Homebrew-installed `/opt/homebrew/bin/backlog-sync`; remove the canadian-ham `tools/backlog-sync` copy and point docs to `djensenius/backlog-sync`.
3. Update dotfiles install.sh to install both backlog-md and backlog-sync from the owner tap/Homebrew as appropriate.
4. Install/update consumer launchd agents, run each once and then again as a no-op, and record objective evidence.
5. Open/merge required PRs in affected repositories, then finalize TASK-9 in backlog-sync with links and validation output.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Consumer migration completed. Canadian Ham: merged https://github.com/djensenius/canadian-ham/pull/90, removing the historical `tools/backlog-sync` tree and updating README to point at the released `djensenius/backlog-sync` binary from `djensenius/tap/backlog-sync`; `pnpm check:links` passed before PR, and after merge Canadian Ham main was updated and `pnpm check:links` passed again. Reinstalled launchd label `com.djensenius.canadian-ham.backlog-sync` using `/opt/homebrew/bin/backlog-sync`, config `/Users/david/Developer/canadian-ham/.backlog-sync.json`, log `/Users/david/Library/Logs/canadian-ham-backlog-sync.log`; first run completed `sync complete: 0 created, 1 updated, 0 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`; second kickstart completed no-op `sync complete: 0 created, 0 updated, 0 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`. ArkhamHorror: committed `.backlog-sync.json` to `/Users/david/Developer/ArkhamHorror` main and pushed `djensenius/ArkhamHorror-Project` main; installed launchd label `com.djensenius.arkham-backlog-sync` using `/opt/homebrew/bin/backlog-sync`, config `/Users/david/Developer/ArkhamHorror/.backlog-sync.json`, log `/Users/david/Library/Logs/arkham-backlog-sync.log`; owner approved dry-run `sync complete: 65 created, 0 updated, 65 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`; live run completed `sync complete: 65 created, 28 updated, 65 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`; second kickstart completed no-op `sync complete: 0 created, 0 updated, 0 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`. Dotfiles: TASK-2 completed via https://github.com/djensenius/dotfiles/pull/378 (merge commit d0a058c571e9e664e807fc2d819fd24f11a39970); current main includes `pi/agent-stack/install.sh` installing `djensenius/tap/backlog-sync` and runtime tests covering install, skip, and failure cases.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Moved the known consumers to the released `backlog-sync` binary. Canadian Ham removed its historical in-repo syncer copy through PR #90 and now uses launchd with `/opt/homebrew/bin/backlog-sync`; Canadian Ham real and no-op runs completed successfully. ArkhamHorror meta repo now has the generic `.backlog-sync.json`, launchd agent `com.djensenius.arkham-backlog-sync`, and validated dry-run, live, and no-op runs using the Homebrew binary. Dotfiles PR #378 updated the agent-stack installer so Homebrew installs both `backlog-md` and `djensenius/tap/backlog-sync`, with tests covering install/skip/failure behavior.
<!-- SECTION:FINAL_SUMMARY:END -->
