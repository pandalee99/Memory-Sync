---
description: Checkpoint, restore, or install Memory-Sync (session memory across machines)
argument-hint: [install|save|restore <id>|status]
allowed-tools: Bash(*), Read, Write, Edit
---

## Memory-Sync

The user invoked `/memory-sync $ARGUMENTS`.

Run the bootstrap script — it ensures the `memory-sync` binary exists at `~/.memory-sync/bin/memory-sync` (downloads it from GitHub Releases on first run), then delegates to it:

!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/memory-sync.sh $ARGUMENTS`

Follow the script's stdout/stderr:
- **install**: if no sync-store URL is configured, ask the user for a **private** git repo URL (public has risk — warn, don't block). Confirm private (preferred) + empty (first-time preferred). Then run `memory-sync install --url <url>`.
- **save**: report the returned checkpoint id + the resume recipe: "On another machine (after `/memory-sync install`): `/memory-sync restore <id>` → `claude --resume <uuid>`."
- **restore <id>**: report the printed `claude --resume <uuid>` line.
- **status**: report the binary version + config.
