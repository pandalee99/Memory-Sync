import json, os, subprocess
from memory_sync import checkpoint, config, restore, encoding


def _bare(tmp_path):
    repo = tmp_path / "ss.git"
    subprocess.run(["git", "init", "--bare", str(repo)], check=True, capture_output=True)
    return repo


def _toml(tmp_path, store_url):
    t = tmp_path / ".memory-sync.toml"
    t.write_text(f'project_id = "p"\n[store]\nbackend = "git"\nurl = "{store_url}"\n')
    return str(t)


def test_restore_rewrites_paths_to_target(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    monkeypatch.setenv("HOME", str(home))
    src_cwd = str(tmp_path / "origin" / "h100")
    os.makedirs(src_cwd)
    target_cwd = str(tmp_path / "target" / "h100")
    os.makedirs(target_cwd)
    uuid = "11111111-2222-3333-4444-555555555555"
    # checkpoint a session (reuse checkpoint)
    proj = encoding.project_dir(src_cwd)
    os.makedirs(os.path.join(proj, "memory"), exist_ok=True)
    open(os.path.join(proj, uuid + ".jsonl"), "w").write(
        '{"type":"user","uuid":"u1","parentUuid":null,"cwd":"' + src_cwd + '","message":{"content":[{"type":"text","text":"see ' + src_cwd + '/x"}]}}\n')
    repo = _bare(tmp_path)
    cfg_path = _toml(tmp_path, str(repo))
    checkpoint.checkpoint(uuid, src_cwd, cfg_path)

    uuid2 = restore.restore(uuid, target_cwd, cfg_path)
    assert uuid2 == uuid
    # target project dir has the rewritten transcript
    tproj = encoding.project_dir(target_cwd)
    out = open(os.path.join(tproj, uuid + ".jsonl")).read()
    assert src_cwd not in out
    assert target_cwd in out
