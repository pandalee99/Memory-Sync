"""Checkpoint a session: read transcript+memory, store, write manifest, push."""
import os
import shutil

from . import config as config_mod
from . import encoding
from . import manifest
from . import store as store_mod


def checkpoint(session_uuid: str, src_cwd: str, cfg_path: str) -> str:
    cfg = config_mod.load(cfg_path)
    src_project = encoding.project_dir(src_cwd)
    work_dir = os.path.join(os.path.expanduser("~"), ".memory-sync", "work", cfg.project_id)
    s = store_mod.GitStore(cfg.store_url, work_dir)

    # build a temp bundle dir
    import tempfile
    bundle = tempfile.mkdtemp(prefix="msync-bundle-")
    try:
        # transcript
        src_jsonl = os.path.join(src_project, session_uuid + ".jsonl")
        if os.path.isfile(src_jsonl):
            shutil.copy2(src_jsonl, os.path.join(bundle, session_uuid + ".jsonl"))
        # memory
        src_mem = os.path.join(src_project, "memory")
        if os.path.isdir(src_mem):
            shutil.copytree(src_mem, os.path.join(bundle, "memory"))
        # manifest
        m = manifest.Manifest(
            project_id=cfg.project_id,
            origin_root=src_cwd,
            origin_dashed=encoding.dashed(src_cwd),
            session_uuid=session_uuid,
            git_head=_git_head(src_cwd),
        )
        with open(os.path.join(bundle, "manifest.json"), "w") as f:
            f.write(manifest.dump(m))
        s.put(session_uuid, bundle)
        return session_uuid
    finally:
        shutil.rmtree(bundle, ignore_errors=True)


def _git_head(cwd: str) -> str:
    import subprocess
    try:
        r = subprocess.run(["git", "-C", cwd, "rev-parse", "HEAD"], capture_output=True, text=True)
        return r.stdout.strip() if r.returncode == 0 else ""
    except Exception:
        return ""
