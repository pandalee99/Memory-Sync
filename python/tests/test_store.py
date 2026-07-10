import os, subprocess
from memory_sync import store


def _bare_repo(tmp_path):
    repo = tmp_path / "sync-store.git"
    subprocess.run(["git", "init", "--bare", str(repo)], check=True, capture_output=True)
    return repo


def test_gitstore_put_get_roundtrip(tmp_path):
    repo = _bare_repo(tmp_path)
    work = tmp_path / "work"
    s = store.GitStore(str(repo), str(work))
    bundle = tmp_path / "bundle"
    bundle.mkdir()
    (bundle / "transcript.jsonl").write_text('{"type":"user","text":"hi"}\n')
    (bundle / "manifest.json").write_text('{"project_id":"p"}')
    s.put("ckpt-1", str(bundle))
    # a fresh working copy sees it
    work2 = tmp_path / "work2"
    s2 = store.GitStore(str(repo), str(work2))
    got = s2.get("ckpt-1")
    assert os.path.isfile(os.path.join(got, "transcript.jsonl"))
    assert os.path.isfile(os.path.join(got, "manifest.json"))
    assert open(os.path.join(got, "transcript.jsonl")).read() == '{"type":"user","text":"hi"}\n'
