import json, os, subprocess
from memory_sync import checkpoint, config


def _bare(tmp_path):
    repo = tmp_path / "ss.git"
    subprocess.run(["git", "init", "--bare", str(repo)], check=True, capture_output=True)
    return repo


def _toml(tmp_path, store_url):
    t = tmp_path / ".memory-sync.toml"
    t.write_text(f'project_id = "p"\n[store]\nbackend = "git"\nurl = "{store_url}"\n')
    return str(t)


def _make_session(tmp_path, src_cwd, uuid):
    # a fake project dir with a transcript + memory
    import os
    proj = os.path.join(os.path.expanduser("~"), ".claude", "projects", _dashed(src_cwd))
    os.makedirs(os.path.join(proj, "memory"), exist_ok=True)
    with open(os.path.join(proj, uuid + ".jsonl"), "w") as f:
        f.write('{"type":"user","uuid":"u1","parentUuid":null,"cwd":"' + src_cwd + '","message":{"content":[{"type":"text","text":"working in ' + src_cwd + '"}]}}\n')
    with open(os.path.join(proj, "memory", "fact.md"), "w") as f:
        f.write("# a learned fact\n")


def _dashed(cwd):
    import re, os
    return re.sub(r"[^a-zA-Z0-9]", "-", os.path.realpath(cwd))


def test_checkpoint_stores_transcript_memory_and_manifest(tmp_path, monkeypatch):
    # point HOME at a temp dir so we control ~/.claude/projects
    home = tmp_path / "home"
    home.mkdir()
    monkeypatch.setenv("HOME", str(home))
    src_cwd = str(tmp_path / "origin" / "h100")
    os.makedirs(src_cwd)
    uuid = "11111111-2222-3333-4444-555555555555"
    _make_session(tmp_path, src_cwd, uuid)
    repo = _bare(tmp_path)
    work = tmp_path / "work"
    cfg_path = _toml(tmp_path, str(repo))
    cid = checkpoint.checkpoint(uuid, src_cwd, cfg_path)
    assert cid == uuid
    # the store now has a dir `uuid/` with transcript + memory + manifest
    from memory_sync.store import GitStore
    s = GitStore(str(repo), str(work))
    got = s.get(uuid)
    assert os.path.isfile(os.path.join(got, uuid + ".jsonl"))
    assert os.path.isfile(os.path.join(got, "memory", "fact.md"))
    m = json.load(open(os.path.join(got, "manifest.json")))
    assert m["session_uuid"] == uuid
    assert m["origin_root"] == src_cwd
