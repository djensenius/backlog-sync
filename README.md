# Backlog Sync

`backlog-sync` mirrors local Backlog.md tasks to GitHub Issues and GitHub Project v2. Backlog.md remains the source of truth; GitHub edits are overwritten on the next sync except for the narrow `inbox` push import path.

The binary is consumer-neutral. All repo/project choices come from a JSON config selected with `--config <path>` (default: `<root>/.backlog-sync.json`). Flags only override config fields for local testing.

> Caution: task titles, descriptions, acceptance criteria, plans, notes, metadata and final summaries are published into whichever issue repository the task maps to. Public issue repos make that task text public.

## Configuration

```jsonc
{
  "root": "/abs/path/to/backlog/repo",          // optional; default = git toplevel of cwd / --root
  "projectOwner": "djensenius",                  // user or org login
  "projectOwnerType": "user",                    // "user" | "org"
  "projectNumber": 9,
  "repos": {                                     // Backlog task project -> issue repo
    "fork": "djensenius/ArkhamHorror",
    "apple": "djensenius/ArkhamHorror-Apple",
    "linux": "djensenius/ArkhamHorror-Linux",
    "meta": "djensenius/ArkhamHorror-Project"
  },
  "defaultRepo": "djensenius/ArkhamHorror-Project",
  "mainBranch": "main",
  "statusMap": {},
  "inbox": { "enabled": true, "label": "inbox", "mode": "push", "push": true },
  "adoptReferencedIssues": false,
  "labels": {
    "managed": ["backlog", "type:*", "area:*", "priority:*", "upstream-candidate", "roadmap", "owner"],
    "addAlways": ["backlog"],
    "priorityPrefix": "priority:"
  },
  "fields": {
    "priority": "Priority",
    "milestone": "Backlog milestone",
    "area": "Area",
    "taskId": "Task ID",
    "branch": "Branch"
  },
  "subIssues": true,
  "lockFile": "/Users/david/Library/Caches/example.lock",
  "timeoutSeconds": 60
}
```

Canadian Ham config committed at the repository root:

```jsonc
{
  "projectOwner": "djensenius",
  "projectOwnerType": "user",
  "projectNumber": 8,
  "repos": {},
  "defaultRepo": "djensenius/canadian-ham",
  "mainBranch": "main",
  "statusMap": {},
  "inbox": { "enabled": true, "label": "inbox", "mode": "manual" },
  "adoptReferencedIssues": false,
  "labels": { "managed": ["*"], "addAlways": [] },
  "subIssues": true,
  "timeoutSeconds": 60
}
```

`labels.managed: ["*"]` means every Backlog task label is eligible to be added to its mirrored issue. It still does not remove arbitrary existing issue labels, and the configured inbox label is never part of the mirror label set.

## Cross-worktree Backlog reads

The tool runs `git -C <root> worktree list --porcelain`, skips bare/prunable/missing worktrees and worktrees without a Backlog data directory, then reads each worktree through the `backlog` CLI (`task list --json` with pagination and `task view <id> --json`). The data directory is discovered read-only from the worktree using the same layouts the Backlog.md CLI resolves when run from the worktree root: root `backlog.config.yml` `backlog_directory`, then `backlog/`, then `.backlog/`. It does not recursively search for nested custom folders because the CLI invoked from the worktree root would not use them. Pagination is validated against the reported `total`. The main worktree must resolve and scan a Backlog directory and must return at least one task before any GitHub call is made.

`BACKLOG_CWD` and other `BACKLOG_*` environment variables are stripped from child processes so the CLI reads the intended worktree. Remote-only branches without a local worktree are out of scope.

Task IDs use the Backlog task prefix read from `backlog config get taskPrefix` / `task_prefix`, falling back to read-only parsing of root `backlog.config.yml` and then the discovered Backlog `config.yml` or `config.yaml` when needed. IDs are normalized to lowercase for markers and title prefixes, including dotted IDs such as `task-1.2.7`.

## Task resolution and placement

For each task ID, the winning copy is chosen deterministically:

1. A worktree branch whose name equals the normalized task ID, or starts with the ID plus `-`, wins case-insensitively. `task-1.2.7-foo` owns `TASK-1.2.7` but not `TASK-1.2`.
2. Otherwise, the latest `updatedAt` wins, falling back to `createdAt`.
3. Timestamp ties use configured Backlog status order.
4. Remaining ties prefer the main worktree.

