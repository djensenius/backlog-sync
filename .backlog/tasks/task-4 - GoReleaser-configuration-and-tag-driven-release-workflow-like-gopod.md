---
id: TASK-4
title: 'GoReleaser configuration and tag-driven release workflow, like gopod'
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 17:35'
labels:
  - ci
  - release
milestone: m-0
dependencies:
  - TASK-2
  - TASK-3
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Users and other machines should install a tagged, checksummed binary instead of building from a checkout. gopod's GoReleaser v2 setup (draft releases from semver tags on main, reproducible archives, checksums) is the model.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `.goreleaser.yaml` (v2) builds `darwin/amd64`, `darwin/arm64`, `linux/amd64` and `linux/arm64` with `CGO` off, `-trimpath`, reproducible timestamps, `tar.gz` archives (`README`, `LICENSE`, launchd template, install script) and `checksums.txt`
- [ ] #2 `.github/workflows/release.yml` publishes only semver tags that point to a commit on `main`, as a draft release (gopod's validation and draft flow)
- [ ] #3 `mise run release:check` and `mise run release:snapshot` work locally
- [ ] #4 A snapshot build's binary reports the injected version
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a GoReleaser v2 configuration that builds the four required darwin/linux amd64/arm64 targets with CGO disabled, trimpath/metadata ldflags, reproducible archives, bundled README/LICENSE/launchd/install assets, and checksums.
2. Extend mise.toml with release:check and release:snapshot tasks that validate the GoReleaser config and produce a local snapshot build.
3. Add a pinned release workflow that runs only for tag pushes, validates the tag as semver, verifies the tagged commit is contained in origin/main, and publishes a draft GoReleaser release.
4. Verify locally with mise run release:check, mise run release:snapshot, snapshot binary --version output, mise run ci/go test/go vet, and actionlint.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented release workflow/config in commit ef72efac00bad6520ad4be8945369dfc3eef8363.

Check evidence from the task worktree:
- `mise run release:check`: `[release:check]   • 1 configuration file(s) validated`; `[release:check] Finished in 38.5ms`.
- `mise run release:snapshot`: built `darwin_amd64_v1`, `darwin_arm64_v8.0`, `linux_arm64_v8.0`, and `linux_amd64_v1`; archived `backlog-sync_0.0.0-snapshot-ef72efa_{darwin_amd64,darwin_arm64,linux_amd64,linux_arm64}.tar.gz`; `release succeeded after 1s`.
- Snapshot binary: `backlog-sync version=0.0.0-snapshot-ef72efa commit=ef72efac00bad6520ad4be8945369dfc3eef8363 date=2026-10-01T17:30:17Z`.
- Archive/checksum spot check: `dist/checksums.txt` contains all four tar.gz archives; the darwin/arm64 tarball contains `backlog-sync`, `README.md`, `LICENSE`, `install-launchd.sh`, and `launchd/com.djensenius.canadian-ham.backlog-sync.plist`; tar metadata showed root/root ownership and commit-time mtimes.
- `mise run ci`: `[test] ok  	github.com/djensenius/backlog-sync	0.171s`; `Finished in 514.4ms`.
- `go test ./...`: `ok  	github.com/djensenius/backlog-sync	0.170s`.
- `go vet ./...`: passed with no output.
- `actionlint .github/workflows/release.yml`: passed with no output.

Reviewer follow-up implemented in commit 83053c5d1ffc777ee618ebca81fc07e45008e785: release workflow now resolves tag commits as refs/tags/${tag}^{commit}, passes GORELEASER_CURRENT_TAG=${{ github.ref_name }} to GoReleaser, disables mise-action caching, and GoReleaser release config sets replace_existing_draft: true.

Follow-up validation from the task worktree:
- `actionlint .github/workflows/release.yml`: passed; output only the mise deprecation warning about go.mod go directive.
- `mise run release:check`: `[release:check]   • 1 configuration file(s) validated`; `[release:check] Finished in 42.0ms`.
- `mise run release:snapshot`: built snapshot version `0.0.0-snapshot-83053c5` for `linux_arm64_v8.0`, `linux_amd64_v1`, `darwin_amd64_v1`, and `darwin_arm64_v8.0`; archived all four tarballs; `release succeeded after 1s`.
- Snapshot binary: `dist/backlog-sync_darwin_arm64_v8.0/backlog-sync --version` -> `backlog-sync version=0.0.0-snapshot-83053c5 commit=83053c5d1ffc777ee618ebca81fc07e45008e785 date=2026-10-01T17:35:06Z`.
- `mise run ci`: `[test] ok  	github.com/djensenius/backlog-sync	0.224s`; `Finished in 556.3ms`.
- `go test ./...`: `ok  	github.com/djensenius/backlog-sync	(cached)`.
- `go vet ./...`: passed with no output.
<!-- SECTION:NOTES:END -->
