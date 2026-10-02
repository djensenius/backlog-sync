---
id: TASK-5
title: 'Homebrew formula in djensenius/homebrew-tap, updated by pull request'
status: Done
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-02 02:17'
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
- [x] #1 The release pipeline opens a PR on djensenius/homebrew-tap that adds or updates Formula/backlog-sync.rb (version, URLs and sha256 for macOS arm64 and Linux) for each stable release
- [x] #2 The formula installs the binary, runs `backlog-sync --version` in its test block, and the tap README lists it
- [x] #3 The token the pipeline needs is a fine-grained token created by the owner and stored as a repository secret (documented; never committed)
- [x] #4 `brew install djensenius/tap/backlog-sync` works for the first release
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Prepare all token-independent Homebrew release plumbing in this repository: document the planned fine-grained token/secret name and update release automation so the tap PR step is gated on that secret.
2. Add deterministic formula/README update generation for djensenius/homebrew-tap using release assets/checksums, with dry-run/local validation where possible.
3. Add or document validation commands for the generated formula and `backlog-sync --version` test path, without committing any secret.
4. Leave token-dependent and first-release-only acceptance evidence (actual tap PR creation and `brew install djensenius/tap/backlog-sync`) clearly noted as blocked until the owner stores the token and a stable release exists.

5. Review blocker follow-up: split .github/workflows/release.yml so tag pushes publish only the draft release/local preview, while release.published events for stable tags download public checksums, update the tap branch, and open or report the tap PR with rerun-safe branch/PR handling.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Token-independent prep implemented on branch task-5-homebrew-tap-pr. Added docs/homebrew-tap.md documenting required fine-grained secret HOMEBREW_TAP_FINE_GRAINED_TOKEN for djensenius/backlog-sync with access limited to djensenius/homebrew-tap contents read/write and pull requests read/write. Added scripts/homebrew-tap-update.go to deterministically generate Formula/backlog-sync.rb and a managed tap README block from GoReleaser release assets and checksums, plus go tests covering generation/idempotence/missing checksum failure. Wired .github/workflows/release.yml to generate a tap preview after GoReleaser, skip tap checkout/push/PR with a clear notice when the secret is absent, and only checkout djensenius/homebrew-tap/open a PR when the secret exists. Local evidence: gh issue list --repo djensenius/backlog-sync --label inbox --state open --limit 100 returned no output; go test ./... passed; go vet ./... passed; actionlint passed (mise emitted only the existing go directive deprecation warning); mise run ci passed; mise run release:check passed; mise run release:snapshot passed; generator rehearsal against dist/checksums.txt passed; ruby -c on generated Formula/backlog-sync.rb returned Syntax OK; brew style on generated Formula/backlog-sync.rb reported 1 file inspected, no offenses detected. Remaining blocked acceptance: actual tap PR creation still requires owner to store HOMEBREW_TAP_FINE_GRAINED_TOKEN, and brew install djensenius/tap/backlog-sync cannot be proven until the first stable release exists and the tap PR merges. No token or secret value was used or committed.

Review blocker fixed in commit 87d4810: .github/workflows/release.yml now keeps tag pushes limited to draft-release creation plus a local stable-tag preview, and moves Homebrew tap PR creation to the release.published job for stable vMAJOR.MINOR.PATCH tags. The published-release job downloads checksums.txt from the public djensenius/backlog-sync release URL, generates a preview, clearly skips tap writes when HOMEBREW_TAP_FINE_GRAINED_TOKEN is absent, and only with that token checks out djensenius/homebrew-tap, prepares/updates the managed backlog-sync-<tag> branch, pushes it, and creates or reports the existing PR. docs/homebrew-tap.md now documents the draft-release vs published-release flow and rerun behavior. Local evidence for this follow-up: actionlint passed; go test ./... passed; go vet ./... passed; mise run ci passed; mise run release:check passed; mise run release:snapshot passed; generator rehearsal from dist/checksums.txt to /tmp/backlog-sync-homebrew-tap passed; ruby -c /tmp/backlog-sync-homebrew-tap/Formula/backlog-sync.rb returned Syntax OK. No secrets were used locally.

Coordinator validation after blocker fix:
- Independent reviewer returned APPROVE WITH NOTES; prior blocker was fixed by moving tap PR creation to the published-release path instead of the draft-release tag-push path.
- `go test ./...` passed: `ok   github.com/djensenius/backlog-sync 0.636s`.
- `go vet ./...` passed with no output.
- `mise run ci` passed; output included fmt, module, vet, test, build, and lint tasks and ended `Finished in 963.6ms`.
- `mise run release:check` passed with `1 configuration file(s) validated`; finished in 44.2ms.
- `actionlint` passed with only the existing mise go directive deprecation warning.
- `mise run release:snapshot` passed, built snapshot version `0.0.0-snapshot-657f550`, archived all four target tarballs, and reported `release succeeded after 4s`.
- Generator rehearsal using `dist/checksums.txt` and version `v0.0.0-snapshot-657f550` succeeded; generated formula passed `ruby -c` (`Syntax OK`) and `brew style` (`1 file inspected, no offenses detected`).
Remaining blockers: actual tap PR creation requires the owner-created `HOMEBREW_TAP_FINE_GRAINED_TOKEN`; `brew install djensenius/tap/backlog-sync` requires a published stable release and merged tap PR.

Release and Homebrew acceptance completed. Evidence: repository secret HOMEBREW_TAP_FINE_GRAINED_TOKEN exists in djensenius/backlog-sync as an Actions repository secret (verified by `gh secret list`, value not printed); release readiness checks passed before tagging (`go test ./...`, `go vet ./...`, `mise run ci`, `mise run release:check`); tag `v0.1.0` was pushed from main commit `0eca8d7caf8d727f2bbc54c54c14bd9607388f6d`; Release workflow run `36954697787` created the draft release successfully; published release https://github.com/djensenius/backlog-sync/releases/tag/v0.1.0 triggered Homebrew tap PR workflow run `36954796336`, which succeeded and opened https://github.com/djensenius/homebrew-tap/pull/1; tap PR #1 updated `Formula/backlog-sync.rb` and README, passed local `ruby -c` and `brew style`, and was merged. Final install evidence: `brew install djensenius/tap/backlog-sync` installed 0.1.0; `/opt/homebrew/bin/backlog-sync --version` printed `backlog-sync version=0.1.0 commit=0eca8d7caf8d727f2bbc54c54c14bd9607388f6d date=2026-10-02T02:03:15Z`; `brew test djensenius/tap/backlog-sync` ran `/opt/homebrew/Cellar/backlog-sync/0.1.0/bin/backlog-sync --version` successfully.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Published the first stable `backlog-sync` release (`v0.1.0`), verified the owner-provided `HOMEBREW_TAP_FINE_GRAINED_TOKEN` repository secret without exposing its value, and confirmed the release workflow opened the generated Homebrew tap PR. Merged djensenius/homebrew-tap PR #1, which added `Formula/backlog-sync.rb` with macOS/Linux amd64/arm64 release URLs and sha256 values plus the README listing. Verified installation with `brew install djensenius/tap/backlog-sync`, `/opt/homebrew/bin/backlog-sync --version`, and `brew test djensenius/tap/backlog-sync`.
<!-- SECTION:FINAL_SUMMARY:END -->
