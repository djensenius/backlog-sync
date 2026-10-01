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

`.github/workflows/release.yml` separates draft release creation from tap publication:

1. A pushed semver tag still runs GoReleaser and creates or replaces the GitHub release as a draft. Stable `vMAJOR.MINOR.PATCH` tag runs also generate a local tap preview from `dist/checksums.txt`, but the tag-push workflow never checks out, pushes to, or opens a PR against `djensenius/homebrew-tap`. Draft-release asset URLs are not public yet, so they are not suitable for a Homebrew formula PR.
2. When that GitHub release is published, the `release` event job runs for stable `vMAJOR.MINOR.PATCH` tags only. It downloads `checksums.txt` from the public release URL under `https://github.com/djensenius/backlog-sync/releases/download/<tag>/checksums.txt`.
3. `scripts/homebrew-tap-update.go` reads the published release version plus downloaded checksums and deterministically generates:
   - `Formula/backlog-sync.rb`
   - the generated `backlog-sync` section of the tap `README.md`
4. If the published release tag is not a stable `vMAJOR.MINOR.PATCH` tag, the workflow emits a notice and skips all Homebrew tap work.
5. If `HOMEBREW_TAP_FINE_GRAINED_TOKEN` is not set, the workflow emits a notice and skips all tap checkout, branch, push, and PR steps after downloading the public checksums and generating the preview.
6. If the secret is set, the workflow checks out `djensenius/homebrew-tap`, prepares the managed `backlog-sync-<tag>` branch from the tap `main`, applies the generated files, validates Ruby syntax for the formula, pushes the branch, and opens a PR with `gh pr create --repo djensenius/homebrew-tap`.
7. If the managed branch or an open PR already exists, a rerun updates the branch and reports the existing PR URL instead of failing because the PR already exists.

The generated formula installs the published release archive binary and runs `backlog-sync --version` in its Homebrew `test do` block. It includes URLs and sha256 values for macOS arm64, macOS amd64, Linux arm64, and Linux amd64 assets from GoReleaser.

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
