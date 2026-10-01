---
id: TASK-10
title: Add distributable sample configuration files
status: In Progress
assignee:
  - '@david'
created_date: '2026-10-01 16:03'
updated_date: '2026-10-01 18:43'
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
- [ ] #1 Example config files exist for a minimal single-repo setup and a multi-repo setup
- [ ] #2 Examples use placeholder owner/repo/project values and no consumer-specific absolute paths
- [ ] #3 Tests or a documented validation command confirm the example configs parse with the current configuration schema
- [ ] #4 README/config documentation links to the committed examples instead of duplicating divergent snippets
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add committed pure-JSON sample configs for a minimal single-repo setup and a multi-repo project-routing setup using placeholder values only.
2. Add a no-network test that loads each sample through the same strict JSON decode/normalization/validation path used by the command.
3. Update README/config docs to link to the committed examples instead of relying on divergent inline snippets.
4. Validate with go test ./..., go vet ./..., mise run ci, and Markdown/link checks where practical.
<!-- SECTION:PLAN:END -->
