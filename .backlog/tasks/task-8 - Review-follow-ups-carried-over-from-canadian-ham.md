---
id: TASK-8
title: Review follow-ups carried over from canadian-ham
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:19'
updated_date: '2026-10-02 01:16'
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
- [ ] #1 Root backlog.config.yml is honoured only when it contains project_name/projectName (matching Backlog.md v1.53.0), and both backlog_directory and backlogDirectory are accepted (Copilot finding on canadian-ham PR #49)
- [ ] #2 An inbox-labelled issue that already carries a marker only becomes canonical when it's the issue the marker index chose; an inbox-created task whose ID already has a marked issue gets no second marker
- [ ] #3 The push-mode inbox import guards the marker-plus-body size against GitHub's limit and records a per-task failure instead of aborting
- [ ] #4 Expected sub-issue rejections warn once per run with a fixed key; RemoveSubIssue checks the parent repo against the allowlist inside the client
- [ ] #5 Tests cover the remove-error path, a captured `task list --json` fixture, a nested custom folder that is NOT discovered, the main worktree without a Backlog folder, and a line-boundary truncation case
- [ ] #6 A dry run is confirmed not to write FETCH_HEAD or refs, or the README documents the backlog CLI's fetch behaviour
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify the existing review-follow-up coverage against TASK-8 acceptance criteria, focusing on push-mode inbox body-size handling and requested fixtures/edge cases.
2. Add the smallest missing test or logic changes for any uncovered TASK-8 edge case without broadening sync behaviour.
3. Run targeted tests plus go test ./... and go vet ./..., then record evidence in TASK-8 notes and commit the code and Backlog updates.
<!-- SECTION:PLAN:END -->
