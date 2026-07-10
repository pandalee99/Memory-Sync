#!/usr/bin/env bash
# memory-sync plugin bootstrap: ensure the binary exists, then delegate to it.
set -euo pipefail

BIN="$HOME/.memory-sync/bin/memory-sync"

# bootstrap: download the binary from GitHub Releases if missing
if [ ! -x "$BIN" ]; then
	OS="$(uname -s | tr '[:upper:]' '[:lower:]')"        # darwin | linux
	ARCH="$(uname -m)"
	case "$ARCH" in
		x86_64) ARCH=amd64 ;;
		aarch64|arm64) ARCH=arm64 ;;
		*) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
	esac
	URL="https://github.com/pandalee99/Memory-Sync/releases/latest/download/memory-sync-${OS}-${ARCH}"
	CHECKSUMS_URL="https://github.com/pandalee99/Memory-Sync/releases/latest/download/checksums.txt"
	EXPECTED_ASSET="memory-sync-${OS}-${ARCH}"
	mkdir -p "$(dirname "$BIN")"
	echo "downloading memory-sync ${OS}-${ARCH}..." >&2
	curl -fsSL -o "$BIN" "$URL"
	# supply-chain: verify the binary's SHA256 against the published checksums before executing it.
	# Do NOT chmod+x an unverified binary; on mismatch/missing entry remove it and abort.
	CHECKSUMS="$(mktemp)"
	if ! curl -fsSL -o "$CHECKSUMS" "$CHECKSUMS_URL"; then
		echo "error: failed to download checksums.txt — refusing to run an unverified binary" >&2
		rm -f "$CHECKSUMS" "$BIN"
		exit 1
	fi
	ACTUAL="$(shasum -a 256 "$BIN" | awk '{print $1}')"
	# sha256sum line format: "<hash>  <filename>" (text) or "<hash> *<filename>" (binary).
	EXPECTED="$(awk -v asset="$EXPECTED_ASSET" '{ fn=$2; sub(/^\*/, "", fn); if (fn == asset) print $1 }' "$CHECKSUMS")"
	rm -f "$CHECKSUMS"
	if [ -z "$EXPECTED" ] || [ "$ACTUAL" != "$EXPECTED" ]; then
		echo "error: SHA256 verification failed for $EXPECTED_ASSET" >&2
		if [ -z "$EXPECTED" ]; then
			echo "  no matching entry in checksums.txt for $EXPECTED_ASSET" >&2
		else
			echo "  expected: $EXPECTED" >&2
		fi
		echo "  actual:   $ACTUAL" >&2
		rm -f "$BIN"
		exit 1
	fi
	echo "checksum verified: $ACTUAL  $EXPECTED_ASSET" >&2
	chmod +x "$BIN"
fi

sub="${1:-status}"
[ $# -gt 0 ] && shift || true

# map user-facing subcommands to the binary; pass --cwd for checkpoint/restore
case "$sub" in
	save)     exec "$BIN" checkpoint --cwd "${CLAUDE_PROJECT_DIR:-$PWD}" "$@" ;;
	restore)  exec "$BIN" restore "$1" --cwd "${CLAUDE_PROJECT_DIR:-$PWD}" ;;
	install)  exec "$BIN" install "$@" ;;
	status)   exec "$BIN" status ;;
	*)        echo "usage: /memory-sync [install|save|restore <id>|status]" >&2; exit 1 ;;
esac
