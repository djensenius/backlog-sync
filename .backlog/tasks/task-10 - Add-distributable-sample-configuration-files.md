---
id: TASK-10
title: Add distributable sample configuration files
status: To Do
assignee: []
created_date: '2026-10-01 16:03'
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
