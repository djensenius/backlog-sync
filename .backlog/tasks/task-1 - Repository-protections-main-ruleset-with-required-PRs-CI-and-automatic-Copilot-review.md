---
id: TASK-1
title: >-
  Repository protections: main ruleset with required PRs, CI and automatic
  Copilot review
status: In Progress
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
- [ ] #1 A main-branch ruleset requires a pull request before merging and blocks force pushes and deletion
- [ ] #2 Copilot code review is requested automatically on every PR (ruleset)
- [ ] #3 The CI status check from the CI task is required once it exists
- [ ] #4 The owner can still merge; settings are recorded in the task notes
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify the effective GitHub rulesets for the default branch.
2. Record the active protection settings in TASK-1 notes.
3. Check acceptance criteria only after verifying PR, deletion/force-push, Copilot review, required CI checks, and owner merge behavior.
4. Mark TASK-1 Done via a metadata-only PR.
<!-- SECTION:PLAN:END -->
