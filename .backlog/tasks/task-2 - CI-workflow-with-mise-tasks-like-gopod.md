---
id: TASK-2
title: 'CI workflow with mise tasks, like gopod'
status: Done
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 17:23'
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
- [x] #1 mise.toml pins Go and tools (actionlint, goreleaser, shellcheck) and defines fmt:check, module:check, vet, test, build, lint and ci tasks, mirroring gopod where it applies
- [x] #2 `.github/workflows/go.yml` runs `mise run ci` on ubuntu-latest and macos-latest for pull requests and pushes to main, with least-privilege permissions and actions pinned to full commit SHAs
- [x] #3 CodeQL (Go) runs on PRs and on a schedule
- [x] #4 shellcheck covers install-launchd.sh
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add root mise.toml pinning Go 1.27 plus actionlint, goreleaser, and shellcheck, with tasks fmt:check, module:check, vet, test, build, lint, and ci.
2. Add .github/workflows/go.yml that runs mise run ci on ubuntu-latest and macos-latest for PRs and pushes to main, with minimal permissions and all actions pinned to full commit SHAs.
3. Add .github/workflows/codeql.yml for Go analysis on PRs and a schedule, with minimal permissions and pinned actions.
4. Ensure lint covers actionlint and shellcheck install-launchd.sh.
5. Run mise run ci plus direct go vet ./... and go test ./..., then record evidence in task notes.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented the CI task files within the recorded plan: root mise.toml pins Go 1.27.1 plus actionlint 1.7.12, goreleaser 2.18.2, and shellcheck 0.11.0; tasks include fmt:check, module:check, vet, test, build, lint, and ci, with lint running actionlint and shellcheck install-launchd.sh. Added Go CI and CodeQL workflows with least-privilege permissions and actions pinned to full commit SHAs.

Check evidence:
- `mise run ci`: succeeded; ran `[test] $ go test ./...`, `[fmt:check] $ files=$(gofmt -l .)`, `[vet] $ go vet ./...`, `[module:check] $ go mod tidy -diff`, `[build] $ go build ./...`, and `[lint] $ actionlint && shellcheck install-launchd.sh`; output ended `Finished in 885.1ms`.
- `go vet ./...`: succeeded with no output.
- `go test ./...`: succeeded with `ok  	github.com/djensenius/backlog-sync	(cached)`.

Coordinator final validation after PR #2 opened:
- `mise run ci` succeeded locally; output included `go test ./...`, gofmt check, `go vet ./...`, `go mod tidy -diff`, `go build ./...`, `actionlint`, and `shellcheck install-launchd.sh`, ending `Finished in 743.6ms`.
- GitHub Actions on PR #2 passed `CI (ubuntu-latest)`, `CI (macos-latest)`, CodeQL `Analyze Go`, and the CodeQL result check.
- Independent reviewer returned APPROVE WITH NOTES with no blocking findings.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added mise-pinned local/CI checks plus Go and CodeQL GitHub Actions workflows. Verified with local `mise run ci`, `go test ./...`, `go vet ./...`, and PR #2 GitHub Actions passing on ubuntu-latest, macos-latest, and CodeQL.
<!-- SECTION:FINAL_SUMMARY:END -->
