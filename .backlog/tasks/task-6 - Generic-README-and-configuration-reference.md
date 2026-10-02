---
id: TASK-6
title: Generic README and configuration reference
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 15:18'
updated_date: '2026-10-02 01:41'
labels:
  - docs
milestone: m-0
dependencies:
  - TASK-4
  - TASK-10
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The README was written while the tool lived inside canadian-ham and still uses that repo's paths. New consumers (e.g. ArkhamHorror and the owner's other projects) need a standalone guide.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The README covers installation (Homebrew, go install, release archives), GitHub prerequisites (gh scopes incl. project, creating the Project and Status options), the full config reference with a single-repo and a multi-repo example, inbox modes, dry runs, launchd per config, and the cross-branch behaviour of the backlog CLI
- [ ] #2 No consumer-specific absolute paths remain except in clearly labelled examples
- [ ] #3 Markdown links resolve
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Rewrite the README into a standalone guide covering installation paths, GitHub prerequisites, project/status setup, full config reference, examples, inbox modes, dry runs, launchd per config, and cross-branch/worktree behavior.
2. Replace or clearly label any consumer-specific paths/examples so no unlabeled absolute consumer paths remain.
3. Add or run a local Markdown link check for README/docs/example links, and keep config docs aligned with current schema and sample files.
4. Validate with go test ./..., go vet ./..., mise run ci, release:check, and link checks before finalizing.

Review-fix pass: tighten README guidance around current-directory/--root scan semantics, launchd WorkingDirectory behavior, config root precedence, release draft publication, and inbox-mode details; add a focused sample drift test for fenced JSON config examples; validate with targeted tests, go test ./..., and go vet ./....
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented standalone generic README/config reference and added README link/stale-path checks. Evidence:
- `go test ./...`: ok github.com/djensenius/backlog-sync 0.864s.
- `go vet ./...`: passed with no output.
- `mise run ci`: passed; test package ok and finished in 812.2ms (mise emitted go-directive deprecation warning).
- `mise run release:check`: GoReleaser config validated (mise emitted go-directive deprecation warning).
- `go test ./... -run 'TestReadme(LinksToSampleConfigs|LocalMarkdownLinksResolve|HasNoConsumerSpecificPaths)'`: ok 0.099s.
- Stale path grep across README/examples/docs/launchd/install script: no `/Users/`, `canadian-ham`, or `ArkhamHorror` matches.

Starting review-fix pass after reviewer REQUEST CHANGES. Scope limited to README/docs/tests unless a tiny fixture update is required; no runtime root/config/launchd behavior changes planned.

Review-fix docs/tests implemented. Evidence so far: targeted README/config sample tests passed with go test ./... -run 'Test(ReadmeConfigExamplesMatchSampleFiles|ReadmeLinksToSampleConfigs|ReadmeLocalMarkdownLinksResolve|ReadmeHasNoConsumerSpecificPaths|SampleConfigsParseStrictlyWithoutNetwork)' (ok github.com/djensenius/backlog-sync 0.230s); go test ./... passed (ok github.com/djensenius/backlog-sync 0.766s); go vet ./... passed with no output. Acceptance criteria intentionally left unchecked until coordinator finalization.

Final review-fix validation after README root-row wording adjustment: targeted README/config sample tests passed (ok github.com/djensenius/backlog-sync 0.135s); go test ./... passed (ok github.com/djensenius/backlog-sync cached); go vet ./... passed with no output. No acceptance criteria checked or status finalized in this pass.
<!-- SECTION:NOTES:END -->
