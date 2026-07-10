# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-07-09

Initial release.

### Engine

- `checkpoint` — normalize transcript + memory → push to a private git sync-store.
- `restore` — pull → rewrite paths to the target layout → print `claude --resume <uuid>`.
- `status` — show checkpoint state for the current session.
- `install` — config setup: sync-store URL + private/empty/SSH checks + toml write.
- step0 gate probe — verifies `claude --resume` accepts filtered/stub transcripts.
- gzip transcripts larger than 1 MB.
- 50 MB / 100 MB bundle-size gate (warn / abort — GitHub release asset limit).

### Sync-store

- Private git repo holding plaintext session checkpoints.
- `install` refuses a public sync-store.

### CI / Release

- CI: test / `go vet` / `gofmt` / race detector + shellcheck + plugin-manifest validation.
- Release: cross-compile darwin/linux × amd64/arm64.

### Known limitations

- Transcripts are stored **unredacted** in the user's private sync-store (see [SECURITY.md](SECURITY.md)). Do not checkpoint sessions containing live secrets to a public repo.
- A write failure mid-checkpoint may corrupt the checkpoint (planned fix in v0.1.1).
- `restore` overwrites the target memory/ directory (planned warn/backup in v0.1.1).
- macOS CI is non-blocking (a known git-subprocess flake).
