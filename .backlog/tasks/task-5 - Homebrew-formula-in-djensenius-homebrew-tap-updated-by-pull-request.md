---
id: TASK-5
title: 'Homebrew formula in djensenius/homebrew-tap, updated by pull request'
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 18:13'
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

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Prepare all token-independent Homebrew release plumbing in this repository: document the planned fine-grained token/secret name and update release automation so the tap PR step is gated on that secret.
2. Add deterministic formula/README update generation for djensenius/homebrew-tap using release assets/checksums, with dry-run/local validation where possible.
3. Add or document validation commands for the generated formula and `backlog-sync --version` test path, without committing any secret.
4. Leave token-dependent and first-release-only acceptance evidence (actual tap PR creation and `brew install djensenius/tap/backlog-sync`) clearly noted as blocked until the owner stores the token and a stable release exists.
<!-- SECTION:PLAN:END -->
