---
id: TASK-5
title: 'Homebrew formula in djensenius/homebrew-tap, updated by pull request'
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 18:19'
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

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Token-independent prep implemented on branch task-5-homebrew-tap-pr. Added docs/homebrew-tap.md documenting required fine-grained secret HOMEBREW_TAP_FINE_GRAINED_TOKEN for djensenius/backlog-sync with access limited to djensenius/homebrew-tap contents read/write and pull requests read/write. Added scripts/homebrew-tap-update.go to deterministically generate Formula/backlog-sync.rb and a managed tap README block from GoReleaser release assets and checksums, plus go tests covering generation/idempotence/missing checksum failure. Wired .github/workflows/release.yml to generate a tap preview after GoReleaser, skip tap checkout/push/PR with a clear notice when the secret is absent, and only checkout djensenius/homebrew-tap/open a PR when the secret exists. Local evidence: gh issue list --repo djensenius/backlog-sync --label inbox --state open --limit 100 returned no output; go test ./... passed; go vet ./... passed; actionlint passed (mise emitted only the existing go directive deprecation warning); mise run ci passed; mise run release:check passed; mise run release:snapshot passed; generator rehearsal against dist/checksums.txt passed; ruby -c on generated Formula/backlog-sync.rb returned Syntax OK; brew style on generated Formula/backlog-sync.rb reported 1 file inspected, no offenses detected. Remaining blocked acceptance: actual tap PR creation still requires owner to store HOMEBREW_TAP_FINE_GRAINED_TOKEN, and brew install djensenius/tap/backlog-sync cannot be proven until the first stable release exists and the tap PR merges. No token or secret value was used or committed.
<!-- SECTION:NOTES:END -->
