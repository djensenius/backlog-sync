# Homebrew tap release preparation

TASK-5 publishes `backlog-sync` through the owner's tap, `djensenius/homebrew-tap`, without committing any credential. The release workflow can prepare formula changes without a token, but it opens the tap pull request only when the owner has stored the required secret.

## Required repository secret

Proposed secret name in `djensenius/backlog-sync`:

```text
HOMEBREW_TAP_FINE_GRAINED_TOKEN
```

The owner should create a fine-grained GitHub personal access token with the minimum scope needed for the tap PR workflow:

- Resource owner: `djensenius`
- Repository access: only `djensenius/homebrew-tap`
- Repository permissions:
  - Contents: Read and write
  - Pull requests: Read and write
  - Metadata: Read-only, automatically included by GitHub
- No issue, project, administration, or organization permissions are needed.

Store only the token value as the repository secret. Do not commit it, print it in logs, or copy it into Backlog notes.

## What the release workflow does

For every stable `vMAJOR.MINOR.PATCH` tag handled by `.github/workflows/release.yml`:

1. GoReleaser builds the release archives and `dist/checksums.txt`.
2. `scripts/homebrew-tap-update.go` reads the release version plus `dist/checksums.txt` and deterministically generates:
   - `Formula/backlog-sync.rb`
   - the generated `backlog-sync` section of the tap `README.md`
3. If the tag is not a stable `vMAJOR.MINOR.PATCH` tag, the workflow emits a notice and skips all Homebrew tap work.
4. If `HOMEBREW_TAP_FINE_GRAINED_TOKEN` is not set, the workflow emits a notice and skips all tap checkout, branch, push, and PR steps.
5. If the secret is set, the workflow checks out `djensenius/homebrew-tap`, applies the same generated files, validates Ruby syntax for the formula, commits the changes on `backlog-sync-<tag>`, pushes that branch to the tap, and opens a PR with `gh pr create --repo djensenius/homebrew-tap`.

The generated formula installs the release archive binary and runs `backlog-sync --version` in its Homebrew `test do` block. It includes URLs and sha256 values for macOS arm64, macOS amd64, Linux arm64, and Linux amd64 assets from GoReleaser.

## Local rehearsal without network writes

Use an existing GoReleaser `checksums.txt` to preview the tap update locally:

```bash
go run ./scripts/homebrew-tap-update.go \
  --version v0.1.0 \
  --checksums dist/checksums.txt \
  --out-dir /tmp/backlog-sync-homebrew-tap
ruby -c /tmp/backlog-sync-homebrew-tap/Formula/backlog-sync.rb
```

For a checked-out tap, use `--tap-dir /path/to/homebrew-tap` instead of `--out-dir`. The generator rewrites only `Formula/backlog-sync.rb` and the managed README block between the `backlog-sync formula section` comments.

If Homebrew is installed, run a local style check against the generated formula:

```bash
brew style /tmp/backlog-sync-homebrew-tap/Formula/backlog-sync.rb
```

Do not run `brew install djensenius/tap/backlog-sync` as acceptance evidence until the first stable release exists and the tap PR has merged.
