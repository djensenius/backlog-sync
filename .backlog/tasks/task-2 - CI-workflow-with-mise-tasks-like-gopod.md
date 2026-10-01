---
id: TASK-2
title: 'CI workflow with mise tasks, like gopod'
status: To Do
assignee: []
created_date: '2026-10-01 15:18'
labels:
  - ci
milestone: m-0
dependencies: []
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Every PR must be checked automatically before merge, on both platforms the tool supports. The owner's other Go projects (e.g. djensenius/gopod) use mise-pinned tools and a single `mise run ci` entry point, so contributors and CI run exactly the same checks.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 mise.toml pins Go and tools (actionlint, goreleaser, shellcheck) and defines fmt:check, module:check, vet, test, build, lint and ci tasks, mirroring gopod where it applies
- [ ] #2 `.github/workflows/go.yml` runs `mise run ci` on ubuntu-latest and macos-latest for pull requests and pushes to main, with least-privilege permissions and actions pinned to full commit SHAs
- [ ] #3 CodeQL (Go) runs on PRs and on a schedule
- [ ] #4 shellcheck covers install-launchd.sh
<!-- AC:END -->
