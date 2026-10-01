# Backlog Sync

`backlog-sync` mirrors the local Backlog.md board to GitHub Issues and GitHub Project v2. Backlog.md remains the source of truth; GitHub edits are overwritten on the next sync except for the narrow `inbox` import path.

Target project for this repository: `users/djensenius/projects/8`.

## Cross-branch Backlog.md behavior

Backlog.md 1.53.0 reads `task list --json` and `task view --json` from the current checkout only (`includeCrossBranch: false`). `backlog board` scans active branches, but the current checkout's task copy wins; an `In Progress` task in another worktree can therefore appear as `To Do` on `main`. `updatedAt` is only minute-resolution, so ties are common.

This tool works around those limits by reading every local git worktree:

1. `git -C <root> worktree list --porcelain` discovers worktrees.
2. Bare, prunable, missing paths, and paths without `backlog/` are skipped.
3. In each worktree, the tool runs `backlog task list --json` with pagination and then `backlog task view <id> --json` for full task details.
4. If the main worktree returns zero tasks, or any required CLI call fails, the sync aborts before touching GitHub.

## Task resolution

For each task ID, the winning copy is chosen deterministically:

1. A worktree branch whose name starts with `<task-id>-` wins, matched case-insensitively on the numeric Backlog ID (for example, `TASK-1.2` is owned by `task-1.2-privacy-label`).
2. Otherwise, the latest `updatedAt` wins, falling back to `createdAt`.
3. Timestamp ties use the configured status order from `backlog config get statuses`; later statuses rank higher (`Done > In Progress > To Do` in this repository).
4. Remaining ties use the main worktree (`--root`).

The winning branch is rendered into the mirrored issue body when it is not `main`.

## GitHub mirror behavior

- One GitHub issue is created for each Backlog task.
- Existing issues are matched only by the first-line hidden marker `<!-- backlog:task-N -->` in the issue body.
- Issue titles are exactly `task-N: <title>` using a lowercase Backlog ID.
- Issues without a marker are never modified, except for `inbox` imports.
- Duplicate markers use the lowest-numbered issue; duplicates are logged and left untouched.
- Labels are set to the Backlog task labels, with an existing `inbox` label preserved.
- Missing mirrored labels are created with neutral colour `ededed`.
- `Done` tasks are closed with `state_reason: completed`; any other status is open/reopened.
- The Project v2 item is added if missing, and the Project `Status` single-select value is changed only when it differs.

Issue bodies are deterministic and contain the marker, one-way mirror notice, status, branch, parent, subtasks, dependencies, milestone, assignees, labels, description, acceptance criteria task list, and final summary when present.

## Inbox imports

Unless `--no-inbox` is set, open unmarked GitHub issues labelled `inbox` are imported into Backlog.md:

- The tool only imports when `--root` is on branch `main` with no merge, rebase, or cherry-pick in progress.
- It creates a new Backlog task with `backlog task create <issue title> --plain -d <issue body + imported-from footer> -l inbox`.
- A crash-replay guard searches existing root tasks for `Imported from GitHub issue #N:` and reuses that task ID instead of creating another task.
- The issue is then retitled to `task-N: <title>`, the marker is prepended, and the `inbox` label is removed.
- An `inbox` issue that already has a marker simply has the `inbox` label removed.
- In `--dry-run`, inbox imports print `would create task from #N` and make no GitHub or Backlog writes.

## Flags

```text
--root            repository root/main worktree (default: git toplevel of cwd)
--repo            GitHub repository owner/name (default: djensenius/canadian-ham)
--project-owner   GitHub Project v2 owner login (default: djensenius)
--project-number  GitHub Project v2 number (default: 8)
--dry-run         print planned writes without changing GitHub or Backlog
--no-inbox        skip inbox imports
--verbose         reserved for additional diagnostics
```

All log lines are written to stdout with an RFC3339 timestamp prefix. Any error exits non-zero.

## Build and run

Requires Go 1.27 or newer and the standard library only.

```bash
cd /Users/david/Developer/canadian-ham/tools/backlog-sync
go test ./...
go vet ./...
go build -o ~/bin/backlog-sync .
```

From the repository root, an equivalent build is:

```bash
go build -C tools/backlog-sync -o ~/bin/backlog-sync .
```

Run a read-only preview:

```bash
~/bin/backlog-sync --root /Users/david/Developer/canadian-ham --dry-run
```

Run for real:

```bash
~/bin/backlog-sync --root /Users/david/Developer/canadian-ham
```

GitHub access is performed only through `gh api` / `gh api graphql`, using the owner's existing `gh` authentication and project scope. Backlog reads and inbox task creation are performed through the `backlog` CLI.

## launchd install

A sample plist is in `tools/backlog-sync/launchd/com.djensenius.canadian-ham.backlog-sync.plist`. It runs the built binary every five minutes with:

- `--root /Users/david/Developer/canadian-ham`
- working directory `/Users/david/Developer/canadian-ham`
- logs at `/Users/david/Library/Logs/canadian-ham-backlog-sync.log`
- `PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`
- `HOME=/Users/david` so `gh` can find its config

Install after building the binary (do not install from a review worktree):

```bash
mkdir -p ~/Library/LaunchAgents ~/Library/Logs
cp /Users/david/Developer/canadian-ham/tools/backlog-sync/launchd/com.djensenius.canadian-ham.backlog-sync.plist \
  ~/Library/LaunchAgents/com.djensenius.canadian-ham.backlog-sync.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.djensenius.canadian-ham.backlog-sync.plist
launchctl kickstart -k gui/$(id -u)/com.djensenius.canadian-ham.backlog-sync
tail -f /Users/david/Library/Logs/canadian-ham-backlog-sync.log
```

Stop and unload:

```bash
launchctl bootout gui/$(id -u)/com.djensenius.canadian-ham.backlog-sync
```

The plist appends stdout and stderr to the same log file. Configure `newsyslog` for rotation if the job is left running long-term, or rotate the file manually during maintenance.
