---
id: TASK-3
title: Version flag and build metadata
status: Done
assignee:
  - '@pi'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-01 16:46'
labels:
  - cli
milestone: m-0
dependencies: []
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Release archives and Homebrew need `backlog-sync --version` to report what's installed, and GoReleaser injects version, commit and date at build time like gopod.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `backlog-sync --version` prints version, commit and date (defaults for local builds)
- [x] #2 main.version, main.commit and main.date can be set with -ldflags -X
- [x] #3 A test covers the version output
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add package-level version, commit, and date variables in main with local-build defaults suitable for -ldflags -X injection.
2. Extend CLI parsing so --version prints the three fields and exits before sync setup or config validation.
3. Add/adjust tests to exercise default version output and ldflags-settable variables without network or shelling to GitHub.
4. Run go test ./... and go vet ./..., then report exact output.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented --version handling before config/git setup, added ldflags-settable main.version/main.commit/main.date defaults, and added TestVersionOutputAndFlagBypassConfig. Verified default and injected outputs with go run plus go test ./... and go vet ./... .

Reviewer follow-up validation:
- TASK-10 creation and the TASK-6 dependency on TASK-10 are intentional coordinator/user-approved scope from the user-approved distribution/configuration request.
- `go test ./...`
  `ok  	github.com/djensenius/backlog-sync	(cached)`
- `go vet ./...`
  `(no output)`
- `go run . --version`
  `backlog-sync version=dev commit=none date=unknown`
- `go run -ldflags "-X main.version=v1.2.3 -X main.commit=abc1234 -X main.date=2026-10-01T15:18:00Z" . --version`
  `backlog-sync version=v1.2.3 commit=abc1234 date=2026-10-01T15:18:00Z`
- `go test -run TestVersionOutputAndFlagBypassConfig -v .`
  `=== RUN   TestVersionOutputAndFlagBypassConfig`
  `--- PASS: TestVersionOutputAndFlagBypassConfig (0.00s)`
  `PASS`
  `ok  	github.com/djensenius/backlog-sync	0.144s`
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added `--version` output with ldflags-settable `main.version`, `main.commit`, and `main.date`; verified defaults, injected metadata, and the dedicated version test with `go test ./...`, `go vet ./...`, `go run . --version`, `go run -ldflags "-X main.version=v1.2.3 -X main.commit=abc1234 -X main.date=2026-10-01T15:18:00Z" . --version`, and `go test -run TestVersionOutputAndFlagBypassConfig -v .`.
<!-- SECTION:FINAL_SUMMARY:END -->
