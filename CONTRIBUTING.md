# Contributing to Memory-Sync

## Dev setup

- **Go 1.26+** (the `memory-sync` binary in `go/`).
- **uv** (for the `python/` v1 prototype, optional).
- **Git identity**: commits are authored `PAN <pandali.kk@qq.com>` (set locally on the repo).

## Test

```bash
cd go && go test ./...   # the suite is hermetic (TestMain provides git identity + init.defaultBranch=main)
go vet ./...
gofmt -l .
```

## Commit conventions

- **Conventional Commits** (`feat:`, `fix:`, `docs:`, `ci:`, `test:`).
- **No `Co-Authored-By` trailer** (project preference).
- **Squash** feature branches to 1 commit before merging to `main` (keeps GitHub activity clean).

## Add a slash command

Drop a `commands/<name>.md` (frontmatter `description` required; body uses `!`bash``, `$ARGUMENTS`, `${CLAUDE_PLUGIN_ROOT}`). Auto-discovered — no manifest edit.

## Release

Tag `v*` → `release.yml` cross-compiles (darwin/linux × amd64/arm64) → GitHub Releases binaries `memory-sync-<os>-<arch>` (the plugin's install curl fetches these).
