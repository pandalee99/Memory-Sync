# Architecture

Two layers:

1. **Go binary** (`go/cmd/memory-sync`, package `memorysync`) — the engine: `checkpoint` (normalize transcript + memory → push to a private git sync-store), `restore` (pull → rewrite paths → `claude --resume <uuid>`), `status`, `install` (config setup: URL + private/empty/SSH checks + toml write), `step0` (compact-restore gate probe). cid == session uuid (M2-Go).
2. **Plugin** (`.claude-plugin/plugin.json` + `commands/memory-sync.md` + `scripts/memory-sync.sh`) — the UX: the `/memory-sync` slash command bootstraps the binary (curl from GitHub Releases) + delegates. No Go code in the plugin; no hooks (explicit-only save).

## Data flow

`/memory-sync save` → script → `memory-sync checkpoint` (session-uuid via **auto-detect** of the newest `.jsonl` in `ProjectDir(cwd)`, or `$CLAUDE_CODE_SESSION_ID` if present) → `GitStore.Put` (git commit/push to the private sync-store) → returns the checkpoint id (= session uuid). `/memory-sync restore <id>` → `memory-sync restore` → `GitStore.Get` (pull) → `RewriteSession` (origin→target paths) → writes `~/.claude/projects/<dashed-cwd>/<uuid>.jsonl` → prints `claude --resume <uuid>`.

## Sync-store

A **private** git repo holding plaintext session checkpoints (decision ②A — age-encryption is a future hardening). The user provides the URL via `/memory-sync install`.

## Compact-restore (next milestone)

Compact-restore (bounded resume for large sessions on tight context providers): planned for a later release.