A task's `project` maps to an issue repo through `repos`; empty or unknown project values use `defaultRepo`. Marker matching searches all configured repos. If a task moves projects after an issue exists elsewhere, the existing issue is updated in place and a warning is logged instead of creating a duplicate.

All GitHub issue calls use explicit REST/GraphQL paths and are wrapped by an allowlist made from `repos` plus `defaultRepo`; unconfigured repos are refused before invoking `gh`.

## GitHub mirror behavior

- One issue is mirrored per task, matched only by first-line marker `<!-- backlog:task-N -->`.
- Titles are exactly `task-N: <title>`.
- Issues without a marker are not modified, except for `inbox` imports and explicit `adoptReferencedIssues` adoption.
- Duplicate markers use the lowest-numbered issue and log a warning.
- Managed labels are compared case-insensitively. Desired labels are `(task labels ∩ managed) ∪ addAlways ∪ priority label`; only managed labels are removed. Unmanaged labels and the inbox label are preserved.
- Missing desired labels are created per repo with neutral colour `ededed`.
- `Done` tasks are closed with `state_reason: completed`; other statuses are opened/reopened.
- Project Status is set through GraphQL using the configured `statusMap` or identity mapping.
- Issue bodies are rendered in a second pass after issue creation/adoption so a second real run is expected to be a no-op.

Bodies are deterministic and include the marker, mirror notice, status, branch when not main, project, milestone title, priority, labels, assignees, parent/dependencies/subtasks with issue links when known, description, acceptance criteria with checked state, implementation plan, implementation notes, and final summary.

## Adoption, Project fields, and sub-issues

When `adoptReferencedIssues` is true and a task has no marked issue, a `https://github.com/<owner>/<repo>/issues/<N>` reference whose repo matches the task target repo is adopted. Pull requests are ignored. If several references qualify, the lowest unclaimed issue number wins and a warning is logged. An issue with a different marker, or an issue already claimed by another task through a marker/inbox link/adoption earlier in the run, is never adopted.

Optional Project fields are skipped quietly when absent (with a one-time `--verbose` diagnostic). Single-select options are matched case-insensitively. Milestone IDs are mapped to titles from `backlog milestone list --plain` (for example `m-0: M1: Night of the Zealot on Apple`). Missing options are **not** auto-added: GitHub's GraphQL docs for `updateProjectV2Field` and `ProjectV2SingleSelectFieldOptionInput` require sending a `singleSelectOptions` list when changing options and expose option IDs, but they do not guarantee that resubmitting/omitting options preserves all existing item values (see https://docs.github.com/en/graphql/reference/mutations#updateprojectv2field and https://docs.github.com/en/graphql/reference/input-objects#projectv2single-select-field-option-input). The tool therefore logs a warning instead of mutating field options. Empty single-select values are skipped when already empty and cleared with `clearProjectV2ItemFieldValue` when stale. Text fields update only when changed; an empty winning main branch clears the Branch field with `clearProjectV2ItemFieldValue`.

When `subIssues` is true, the tool checks each child issue's current GitHub parent and links it under the parent task's issue only when different. Cross-repo sub-issue failures are logged once per run and do not abort the sync.

## Inbox imports

Open issues labelled with the configured inbox label are scanned in every configured repo. The inbox label is not a managed mirror label.

`inbox.mode` selects how new tasks are imported:

- `push` preserves the direct-commit behavior used by repositories whose Backlog branch is allowed to take direct commits. Before creating a task, the root worktree must be on `mainBranch`, not mid-merge/rebase/cherry-pick, and have a clean worktree/index. The tool first checks whether any task already references the issue URL; if so, it reuses that task and only marks the issue. Otherwise it creates a task with `--ref <issue URL>`, `--project <reverse-mapped project>` when applicable, and the issue body as the description. It then immediately removes the inbox label, prepends the marker, and retitles the issue. After any inbox creation, `git -C <root> push origin <mainBranch>` runs when legacy `inbox.push` is true; push failures are logged without aborting.
- `manual` is report-only for repositories where inbox issues must be triaged into normal task changes by a coordinator. An open inbox-labelled issue that already carries a task marker belongs to the mirror: it is not reported as needing triage, it is mirrored normally, its inbox label is left alone, and managed labels never include the inbox label. An open inbox-labelled issue without a marker is report-only: it is added to the run's claimed-issues set so `adoptReferencedIssues` and every other pass never write to it, logged as `inbox issue <repo>#<N> needs triage: <title>`, and counted in the `<N> inbox issues need triage` sync summary; the same summary clause also appears in `push` mode. It never creates a Backlog task, edits labels or issue bodies, creates markers, creates branches, or pushes for those unmarked inbox issues.

