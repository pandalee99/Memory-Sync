"""Path rewriting + event taxonomy for Claude Code session JSONL."""
import json
import os
from dataclasses import dataclass

from . import encoding

# Event types with no `uuid` and not in the parentUuid DAG (Step 0 verified:
# 0 orphans; stripping them cannot break the resume chain).
STRIP_TYPES = {"file-history-snapshot", "queue-operation"}


def build_subs(src_cwd: str, dst_cwd: str) -> list[tuple[str, str]]:
    """Ordered origin->target path substitutions.

    Most-specific first; the bare dashed project-dir name is last as a catch-all.
    M2 uses direct same-HOME substitution (HOME is the SAME on both sides);
    cross-HOME logical-placeholder resolution is deferred to the Go port.
    """
    home = os.path.expanduser("~")
    src_d = encoding.dashed(src_cwd)
    dst_d = encoding.dashed(dst_cwd)
    return [
        (os.path.join(home, ".claude", "projects", src_d),
         os.path.join(home, ".claude", "projects", dst_d)),
        (os.path.realpath(src_cwd), os.path.realpath(dst_cwd)),
        (src_cwd, dst_cwd),
        (src_d, dst_d),
    ]


def rewrite_str(s, subs) -> str:
    if not isinstance(s, str):
        return s
    for a, b in subs:
        if a and a in s:
            s = s.replace(a, b)
    return s


def rewrite_obj(o, subs):
    if isinstance(o, str):
        return rewrite_str(o, subs)
    if isinstance(o, list):
        return [rewrite_obj(x, subs) for x in o]
    if isinstance(o, dict):
        return {k: rewrite_obj(v, subs) for k, v in o.items()}
    return o


@dataclass
class RewriteStats:
    kept: int = 0
    stripped: int = 0
    orphans: int = 0
    residue_src_cwd: int = 0
    residue_src_dashed: int = 0
    malformed: int = 0


def rewrite_session(src_jsonl: str, dst_jsonl: str, src_cwd: str, dst_cwd: str) -> RewriteStats:
    """Read src JSONL, strip machine-local events, rewrite paths, write dst JSONL.

    Returns stats incl. orphan-parentUuid and residue checks (the Step 0 invariants).
    Does NOT copy memory/tool-results sideload — that is checkpoint.py's job in M2.
    """
    subs = build_subs(src_cwd, dst_cwd)
    src_d = encoding.dashed(src_cwd)
    dst_d = encoding.dashed(dst_cwd)
    stats = RewriteStats()
    out_objs = []
    with open(src_jsonl) as f, open(dst_jsonl, "w") as out:
        for line in f:
            line = line.rstrip("\n")
            if not line.strip():
                continue
            try:
                o = json.loads(line)
            except json.JSONDecodeError:
                stats.malformed += 1
                continue
            t = o.get("type", "?")
            if t in STRIP_TYPES:
                stats.stripped += 1
                continue
            o = rewrite_obj(o, subs)
            out.write(json.dumps(o, ensure_ascii=False) + "\n")
            out_objs.append(o)
            stats.kept += 1

    uuids = {o.get("uuid") for o in out_objs if isinstance(o, dict) and o.get("uuid")}
    for o in out_objs:
        if isinstance(o, dict):
            p = o.get("parentUuid")
            if p is not None and p not in uuids:
                stats.orphans += 1

    with open(dst_jsonl) as f:
        for line in f:
            # residue checks: count src NOT covered by a dst span (avoids
            # false-positive when dst contains src as a substring)
            if src_cwd in line:
                masked = line.replace(dst_cwd, "\x00" * len(dst_cwd)) if dst_cwd and src_cwd in dst_cwd else line
                if src_cwd in masked:
                    stats.residue_src_cwd += 1
            if src_d in line:
                masked = line.replace(dst_d, "\x00" * len(dst_d)) if dst_d and src_d in dst_d else line
                if src_d in masked:
                    stats.residue_src_dashed += 1

    return stats
