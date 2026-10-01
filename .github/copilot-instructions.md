# Copilot review instructions

## Backlog.md task files
Files under `.backlog/` (and `backlog/`) are Backlog.md task-tracking records written by the `backlog` CLI, not code. Do not review or comment on them.

## Review context
- `backlog-sync` is Go with the standard library only; GitHub access goes through the `gh` CLI with explicit `owner/repo`, limited to the configured allowlist.
- Issue bodies and other output must be deterministic so a second sync run makes no changes.
- Dry runs must not write to GitHub, Backlog.md or git. Tests use fakes and no network.
