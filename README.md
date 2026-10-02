# Backlog Sync

`backlog-sync` mirrors local [Backlog.md](https://github.com/MrLesk/Backlog.md) tasks one-way to GitHub Issues and a GitHub Project v2. Backlog.md remains the source of truth: changes made directly on mirrored GitHub issues are overwritten on the next sync, except for the explicit `inbox` import path.

The command is generic. The JSON config describes the GitHub targets and optional sync settings; it does not, by its location alone, choose which local repository is scanned. With reusable configs that omit `root`, `backlog-sync` scans the git repository for the current directory unless `--root <path>` is passed. The same binary can run multiple independent configs for single-repo or multi-repo consumers.

> Caution: task titles, descriptions, acceptance criteria, plans, notes, metadata, and final summaries are copied into the configured issue repository. If that repository is public, the task text becomes public.

## Install

Choose one installation method per machine.

### Homebrew

After a tagged release is published and the Homebrew tap PR has merged, install the released binary with:

```bash
brew install djensenius/tap/backlog-sync
backlog-sync --version
```

Upgrade later with:

```bash
brew update
brew upgrade backlog-sync
```

The tap publication workflow and local formula rehearsal are documented in [docs/homebrew-tap.md](docs/homebrew-tap.md). Until the first stable release and tap PR exist, use a release archive or `go install` instead.

### `go install`

Use Go 1.27 or newer:

```bash
go install github.com/djensenius/backlog-sync@latest
# or pin a version once releases exist:
go install github.com/djensenius/backlog-sync@v0.1.0

backlog-sync --version
```

### Release archives

Tagged versions are packaged by GoReleaser as draft GitHub releases for macOS and Linux on `amd64` and `arm64`. A maintainer must publish the draft release before public download links work and before the Homebrew tap can publish the new version. Each archive contains:

- the `backlog-sync` binary
- `README.md`
- `LICENSE`
- `examples/minimal-single-repo.json`
- `examples/multi-repo.json`
- `install-launchd.sh`
- `launchd/backlog-sync.plist.template`

Download the archive for your OS/architecture from the [GitHub Releases page](https://github.com/djensenius/backlog-sync/releases), verify it against `checksums.txt`, and extract it:

```bash
version=v0.1.0
asset="backlog-sync_${version#v}_darwin_arm64.tar.gz"

gh release download "$version" --repo djensenius/backlog-sync --pattern "$asset" --pattern checksums.txt
grep "  ${asset}$" checksums.txt | shasum -a 256 -c -
# Linux: grep "  ${asset}$" checksums.txt | sha256sum -c -

tar -xzf "$asset"
./backlog-sync --version
```

### Build from a checkout

```bash
go test ./...
go vet ./...
go build -o ./backlog-sync .
./backlog-sync --version
```

## GitHub prerequisites

`backlog-sync` shells out to the GitHub CLI (`gh`) for all GitHub access. Authenticate `gh` for the account or bot that is allowed to edit the configured issue repositories and Project v2:

```bash
gh auth login -h github.com -s repo -s project
# To add scopes to an existing login:
gh auth refresh -h github.com -s repo -s project
```

The `repo` scope is needed for private issue repositories. The `project` scope is required for GitHub Projects v2 GraphQL reads and writes. Organization-owned projects may also require the organization to allow the token or bot account to access the project.

Before the first sync:

1. Create or choose every GitHub repository that can receive mirrored issues. The configured repositories are the allowlist; writes to any other repository are refused before invoking `gh`.
2. Create a GitHub Project v2 owned by the configured user or organization. In GitHub's UI, use **New project**. With `gh`, `gh project create --owner <OWNER_LOGIN> --title "Backlog"` can create a project when your installed `gh` version supports Projects v2 commands.
3. Record the project number from the project URL or `gh project list --owner <OWNER_LOGIN>`.
4. Ensure the project has a `Status` single-select field. Every Backlog status returned by `backlog config get statuses` must have a matching Status option, unless `statusMap` maps that Backlog status to a different Project option name. Missing options make the run fail before issue writes.
5. Optionally create project fields for the configured `fields` names. `priority`, `milestone`, and `area` are single-select fields. `taskId` and `branch` are text fields. Missing optional fields are skipped; missing single-select options are warned about, not created automatically.

## Quick start

Run from a checkout or worktree that contains Backlog.md data.

1. Copy one of the committed sample configs:

   - [examples/minimal-single-repo.json](examples/minimal-single-repo.json) mirrors all tasks to one issue repository.
   - [examples/multi-repo.json](examples/multi-repo.json) routes Backlog task `project` values to different issue repositories through `repos`.

2. Save it as `.backlog-sync.json` in the consumer repository for reusable CLI and launchd setups. For one-off CLI runs, the config can live at another private path, but the config file's directory does not select the repository to scan.
3. Replace placeholder owner, repository, and project values.
4. Start with a dry run from the consumer repository:

   ```bash
   cd /path/to/consumer-repo
   backlog-sync --config .backlog-sync.json --dry-run --verbose
   ```

   Or pass the repository explicitly when running from another directory:

   ```bash
   backlog-sync --root /path/to/consumer-repo --config /path/to/private/backlog-sync.json --dry-run --verbose
   ```

5. When the dry run is clean, run without `--dry-run`.

These example paths are placeholders, not consumer-specific paths. Avoid committing `root` or `lockFile` unless the absolute path is intentionally private to that consumer.

### Single-repo config example

Use this shape when every Backlog task should mirror to one issue repository. This is the same schema as [examples/minimal-single-repo.json](examples/minimal-single-repo.json):

```json
{
  "projectOwner": "OWNER_LOGIN",
  "projectOwnerType": "user",
  "projectNumber": 1,
  "repos": {},
  "defaultRepo": "OWNER_LOGIN/REPOSITORY_NAME",
  "mainBranch": "main",
  "statusMap": {},
  "inbox": {
    "enabled": true,
    "label": "inbox",
    "mode": "manual"
  },
  "adoptReferencedIssues": false,
  "labels": {
    "managed": ["*"],
    "addAlways": []
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

### Multi-repo config example

Use this shape when Backlog task `project` values should choose different issue repositories. Any task with an empty or unmapped `project` uses `defaultRepo`. This matches [examples/multi-repo.json](examples/multi-repo.json):

```json
{
  "projectOwner": "PROJECT_OWNER_LOGIN",
  "projectOwnerType": "org",
  "projectNumber": 1,
  "repos": {
    "PROJECT_A": "PROJECT_OWNER_LOGIN/PROJECT_A_REPOSITORY",
    "PROJECT_B": "PROJECT_OWNER_LOGIN/PROJECT_B_REPOSITORY",
    "PROJECT_DOCS": "PROJECT_OWNER_LOGIN/PROJECT_DOCS_REPOSITORY"
  },
  "defaultRepo": "PROJECT_OWNER_LOGIN/DEFAULT_REPOSITORY",
  "mainBranch": "main",
  "statusMap": {},
  "inbox": {
    "enabled": true,
    "label": "inbox",
    "mode": "manual"
  },
  "adoptReferencedIssues": false,
  "labels": {
    "managed": ["backlog", "type:*", "area:*", "priority:*"],
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

## Configuration reference

The config file is strict JSON. Unknown fields are rejected. JSON comments are not allowed.

| Field | Type | Required/default | Meaning |
| --- | --- | --- | --- |
| `root` | string | Optional; used when set and `--root` is absent; otherwise defaults to the git top-level of the current directory; overridden by `--root` | Main worktree root to scan and use for git operations. Omit it from committed reusable configs when possible. |
| `projectOwner` | string | Required | User or organization login that owns the GitHub Project v2. |
| `projectOwnerType` | string | Optional; default `user`; valid values `user`, `org` | Selects whether `projectOwner` is queried as a user or organization. |
| `projectNumber` | number | Required | Project v2 number, not the Project node ID. |
| `repos` | object of string to `owner/name` | Optional; default `{}` | Maps Backlog task `project` values to GitHub issue repositories. Keys must be non-empty. |
| `defaultRepo` | string `owner/name` | Required | Issue repository for tasks with no mapped project, and part of the allowlist scanned for markers and inbox issues. |
| `mainBranch` | string | Optional; default `main` | The canonical branch for root worktree checks, branch metadata, and inbox `push` mode. |
| `statusMap` | object of string to string | Optional; default `{}` | Maps Backlog status names to GitHub Project `Status` option names. Omitted statuses use identity mapping. Matching is case-insensitive when reading configured map keys. |
| `inbox` | object | Optional; defaults are listed below | Controls scanning of open issues carrying the inbox label. |
| `adoptReferencedIssues` | boolean | Optional; default `false` | Lets an unmirrored task adopt an existing referenced issue in the task's target repo. |
| `labels` | object | Optional; defaults to empty label management | Controls which Backlog labels may be added or removed and which labels are always added. |
| `fields` | object | Optional; defaults to no optional Project fields | Names optional GitHub Project fields to sync. Empty field names are skipped. |
| `subIssues` | boolean | Optional; default `false` | Enables GitHub parent/child issue links for Backlog subtasks. |
| `lockFile` | string | Optional; default is a per-config hash under the user's cache directory | File used for an advisory run lock. Set only when you need a stable custom lock location. |
| `timeoutSeconds` | number | Optional; default `60` | Timeout for each subprocess invocation (`git`, `backlog`, and `gh`). |

Runtime-only flags (`--dry-run`, `--no-inbox`, `--verbose`, and `--version`) are not JSON fields. `--repo`, `--project-owner`, `--project-owner-type`, `--project-number`, and `--main-branch` override their matching config values for local testing.

### Root and config path resolution

`--config` chooses which JSON file to load. Its directory does not choose the repository to scan.

- With `--root /path/to/repo`, the command scans that repository root. The flag also overrides any `root` value inside the config.
- Without `--root`, the command first resolves the git top-level of the current directory. If `--config` is omitted, it loads `<current-git-top-level>/.backlog-sync.json`.
- After loading the config, a `root` field in the config replaces the current-directory git top-level. If the config omits `root`, the current-directory git top-level remains the scanned repository.

For reusable configs, prefer omitting `root` and running either `cd /path/to/consumer-repo && backlog-sync --config .backlog-sync.json ...` or `backlog-sync --root /path/to/consumer-repo --config /path/to/config.json ...`.

### `inbox`

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `enabled` | boolean | `false` | Enables inbox scanning unless `--no-inbox` is passed. |
| `label` | string | `inbox` | GitHub issue label that marks inbox issues. The inbox label is never treated as a managed mirror label. |
| `mode` | string | `push`; valid values `manual`, `push` | Selects report-only triage or direct Backlog task creation. New configs should normally use `manual`. |
| `push` | boolean | `false` | Legacy switch honored only in `push` mode. When true and tasks were imported, the tool runs `git push origin <mainBranch>` after local Backlog changes. |

`manual` mode is report-only for new, unmarked inbox issues. The sync logs each issue as needing triage, claims it for that run so no adoption or sub-issue write touches it, and leaves the issue unchanged. A coordinator should create or update a normal Backlog task with the issue reference, then remove the inbox label by hand or intentionally add a task marker. If an issue already has both the inbox label and a Backlog marker, it mirrors normally and keeps the inbox label in manual mode.

`push` mode creates Backlog tasks directly for new inbox issues. It checks the root worktree once per run; when the root is not on `mainBranch`, is in the middle of merge/rebase/cherry-pick, or has dirty worktree/index changes, each unmarked inbox issue is logged and skipped instead of failing the whole run. For an unmarked issue that is ready to import, push mode creates a task with `--ref <issue URL>`, reverse-maps the issue repository to a Backlog `project` value when possible, removes the inbox label, adds the task marker to the issue, and retitles the issue. If an existing Backlog task already references the issue URL, push mode reuses that task instead of creating another one. If an already marked issue still has the inbox label, push mode mirrors it normally and removes only the inbox label.

`--no-inbox` skips inbox processing for a run regardless of config.

### `labels`

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `managed` | array of strings | `[]` | Label names or `prefix:*` patterns that the mirror may manage. `*` allows adding any Backlog task label, but wildcard does not remove existing issue labels. |
| `addAlways` | array of strings | `[]` | Labels added to every mirrored issue. Every value must be covered by `managed`. |
| `priorityPrefix` | string | empty | When set and a task has priority, adds a lower-case priority label such as `priority:high` if covered by `managed`. |

Managed label comparison is case-insensitive. Unmanaged labels are preserved. Labels missing from a repository are created with neutral color `ededed` before use.

### `fields`

| Field | Project field type | Value written |
| --- | --- | --- |
| `priority` | single-select | Backlog task priority |
| `milestone` | single-select | Backlog milestone title resolved from `backlog milestone list --plain` |
| `area` | single-select | Backlog task `project` value |
| `taskId` | text | Canonical lower-case task ID, for example `task-6` |
| `branch` | text | Winning worktree branch when it is not `mainBranch`; stale values are cleared |

The GitHub Project `Status` field is always required. These optional fields are skipped quietly when absent, with a one-time diagnostic in `--verbose` mode. Single-select options are matched case-insensitively. Missing optional options are not created automatically.

## Cross-branch Backlog CLI behavior

`backlog-sync` reads Backlog.md data through the `backlog` CLI, not by editing Backlog files directly.

For every run, it:

1. Resolves the root worktree from `--root`; otherwise from config `root` after loading the config; otherwise from the git top-level of the current directory. The config file's directory is not used as the root unless it is also the current-directory git top-level or is named by `root`/`--root`.
2. Runs `git -C <root> worktree list --porcelain`.
3. Skips bare, prunable, missing, and Backlog-less worktrees.
4. Discovers the Backlog data directory the same way the Backlog CLI would when invoked from each worktree root: root `backlog.config.yml` `backlog_directory`, then `backlog/`, then `.backlog/`.
5. Clears `BACKLOG_CWD` and other `BACKLOG_*` environment variables before invoking `backlog`, so each child process reads the intended worktree.
6. Reads tasks with paginated `backlog task list --json` and `backlog task view <id> --json`.

Remote-only branches are not scanned; a branch must have a local worktree to contribute task data.

When the same task ID appears in more than one worktree, the winning copy is deterministic:

1. A branch named exactly like the normalized task ID, or beginning with `<task-id>-`, wins case-insensitively. For example, branch `task-12-fix` owns `TASK-12`.
2. Otherwise, the newest `updatedAt` wins, falling back to `createdAt`.
3. Timestamp ties use the configured Backlog status order.
4. Remaining ties prefer the main worktree.

Task IDs use the Backlog task prefix read from `backlog config get taskPrefix` / `task_prefix`, with read-only fallback parsing of Backlog config files when needed. IDs are normalized to lowercase for markers and title prefixes, including dotted IDs such as `task-1.2.7`.

## Mirror behavior

- One GitHub issue is mirrored per Backlog task, matched by first-line marker `<!-- backlog:task-N -->` using the configured task prefix.
- Titles are exactly `task-N: <title>`.
- Issues without a marker are not modified, except through inbox imports or `adoptReferencedIssues` adoption.
- Duplicate markers use the lowest-numbered issue and log a warning.
- A task's `project` maps to an issue repo through `repos`; empty or unknown project values use `defaultRepo`. If a task later maps elsewhere after an issue already exists, the existing issue is updated in place and a warning is logged instead of creating a duplicate.
- `Done` tasks are closed with `state_reason: completed`; other statuses are opened or reopened.
- Issue bodies are rendered after issue creation/adoption so a second real run should be a no-op.
- Bodies are deterministic and include the marker, mirror notice, status, branch when not main, project, milestone title, priority, labels, assignees, parent/dependencies/subtasks with issue links when known, description, acceptance criteria, implementation plan, implementation notes, and final summary.

When `adoptReferencedIssues` is true and a task has no marked issue, a `https://github.com/<owner>/<repo>/issues/<N>` reference whose repo matches the task target repo can be adopted. Pull requests are ignored. If several references qualify, the lowest unclaimed issue number wins. An issue with a different marker, or an issue already claimed by another task through a marker, inbox link, or earlier adoption in the run, is never adopted.

When `subIssues` is true, the tool checks each child issue's current GitHub parent and links it under the parent task's issue only when different. Cross-repo sub-issue failures are logged once per run and do not abort the sync.

## Dry runs

Use `--dry-run` before enabling a new config or changing mappings. Run from the consumer repository, or pass `--root` explicitly:

```bash
cd /path/to/consumer-repo
backlog-sync --config .backlog-sync.json --dry-run --verbose

# From another directory:
backlog-sync --root /path/to/consumer-repo --config /path/to/private/backlog-sync.json --dry-run --verbose
```

Dry runs still read git worktrees, Backlog data, GitHub issues, and Project metadata from the resolved root repository. The `--config` path alone does not select that repository. Dry runs make no GitHub, Backlog, or git writes. Planned writes are printed as log lines. The command still validates Project Status options and config safety before it would write.

## launchd per config

On macOS, use `install-launchd.sh` to render one LaunchAgent plist per config. The script writes `~/Library/LaunchAgents/<label>.plist` and does not call `launchctl` unless `--load` is passed.

Current behavior: the generated plist sets `WorkingDirectory` to the config file's directory and runs `backlog-sync --config <path>` without `--root`. Because `--config` alone does not select the scan root, launchd configs should live in the consumer repository (normally as `/path/to/consumer-repo/.backlog-sync.json`) so the job starts inside that git repository. Do not point this installer at a private config outside the consumer repo unless you also maintain a custom plist/script that sets the working directory or passes `--root`.

Example placeholder paths:

```bash
./install-launchd.sh \
  --config /path/to/consumer-repo/.backlog-sync.json \
  --label com.example.backlog-sync.consumer-repo \
  --log /path/to/logs/backlog-sync-consumer-repo.log \
  --binary /path/to/bin/backlog-sync
```

Load immediately if desired:

```bash
./install-launchd.sh \
  --config /path/to/consumer-repo/.backlog-sync.json \
  --label com.example.backlog-sync.consumer-repo \
  --log /path/to/logs/backlog-sync-consumer-repo.log \
  --binary /path/to/bin/backlog-sync \
  --load
```

Stop a loaded job with:

```bash
launchctl bootout gui/$(id -u)/com.example.backlog-sync.consumer-repo
```

Release archives include [launchd/backlog-sync.plist.template](launchd/backlog-sync.plist.template) for manual rendering. Each config gets a distinct default lock path derived from the config path hash, unless `lockFile` is set. A second concurrent run for the same config exits 0 with `locked, skipping`; different configs can run side by side.

The plist appends stdout and stderr to the same log file. Configure log rotation if the job is left running long-term.

## Flags

```text
--config              JSON config path (default: <resolved-root>/.backlog-sync.json; does not select root by path)
--root                repository root/main worktree (default: git top-level of cwd, unless config root is set)
--repo                override defaultRepo
--project-owner       override projectOwner
--project-owner-type  override Project owner type: user or org
--project-number      override GitHub Project v2 number
--main-branch         override mainBranch
--dry-run             print planned writes without changing GitHub, Backlog, or git
--no-inbox            skip inbox processing
--verbose             print one-time diagnostics for skipped optional fields
--version             print version information and exit
```

All log lines are written to stdout with an RFC3339 timestamp prefix. Flag errors and runtime errors exit non-zero with the same timestamped format.

## Local validation

For this repository, the expected local checks are:

```bash
go test ./...
go vet ./...
mise run ci
mise run release:check
```
