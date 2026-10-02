---
id: TASK-8
title: Review follow-ups carried over from canadian-ham
status: Done
assignee:
  - '@david'
created_date: '2026-10-01 15:19'
updated_date: '2026-10-02 01:23'
labels:
  - bug
  - hardening
milestone: m-1
dependencies: []
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Code reviews of the syncer in djensenius/canadian-ham (task-16 to task-19 and PR #49) approved it with non-blocking notes that were deferred. Most are rare edge cases, but some could produce a duplicate marked issue or a mismatch with the backlog CLI once more consumers use adoption and inbox imports.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Root backlog.config.yml is honoured only when it contains project_name/projectName (matching Backlog.md v1.53.0), and both backlog_directory and backlogDirectory are accepted (Copilot finding on canadian-ham PR #49)
- [x] #2 An inbox-labelled issue that already carries a marker only becomes canonical when it's the issue the marker index chose; an inbox-created task whose ID already has a marked issue gets no second marker
- [x] #3 The push-mode inbox import guards the marker-plus-body size against GitHub's limit and records a per-task failure instead of aborting
- [x] #4 Expected sub-issue rejections warn once per run with a fixed key; RemoveSubIssue checks the parent repo against the allowlist inside the client
- [x] #5 Tests cover the remove-error path, a captured `task list --json` fixture, a nested custom folder that is NOT discovered, the main worktree without a Backlog folder, and a line-boundary truncation case
- [x] #6 A dry run is confirmed not to write FETCH_HEAD or refs, or the README documents the backlog CLI's fetch behaviour
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify the existing review-follow-up coverage against TASK-8 acceptance criteria, focusing on push-mode inbox body-size handling and requested fixtures/edge cases.
2. Add the smallest missing test or logic changes for any uncovered TASK-8 edge case without broadening sync behaviour.
3. Run targeted tests plus go test ./... and go vet ./..., then record evidence in TASK-8 notes and commit the code and Backlog updates.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented review follow-ups and verified with targeted tests plus full validation. Evidence: root config gating/camelCase tests, existing duplicate-marker/inbox canonicalization tests, new push-mode oversized marker+body failure-and-continue test, sub-issue remove-error/fixed-warning/RemoveSubIssue allowlist tests, captured task-list fixture plus nested/no-Backlog/line-boundary tests, and README dry-run FETCH_HEAD/refs documentation. Commands passed: targeted go test -run ..., go test ./..., go vet ./....
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Hardened TASK-8 review follow-ups: root Backlog config discovery now requires project_name/projectName and accepts backlogDirectory; push-mode inbox marker+body oversize is recorded as a per-task failure while later imports continue; sub-issue warning/remove allowlist handling is covered; required Backlog CLI fixture/discovery/truncation coverage is present; README documents dry-run not writing FETCH_HEAD or refs. Verified with targeted go test, go test ./..., and go vet ./....
<!-- SECTION:FINAL_SUMMARY:END -->
