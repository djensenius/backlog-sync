---
id: TASK-5
title: 'Homebrew formula in djensenius/homebrew-tap, updated by pull request'
status: To Do
assignee:
  - '@djensenius'
created_date: '2026-10-01 15:18'
labels:
  - release
milestone: m-0
dependencies:
  - TASK-4
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The owner installs Backlog.md with Homebrew and wants backlog-sync distributed the same way as their other tools, through djensenius/homebrew-tap. Because all of the owner's repos use a PR workflow, the release pipeline should propose formula updates as a pull request on the tap rather than pushing to it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The release pipeline opens a PR on djensenius/homebrew-tap that adds or updates Formula/backlog-sync.rb (version, URLs and sha256 for macOS arm64 and Linux) for each stable release
- [ ] #2 The formula installs the binary, runs `backlog-sync --version` in its test block, and the tap README lists it
- [ ] #3 The token the pipeline needs is a fine-grained token created by the owner and stored as a repository secret (documented; never committed)
- [ ] #4 `brew install djensenius/tap/backlog-sync` works for the first release
<!-- AC:END -->
