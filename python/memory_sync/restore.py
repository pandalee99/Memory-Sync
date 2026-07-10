"""Restore a session: pull, rewrite origin->target, write to target project dir."""
import os
import shutil

from . import config as config_mod
from . import encoding
from . import manifest as manifest_mod
from . import rewriter
from . import store as store_mod


def restore(checkpoint_id: str, target_cwd: str, cfg_path: str) -> str:
    cfg = config_mod.load(cfg_path)
    work_dir = os.path.join(os.path.expanduser("~"), ".memory-sync", "work", cfg.project_id)
    s = store_mod.GitStore(cfg.store_url, work_dir)
    bundle = s.get(checkpoint_id)

    with open(os.path.join(bundle, "manifest.json")) as f:
        m = manifest_mod.load(f.read())
    uuid = m.session_uuid
    target_project = encoding.project_dir(target_cwd)
    os.makedirs(target_project, exist_ok=True)

    # transcript: rewrite origin -> target
    src_jsonl = os.path.join(bundle, uuid + ".jsonl")
    if os.path.isfile(src_jsonl):
        dst_jsonl = os.path.join(target_project, uuid + ".jsonl")
        rewriter.rewrite_session(src_jsonl, dst_jsonl, m.origin_root, target_cwd)

    # memory: copy as-is (paths rewritten in bodies is M3's concern; M2 copies raw)
    src_mem = os.path.join(bundle, "memory")
    if os.path.isdir(src_mem):
        dst_mem = os.path.join(target_project, "memory")
        if os.path.exists(dst_mem):
            shutil.rmtree(dst_mem)
        shutil.copytree(src_mem, dst_mem)

    return uuid
