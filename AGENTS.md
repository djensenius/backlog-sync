# Agent instructions

`backlog-sync` is a small, generic Go command (standard library only) that mirrors Backlog.md tasks
one-way to GitHub Issues and a GitHub Project v2. It runs locally (usually from launchd) for several
consumer repositories, each with its own `.backlog-sync.json`.

## Coordinator rules (the Pi session in the root pane)
- You are the coordinator. Plan and delegate; do not edit source files yourself.
- Track all work in Backlog.md (see "Backlog.md task workflow" below).
- Use a `scout` subagent to investigate before assigning implementation. When a worker gets stuck,
  inspect the failure yourself and hand the next worker concrete root causes.
- Use `worker` subagents for implementation. Before each worker, create a persistent branch and
  worktree named after the task (`git worktree add ../backlog-sync-task-12 -b task-12-short-slug`)
  in a separate step that finishes before the launch. Do not use managed `worktree: true` runs.
- Run workers one at a time unless the owner allows parallel lanes (separate worktrees, disjoint files).
  Keep worker scope small; split stalled work into smaller sequential workers.
- Each worker commits in logical steps (never amends) and reports the branch, full commit SHA, files,
  and the checks it ran with their actual output lines.
- After each worker, start a `reviewer` on the branch with the task spec; require it to fetch every
  `review_git` page. Only open a PR on APPROVE or APPROVE WITH NOTES.
- Never weaken a check to make it pass (no skipped tests, no raised timeouts without a diagnosed cause).
- Product or scope decisions belong to the owner: ask, then record the answer in the task.

## Pull request workflow (all agents)
Nothing reaches `main` directly. Every change goes through a pull request:
1. Work on the task branch (`task-12-short-slug`); task status changes ride in the same branch.
2. Push the branch and open a PR titled `task-12: Short title` that names the Backlog task and lists
   the checks run.
3. Copilot code review runs automatically and CI must pass. Address findings, push, reply on each
   comment, and request a new Copilot review.
4. Review-loop stopping rule: fix high-severity findings and re-request review. Medium or low
   findings, or ones that don't fit the project, are fixed or answered with reasoning and resolved.
   Findings about code the PR didn't change become follow-up Backlog tasks. Merge when the latest
   Copilot review has no unresolved high-severity findings and CI is green.
5. Either the owner or the coordinator merges. Afterwards, update the main checkout, re-run the
   checks, and remove the task worktree.

## Backlog.md task workflow (all agents)
Backlog.md (`.backlog/`) is the single source of truth for work. This repo's GitHub Project is a
one-way mirror produced by backlog-sync itself; never track work on GitHub.

- At the start of a session run `backlog instructions overview`, and read
  `backlog instructions task-creation`, `task-execution` or `task-finalization` before creating,
  working on, or finishing tasks. Also check for open GitHub issues labelled `inbox` and triage
  them into tasks (search first), then remove the label.
- Always use the `backlog` CLI with `--plain` (or `--json` for scripts). Never edit files in
  `.backlog/` directly. Pass task text with backticks as argv or single-quoted strings.
- Search before creating; descriptions say why, acceptance criteria are testable, use `--dep`, `-p`
  and `-m`, and set `-a` when the owner is known. No implementation plan at creation time.
- Start a task from its own branch: `backlog task edit task-12 -s "In Progress" -a @<name>`, then
  research and record a short plan with `--plan` before writing code. The GitHub Project (kept
  current from every worktree) is the live board; `backlog board` on main catches up when PRs merge.
- Make every later change (notes, checked criteria, final summary, Done) from the task's worktree so
  it lands with the PR. If a merge conflicts only in a `.backlog/` task file, keep the branch's version.
- When finished: verify each acceptance criterion with real evidence, check it, add notes, write a
  final summary, and set Done as the branch's last task change. Leave criteria unchecked if they
  need evidence you don't have.
- If you're blocked, say so in the task notes, leave it In Progress, and tell the owner.

## Project rules (all agents)
- Go, standard library only. Run `go vet ./...` and `go test ./...` (later: `mise run ci`).
- All GitHub access goes through the `gh` CLI with explicit `owner/repo` (never cwd-dependent
  default repo resolution) and is limited to the repos in the consumer's config (allowlist).
- All Backlog access goes through the `backlog` CLI. The tool never writes into a Backlog folder
  except creating inbox tasks via the CLI in `push` mode.
- Output must stay deterministic (no timestamps or run-dependent content in issue bodies), so a
  second run is a no-op. Tests use fakes; no network in tests.
- Dry runs make no writes (GitHub, Backlog, git).
