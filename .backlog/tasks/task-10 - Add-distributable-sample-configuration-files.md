---
id: TASK-10
title: Add distributable sample configuration files
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 16:03'
updated_date: '2026-10-01 19:24'
labels:
  - docs
  - config
milestone: m-0
dependencies: []
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The tool is meant to be configured per consumer repository, but the repo currently relies mostly on inline README snippets and old consumer examples. Committed sample configs give users copyable starting points and give tests/docs a stable source of truth.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Example config files exist for a minimal single-repo setup and a multi-repo setup
- [x] #2 Examples use placeholder owner/repo/project values and no consumer-specific absolute paths
- [x] #3 Tests or a documented validation command confirm the example configs parse with the current configuration schema
- [x] #4 README/config documentation links to the committed examples instead of duplicating divergent snippets
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add committed pure-JSON sample configs for a minimal single-repo setup and a multi-repo project-routing setup using placeholder values only.
2. Add a no-network test that loads each sample through the same strict JSON decode/normalization/validation path used by the command.
3. Update README/config docs to link to the committed examples instead of relying on divergent inline snippets.
4. Validate with go test ./..., go vet ./..., mise run ci, and Markdown/link checks where practical.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented sample config files and validation on branch task-10-sample-configs. Added examples/minimal-single-repo.json and examples/multi-repo.json as pure JSON placeholders without root/lockFile absolute paths; README now links to those examples instead of inline JSON snippets. Added config_samples_test.go to verify samples are valid JSON and pass parseFlags strict loadConfig decode, Normalized defaults, and validateConfig with --root t.TempDir() (no network). Validation passed: go test ./... -> ok github.com/djensenius/backlog-sync 0.777s; go vet ./... -> no output; mise run ci -> fmt/module/vet/test/build/lint passed, test line ok github.com/djensenius/backlog-sync 0.447s; local Markdown link check -> checked local Markdown links in README.md and docs/*.md; README stale config snippet grep -> no matches.

Reviewer follow-up fixed: release archives now include examples/minimal-single-repo.json and examples/multi-repo.json with deterministic GoReleaser metadata; README archive contents text names examples/; labels.managed docs now state that ["*"] adds any Backlog task label but does not remove issue labels, and explicit names/prefixes are required for removal; config sample tests now assert root/lockFile are omitted and loadConfig rejects unknown fields. Validation passed: go test ./... -> ok github.com/djensenius/backlog-sync 0.498s; go vet ./... -> no output; mise run ci -> fmt/module/vet/test/build/lint passed, test line ok github.com/djensenius/backlog-sync 0.445s; mise run release:check -> 1 configuration file(s) validated; mise run release:snapshot -> release succeeded and archived four platform tarballs; archive contents check -> README.md, LICENSE, examples/minimal-single-repo.json, examples/multi-repo.json, install-launchd.sh, and launchd/backlog-sync.plist.template present; Markdown link check -> checked local Markdown links in README.md and docs/*.md.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added distributable minimal and multi-repo sample configs, documented them from README/config reference, and fixed review findings so release archives ship the linked examples and label-management docs accurately describe wildcard add-only behavior. Verified samples through strict JSON config loading/normalization/validation, root/lockFile omission checks, unknown-field rejection, GoReleaser check/snapshot archive contents, local Markdown link checking, go test ./..., go vet ./..., and mise run ci.
<!-- SECTION:FINAL_SUMMARY:END -->
