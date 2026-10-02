---
id: TASK-1
title: >-
  Repository protections: main ruleset with required PRs, CI and automatic
  Copilot review
status: Done
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-02 02:33'
labels:
  - ci
  - repo
milestone: m-0
dependencies: []
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The project follows a pull-request-only workflow (see AGENTS.md): nothing may reach main without a PR, Copilot code review and green CI. The repository was bootstrapped with a direct push, so the protections still need to be switched on once CI exists.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A main-branch ruleset requires a pull request before merging and blocks force pushes and deletion
- [x] #2 Copilot code review is requested automatically on every PR (ruleset)
- [x] #3 The CI status check from the CI task is required once it exists
- [x] #4 The owner can still merge; settings are recorded in the task notes
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify the effective GitHub rulesets for the default branch.
2. Record the active protection settings in TASK-1 notes.
3. Check acceptance criteria only after verifying PR, deletion/force-push, Copilot review, required CI checks, and owner merge behavior.
4. Mark TASK-1 Done via a metadata-only PR.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Verified GitHub repository rulesets after owner configuration. Authoritative ruleset is `Main protection` (ruleset id 24344547), active on `~DEFAULT_BRANCH` with no bypass actors. Evidence from `gh api repos/djensenius/backlog-sync/rulesets/24344547`: rules include `deletion`, `non_fast_forward`, `pull_request`, `copilot_code_review`, and `required_status_checks`. Pull request parameters require PRs with `required_approving_review_count: 0`, `required_review_thread_resolution: true`, no code owner/last-push approval requirement, and allowed merge methods merge/squash/rebase. Copilot parameters are `review_on_push: true` and `review_draft_pull_requests: false`. Required status checks are strict (`strict_required_status_checks_policy: true`) and require `CI (ubuntu-latest)`, `CI (macos-latest)`, and `Analyze Go`. The owner can still merge through the normal PR flow after required checks pass and review threads are resolved; there are no bypass actors. A prior active ruleset `main: PRs, Copilot review` remains in place as redundant/additive protection, but `Main protection` contains all TASK-1 requirements.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Configured and verified repository protection for the default branch. The active `Main protection` ruleset blocks deletion and force pushes, requires pull requests, requests Copilot code review on push, requires review thread resolution, and requires strict status checks for `CI (ubuntu-latest)`, `CI (macos-latest)`, and `Analyze Go`. No bypass actors are configured, so the owner can merge through the normal protected PR flow once checks pass and review threads are resolved. A redundant older `main: PRs, Copilot review` ruleset remains active but does not weaken the stricter protection.
<!-- SECTION:FINAL_SUMMARY:END -->
