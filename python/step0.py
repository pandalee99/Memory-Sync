#!/usr/bin/env python3
"""Step 0 — make-or-break: rewrite a Claude Code session to a target layout + static-verify.

Usage: step0.py <session_uuid> <src_cwd> <dst_cwd>

Rewrites the session JSONL (and memory + tool-results sideload) from the
src project dir into the dst project dir (dashed names derived from
src_cwd/dst_cwd by Claude's rule: replace every '/' with '-'). Rewrites
absolute paths, strips machine-local event types (file-history-snapshot,
queue-operation), and static-verifies: valid JSON, 0 orphan parentUuid,
no unrewritten origin paths, tool-results refs resolve.

Local-only; no secret redaction yet (Step 3). The live `claude --resume`
probe is run separately.
"""
import json
import os
import re
import shutil
import sys
import collections

HOME = os.path.expanduser("~")
PROJECTS = os.path.join(HOME, ".claude", "projects")
STRIP_TYPES = {"file-history-snapshot", "queue-operation"}


def dashed(cwd: str) -> str:
    # Claude's project-dir encoding (empirically verified Step 0): resolve realpath
    # (macOS /tmp -> /private/tmp), then replace EVERY non-alphanumeric char
    # (including '/' AND '_') with '-'. Case preserved. Not just '/' -> '-'.
    return re.sub(r"[^a-zA-Z0-9]", "-", os.path.realpath(cwd))


def rewrite_str(s, subs):
    if not isinstance(s, str):
        return s
    for a, b in subs:
        if a in s:
            s = s.replace(a, b)
    return s


def rewrite_obj(o, subs):
    if isinstance(o, str):
        return rewrite_str(o, subs)
    if isinstance(o, list):
        return [rewrite_obj(x, subs) for x in o]
    if isinstance(o, dict):
        return {k: rewrite_obj(v, subs) for k, v in o.items()}  # keys are field names, never paths
    return o


def main(uuid, src_cwd, dst_cwd):
    src_d = dashed(src_cwd)
    dst_d = dashed(dst_cwd)
    src_dir = os.path.join(PROJECTS, src_d)
    dst_dir = os.path.join(PROJECTS, dst_d)
    src_jsonl = os.path.join(src_dir, uuid + ".jsonl")
    dst_jsonl = os.path.join(dst_dir, uuid + ".jsonl")
    assert os.path.exists(src_jsonl), f"missing source: {src_jsonl}"

    # substitutions: most-specific first; bare dashed name last (catch-all)
    subs = [
        (os.path.join(HOME, ".claude", "projects", src_d),
         os.path.join(HOME, ".claude", "projects", dst_d)),
        (src_cwd, dst_cwd),
        (src_d, dst_d),
    ]

    if dst_d != src_d and os.path.exists(dst_dir):
        shutil.rmtree(dst_dir)
    os.makedirs(os.path.join(dst_dir, uuid, "tool-results"), exist_ok=True)
    os.makedirs(os.path.join(dst_dir, "memory"), exist_ok=True)

    kept = stripped = 0
    tk = collections.Counter()
    ts = collections.Counter()
    out_objs = []
    with open(src_jsonl) as f, open(dst_jsonl, "w") as out:
        for line in f:
            line = line.rstrip("\n")
            if not line.strip():
                continue
            try:
                o = json.loads(line)
            except json.JSONDecodeError as e:
                print("PARSE FAIL:", e)
                continue
            t = o.get("type", "?")
            if t in STRIP_TYPES:
                stripped += 1
                ts[t] += 1
                continue
            o = rewrite_obj(o, subs)
            out.write(json.dumps(o, ensure_ascii=False) + "\n")
            out_objs.append(o)
            kept += 1
            tk[t] += 1

    # copy memory files (rewrite body paths)
    mem_n = 0
    src_mem = os.path.join(src_dir, "memory")
    dst_mem = os.path.join(dst_dir, "memory")
    if os.path.isdir(src_mem):
        for fn in os.listdir(src_mem):
            p = os.path.join(src_mem, fn)
            if not os.path.isfile(p):
                continue
            open(os.path.join(dst_mem, fn), "w").write(rewrite_str(open(p).read(), subs))
            mem_n += 1

    # copy tool-results sideload tree
    tr_n = 0
    src_tr = os.path.join(src_dir, uuid, "tool-results")
    dst_tr = os.path.join(dst_dir, uuid, "tool-results")
    if os.path.isdir(src_tr):
        for root, _dirs, files in os.walk(src_tr):
            rel = os.path.relpath(root, src_tr)
            tod = dst_tr if rel == "." else os.path.join(dst_tr, rel)
            os.makedirs(tod, exist_ok=True)
            for fn in files:
                shutil.copy2(os.path.join(root, fn), os.path.join(tod, fn))
                tr_n += 1

    print(f"\n=== {uuid} ===")
    print(f"src_dashed={src_d}  dst_dashed={dst_d}")
    print(f"kept={kept} stripped={stripped} memory_files={mem_n} tool_result_files={tr_n}")
    print("stripped:", dict(ts))
    print("kept:", dict(tk))

    # 1+2. valid JSON (re-parsed) + parentUuid orphans
    uuids = {o.get("uuid") for o in out_objs if isinstance(o, dict) and o.get("uuid")}
    orphans = [o.get("parentUuid") for o in out_objs if isinstance(o, dict)
               and o.get("parentUuid") is not None and o.get("parentUuid") not in uuids]
    print(f"orphan parentUuid refs: {len(orphans)} (expect 0)")

    # 3. no unrewritten origin paths
    rp = rd = 0
    with open(dst_jsonl) as f:
        for line in f:
            if src_cwd in line:
                rp += 1
            if src_d in line:
                rd += 1
    print(f"residue '{src_cwd}': {rp} (expect 0)")
    print(f"residue '{src_d}': {rd} (expect 0)")

    # 4. tool-results referenced paths resolve on disk
    marker = os.path.join(dst_dir, uuid, "tool-results")
    pat = re.compile(re.escape(marker) + r"[^\s\"'\\)]*")
    refs = collections.Counter()
    missing = []
    for o in out_objs:
        for m in pat.finditer(json.dumps(o, ensure_ascii=False)):
            ref = m.group(0)
            refs[ref] += 1
            if not os.path.exists(ref):
                missing.append(ref)
    print(f"tool-results refs: {sum(refs.values())} total / {len(refs)} unique; missing on disk: {len(missing)}")
    if missing:
        print("  missing:", missing[:5])

    ok = not orphans and rp == 0 and rd == 0 and not missing
    print("STATIC VERDICT:", "PASS" if ok else "FAIL")
    print("dst:", dst_jsonl)
    return ok


if __name__ == "__main__":
    if len(sys.argv) < 4:
        print("usage: step0.py <session_uuid> <src_cwd> <dst_cwd>", file=sys.stderr)
        sys.exit(2)
    sys.exit(0 if main(sys.argv[1], sys.argv[2], sys.argv[3]) else 1)
