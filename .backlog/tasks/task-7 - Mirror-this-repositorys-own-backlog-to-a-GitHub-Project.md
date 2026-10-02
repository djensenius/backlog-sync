---
id: TASK-7
title: Mirror this repository's own backlog to a GitHub Project
status: Done
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-02 02:49'
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
- [x] #1 A GitHub Project (v2) for backlog-sync exists with Status options To Do / In Progress / Done
- [x] #2 `.backlog-sync.json` is committed (defaultRepo djensenius/backlog-sync, inbox mode manual)
- [x] #3 A launchd agent for this repo is installed with its own label, lock and log, and a real run plus a no-op second run are recorded in the notes
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify/create the djensenius backlog-sync GitHub Project v2 and confirm Status options To Do, In Progress, Done.
2. Commit this repository's `.backlog-sync.json` with defaultRepo `djensenius/backlog-sync`, manual inbox mode, release-safe labels/fields, and a repo-specific lock path.
3. Install a repo-specific launchd agent using the Homebrew-installed `backlog-sync`, with distinct label/log/lock paths.
4. Run a real sync and a second no-op sync, capture evidence in TASK-7 notes, then finalize through PR after checks/review.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Project setup and config validation progress: created GitHub Project v2 `backlog-sync` as djensenius project #11 (`PVT_kwHOAAvwsM4BlZ3m`). Updated its default Status field options to exactly `To Do`, `In Progress`, and `Done` via GraphQL while preserving existing option IDs. Added repo-local `.backlog-sync.json` with `projectOwner: djensenius`, `projectOwnerType: user`, `projectNumber: 11`, `defaultRepo: djensenius/backlog-sync`, and manual inbox mode. Dry-run validation from the TASK-7 worktree using `/opt/homebrew/bin/backlog-sync --config .backlog-sync.json --dry-run --verbose` succeeded and reported: `sync complete: 10 created, 0 updated, 10 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`. Launchd installation and real/no-op runs are deferred until this config lands on the durable main checkout.

Launchd and real/no-op sync acceptance completed from the durable main checkout. Evidence: Project #11 `backlog-sync` exists under owner `djensenius`; `gh project field-list 11 --owner djensenius --format json` shows Status options `To Do`, `In Progress`, and `Done`. `.backlog-sync.json` is committed on main via PR #10 with `defaultRepo: djensenius/backlog-sync`, `projectNumber: 11`, and manual inbox mode. Installed launchd with `./install-launchd.sh --config /Users/david/Developer/backlog-sync/.backlog-sync.json --label com.djensenius.backlog-sync --log /Users/david/Library/Logs/backlog-sync/backlog-sync.log --binary /opt/homebrew/bin/backlog-sync --load`. The rendered plist is `/Users/david/Library/LaunchAgents/com.djensenius.backlog-sync.plist`, label `com.djensenius.backlog-sync`, `WorkingDirectory` `/Users/david/Developer/backlog-sync`, stdout/stderr log `/Users/david/Library/Logs/backlog-sync/backlog-sync.log`, binary `/opt/homebrew/bin/backlog-sync`, and StartInterval 300. The distinct default lock path for this config is `/Users/david/Library/Caches/backlog-sync-e6dfee1ec6021bc1.lock` (hash-derived from the committed config path). First launchd run completed with `sync complete: 10 created, 8 updated, 10 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`; project item summary after the run showed 10 items: 8 Done, 1 In Progress, 1 To Do. A second launchd kickstart completed as a no-op with `sync complete: 0 created, 0 updated, 0 status changes, 0 imported, 0 inbox issues need triage, 0 failed operations`.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Dogfooded backlog-sync on this repository. Created djensenius Project v2 #11 named `backlog-sync`, configured the Status field with `To Do`, `In Progress`, and `Done`, and committed `.backlog-sync.json` targeting `djensenius/backlog-sync` with manual inbox mode. Installed launchd agent `com.djensenius.backlog-sync` using `/opt/homebrew/bin/backlog-sync`, log `/Users/david/Library/Logs/backlog-sync/backlog-sync.log`, and the per-config hash-derived lock. Verified launchd with a real run that created 10 mirrored issues/project items and a second no-op run with zero changes.
<!-- SECTION:FINAL_SUMMARY:END -->
