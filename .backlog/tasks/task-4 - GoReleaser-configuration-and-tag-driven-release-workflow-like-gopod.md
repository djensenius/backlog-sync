---
id: TASK-4
title: 'GoReleaser configuration and tag-driven release workflow, like gopod'
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 17:27'
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
