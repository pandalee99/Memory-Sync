<div align="center">

# Memory-Sync

**Synchronize a Claude Code session across machines — resume on a second machine with the full history the first machine learned.**

[![CI](https://github.com/pandalee99/Memory-Sync/actions/workflows/ci.yml/badge.svg)](https://github.com/pandalee99/Memory-Sync/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/pandalee99/Memory-Sync?include_prereleases)](https://github.com/pandalee99/Memory-Sync/releases)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed?logo=anthropic&logoColor=white)](https://claude.com/claude-code)

</div>

---

## Quickstart

In Claude Code:

```
/plugin marketplace add pandalee99/Memory-Sync
/plugin install memory-sync
/memory-sync install
```

`/memory-sync install` downloads the `memory-sync` binary and asks for a **private** sync-store git repo URL (the one piece of setup). Then:

```
/memory-sync save          # checkpoint the current session → returns a checkpoint id
/memory-sync restore <id>  # on another machine (after install) → prints `claude --resume <uuid>`
```

That's it — save on machine A, restore on machine B, resume with full history.

## How it works

`save` → `memory-sync checkpoint` → normalizes the session transcript + memory → pushes to your **private** git sync-store. `restore <id>` → pulls → rewrites paths to the target layout → `claude --resume <uuid>` loads full history. No fork, no cloud (for checkpoint mode) — a non-invasive adapter on Claude Code's own `--resume`.

## Status

- v0.1.0: install + save + restore + status (stable).
- Planned: compact-restore (bounded resume for large sessions).

## Repo structure

- `go/` — the `memory-sync` Go binary (engine: checkpoint/restore/status/install + step0 gate).
- `.claude-plugin/` + `commands/` + `scripts/` — the Claude Code plugin.
- `.github/workflows/` — CI (test/lint/build) + release (cross-compile binaries).
- `.memory-sync.toml.example` — per-project sync config template.

## Docs

- [CONTRIBUTING.md](CONTRIBUTING.md) — dev setup + how to add commands.
- [ARCHITECTURE.md](ARCHITECTURE.md) — the 2-layer model + data flow.
- [CHANGELOG.md](CHANGELOG.md) — release history.
- [SECURITY.md](SECURITY.md) — transcript storage + sync-store privacy.
- [LICENSE](LICENSE) — MIT.
