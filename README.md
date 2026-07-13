<div align="center">

# Memory-Sync

**Never lose your Claude Code context across machines. Save on one, resume on another — full history.**

[![CI](https://github.com/pandalee99/Memory-Sync/actions/workflows/ci.yml/badge.svg)](https://github.com/pandalee99/Memory-Sync/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/pandalee99/Memory-Sync?include_prereleases)](https://github.com/pandalee99/Memory-Sync/releases)
[![GitHub stars](https://img.shields.io/github/stars/pandalee99/Memory-Sync?style=social)](https://github.com/pandalee99/Memory-Sync/stargazers)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed?logo=anthropic&logoColor=white)](https://claude.com/claude-code)

A non-invasive adapter on Claude Code's own `--resume` — no fork, no cloud.

</div>

---

## The problem

You've been working with Claude Code for hours on machine A — deep in a task, full context loaded. You switch to machine B, and all of that context is gone. You start over: re-explain, re-paste files, re-build the mental model.

**Memory-Sync fixes this.** Save the session on A, restore on B, resume with the full history — `claude --resume` loads everything, paths auto-rewritten to B's layout.

## Quickstart

In Claude Code:

```
/plugin marketplace add pandalee99/Memory-Sync
/plugin install memory-sync
/memory-sync install      # downloads the binary + asks for a private sync-store git URL
/memory-sync save         # checkpoint the current session → returns a checkpoint id
/memory-sync restore <id> # on machine B (after install) → prints `claude --resume <uuid>`
```

That's it — save on machine A, restore on machine B, resume with full history.

## Features

- **Cross-machine resume** — save a Claude Code session, restore on another machine, resume with full history (paths auto-rewritten to the target layout).
- **Private sync-store** — your sessions live in your own private git repo. No cloud, no third party. The install refuses a public sync-store (transcripts may contain secrets).
- **Auto-detect** — `save` checkpoints the current session automatically (no manual uuid lookup).
- **SHA256-verified binary** — the bootstrap downloads the binary from GitHub Releases and verifies its checksum before executing.
- **Claude Code plugin** — one-command install via the plugin marketplace; `/memory-sync` slash command.
- **Single zero-dep binary** — pure Go, cross-compiled (darwin/linux × amd64/arm64).
- **Gzip compression** for transcripts >1MB; 50/100MB bundle-size gate.

## How it works

```
save             → memory-sync checkpoint → normalize (path-rewrite) → push to your private git sync-store
restore <id>     → pull → rewrite paths to target → claude --resume <uuid> loads full history
```

No fork, no cloud — a non-invasive adapter on Claude Code's own `--resume`. The session uuid is the checkpoint id (cid == uuid).

## Why not just copy the transcript?

You could manually copy `~/.claude/projects/.../<uuid>.jsonl` to machine B. But:

- **Paths break** — they're hard-coded to A's layout (`/Users/you/...`); `--resume` on B loads broken paths.
- **No memory sync** — your `MEMORY.md` + memory files don't transfer.
- **Manual & error-prone** — no checksums, no compression, no automation.

Memory-Sync rewrites the paths to B's layout, syncs your memory dir, and automates the whole flow — with checksum-verified binaries and a private sync-store.

## Status

- **v0.1.0** — install + save + restore + status (stable).
- **Planned** — compact-restore (bounded resume for large sessions on tight-context providers).

See the [CHANGELOG](CHANGELOG.md) + [SECURITY](SECURITY.md).

## Repo structure

- `go/` — the `memory-sync` Go binary (engine: checkpoint/restore/status/install).
- `.claude-plugin/` + `commands/` + `scripts/` — the Claude Code plugin.
- `.github/workflows/` — CI (test/vet/gofmt/race + shellcheck + plugin-manifest validation) + release (cross-compile binaries).
- `.memory-sync.toml.example` — per-project sync config template.

## Docs

- [CONTRIBUTING.md](CONTRIBUTING.md) — dev setup + how to add commands.
- [ARCHITECTURE.md](ARCHITECTURE.md) — the 2-layer model + data flow.
- [CHANGELOG.md](CHANGELOG.md) — release history + known limitations.
- [SECURITY.md](SECURITY.md) — transcript storage + sync-store privacy.
- [LICENSE](LICENSE) — MIT.

---

<div align="center">

**If Memory-Sync saves you context-switching time, [star the repo](https://github.com/pandalee99/Memory-Sync/stargazers) — it helps others find it.**

</div>
