---
id: TASK-4
title: 'GoReleaser configuration and tag-driven release workflow, like gopod'
status: To Do
assignee: []
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 16:03'
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
- [ ] #1 .goreleaser.yaml (v2) builds darwin/amd64, darwin/arm64, linux/amd64 and linux/arm64 with CGO off, -trimpath, reproducible timestamps, tar.gz archives (README, LICENSE, launchd template, install script) and checksums.txt
- [ ] #2 .github/workflows/release.yml publishes only semver tags that point to a commit on main, as a draft release (gopods validation and draft flow)
- [ ] #3 mise run release:check and release:snapshot work locally
- [ ] #4 A snapshot builds binary reports the injected version
<!-- AC:END -->
