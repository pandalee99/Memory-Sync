"""memory-sync CLI: checkpoint | restore | status."""
import argparse
import os
import sys

from . import checkpoint as checkpoint_mod
from . import restore as restore_mod


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="memory-sync")
    sub = p.add_subparsers(dest="cmd", required=True)

    pc = sub.add_parser("checkpoint", help="upload the current session to the sync-store")
    pc.add_argument("session_uuid")
    pc.add_argument("--cwd", default=os.getcwd())
    pc.add_argument("--config", default=".memory-sync.toml")

    pr = sub.add_parser("restore", help="restore a session to this machine, then claude --resume")
    pr.add_argument("checkpoint_id")
    pr.add_argument("--cwd", default=os.getcwd())
    pr.add_argument("--config", default=".memory-sync.toml")

    sub.add_parser("status", help="show sync status")

    args = p.parse_args(argv)
    if args.cmd == "checkpoint":
        cid = checkpoint_mod.checkpoint(args.session_uuid, args.cwd, args.config)
        print(cid)
        return 0
    if args.cmd == "restore":
        uuid = restore_mod.restore(args.checkpoint_id, args.cwd, args.config)
        print(f"claude --resume {uuid}")
        return 0
    if args.cmd == "status":
        print("memory-sync v0.1.0 (unencrypted)")
        return 0
    return 1


if __name__ == "__main__":
    sys.exit(main())
