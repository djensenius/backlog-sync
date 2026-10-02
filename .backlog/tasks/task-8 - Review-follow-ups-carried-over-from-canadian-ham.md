---
id: TASK-8
title: Review follow-ups carried over from canadian-ham
status: Done
assignee:
  - '@david'
created_date: '2026-10-01 15:19'
updated_date: '2026-10-02 01:37'
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
Review-fix pass for reviewer REQUEST CHANGES ca045b71-7d43-42bd-99d2-7f53bb0f9e90.
1. Fix push-mode marked-inbox duplicate-marker loser handling so loser issues are stripped/warned but never made canonical.
2. Guard inbox-import marker collision when a newly-created task ID already has a different marked issue; skip that inbox issue safely without adding a second marker.
3. Add targeted tests for both blocking duplicate-marker cases.
4. Update README dry-run/backlog CLI wording for remote_operations instead of claiming unproven FETCH_HEAD/ref guarantees.
5. Run targeted tests, go test ./..., and go vet ./..., then update TASK-8 evidence before finalizing.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented review follow-ups and verified with targeted tests plus full validation. Evidence: root config gating/camelCase tests, existing duplicate-marker/inbox canonicalization tests, new push-mode oversized marker+body failure-and-continue test, sub-issue remove-error/fixed-warning/RemoveSubIssue allowlist tests, captured task-list fixture plus nested/no-Backlog/line-boundary tests, and README dry-run FETCH_HEAD/refs documentation. Commands passed: targeted go test -run ..., go test ./..., go vet ./....

Reviewer REQUEST CHANGES ca045b71-7d43-42bd-99d2-7f53bb0f9e90 reopened acceptance criteria #2 and #6. Criteria #2 and #6 are unchecked for this review-fix pass and the prior final summary was cleared until the duplicate-marker fixes and README/evidence correction are revalidated.

Review-fix validation complete. AC #2 is now covered by TestInboxMarkedDuplicateLoserOnlyStripsInboxLabel (duplicate marker loser only loses the inbox label and the marker-index winner remains canonical) and TestInboxCreatedTaskIDCollisionLeavesIssueUnmarkedAndContinues (newly-created ID collision leaves the inbox issue unmarked, records one per-task failure, and continues to the next inbox issue). AC #5 also gained ExecBacklog.ListTasks coverage through scriptRunner for the captured task-list fixture. AC #6 is satisfied by rewriting README dry-run wording to avoid an unproven FETCH_HEAD/ref guarantee and to document that Backlog CLI read operations may touch remotes when remote_operations is enabled. Validation output: targeted go test ./... -run "TestInbox(MarkedDuplicateLoserOnlyStripsInboxLabel|CreatedTaskIDCollisionLeavesIssueUnmarkedAndContinues)|TestBacklogJSONFixtureFieldNames" -count=1 => ok github.com/djensenius/backlog-sync 0.133s; go test ./... => ok github.com/djensenius/backlog-sync 1.040s; go vet ./... => no output.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Resolved reviewer REQUEST CHANGES ca045b71-7d43-42bd-99d2-7f53bb0f9e90 by hardening push-mode inbox marker collision handling. Marked duplicate-marker inbox losers now only have the inbox label stripped and never replace the marker-index winner; newly-created task IDs that collide with an existing marked issue leave the inbox issue unmarked, record a per-task failure, and continue. README dry-run wording now documents Backlog CLI remote_operations caveats instead of claiming an unproven FETCH_HEAD/ref guarantee. Added targeted tests for both blocker cases and routed the task-list fixture through ExecBacklog.ListTasks. Verified with targeted go test, go test ./..., and go vet ./....
<!-- SECTION:FINAL_SUMMARY:END -->