Existing tasks are never updated. `--dry-run` performs no GitHub, Backlog, or git writes.

## Flags

```text
--config              JSON config path (default: <root>/.backlog-sync.json)
--root                repository root/main worktree (default: git toplevel of cwd)
--repo                override defaultRepo
--project-owner       override projectOwner
--project-owner-type  override projectOwnerType
--project-number      override projectNumber
--main-branch         override mainBranch
--dry-run             print planned writes without changing GitHub, Backlog, or git
--no-inbox            skip inbox imports
--verbose             print one-time diagnostics for skipped optional fields
```

All log lines are written to stdout with an RFC3339 timestamp prefix. Flag errors and runtime errors exit non-zero with the same timestamped format. Each subprocess has a configurable timeout (`timeoutSeconds`, default 60 seconds).

## Build and dry run

Requires Go 1.27 or newer and the standard library only.

```bash
cd /Users/david/Developer/canadian-ham/tools/backlog-sync
go test ./...
go vet ./...
go build -o ~/bin/backlog-sync .
~/bin/backlog-sync --config /Users/david/Developer/canadian-ham/.backlog-sync.json --dry-run
```

ArkhamHorror uses the same binary with a temporary or private config, for example:

```jsonc
{
  "root": "/Users/david/Developer/ArkhamHorror",
  "projectOwner": "djensenius",
  "projectOwnerType": "user",
  "projectNumber": 9,
  "repos": {
    "fork": "djensenius/ArkhamHorror",
    "apple": "djensenius/ArkhamHorror-Apple",
    "linux": "djensenius/ArkhamHorror-Linux",
    "meta": "djensenius/ArkhamHorror-Project"
  },
  "defaultRepo": "djensenius/ArkhamHorror-Project",
  "mainBranch": "main",
  "inbox": { "enabled": true, "label": "inbox", "mode": "push", "push": true },
  "adoptReferencedIssues": true,
  "labels": {
    "managed": ["backlog", "type:*", "area:*", "priority:*", "upstream-candidate", "roadmap", "owner"],
    "addAlways": ["backlog"],
    "priorityPrefix": "priority:"
  },
  "fields": {
    "priority": "Priority",
    "milestone": "Backlog milestone",
    "area": "Area",
    "taskId": "Task ID",
    "branch": "Branch"
  },
  "subIssues": true,
  "timeoutSeconds": 60
}
```

## launchd install

Use `install-launchd.sh` to render one LaunchAgent plist per config. It does not call `launchctl` unless `--load` is passed.

```bash
tools/backlog-sync/install-launchd.sh \
  --config /Users/david/Developer/canadian-ham/.backlog-sync.json \
  --label com.djensenius.canadian-ham.backlog-sync \
  --log /Users/david/Library/Logs/canadian-ham-backlog-sync.log \
  --binary /Users/david/bin/backlog-sync

tools/backlog-sync/install-launchd.sh \
  --config /Users/david/Developer/ArkhamHorror/.backlog-sync.json \
  --label com.djensenius.arkham-backlog-sync \
  --log /Users/david/Library/Logs/arkham-backlog-sync.log \
  --binary /Users/david/bin/backlog-sync
```

Each config gets a distinct default lock path derived from the config path hash (or `lockFile` if configured). A second concurrent run for the same config exits 0 with `locked, skipping`; different configs can run side by side.

Stop/unload a loaded job with:

```bash
launchctl bootout gui/$(id -u)/com.djensenius.canadian-ham.backlog-sync
```

The plist appends stdout and stderr to the same log file. Configure log rotation if the job is left running long-term.
