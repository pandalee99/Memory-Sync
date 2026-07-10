import os, subprocess
from memory_sync import cli, checkpoint, config


def _bare(tmp_path):
    repo = tmp_path / "ss.git"
    subprocess.run(["git", "init", "--bare", str(repo)], check=True, capture_output=True)
    return repo


def _toml(tmp_path, store_url):
    t = tmp_path / ".memory-sync.toml"
    t.write_text(f'project_id = "p"\n[store]\nbackend = "git"\nurl = "{store_url}"\n')
    return str(t)


def test_cli_checkpoint_then_restore_prints_resume_command(tmp_path, monkeypatch, capsys):
    home = tmp_path / "home"; home.mkdir()
    monkeypatch.setenv("HOME", str(home))
    from memory_sync import encoding
    src_cwd = str(tmp_path / "o" / "h100"); os.makedirs(src_cwd)
    tgt_cwd = str(tmp_path / "t" / "h100"); os.makedirs(tgt_cwd)
    uuid = "11111111-2222-3333-4444-555555555555"
    proj = encoding.project_dir(src_cwd); os.makedirs(proj, exist_ok=True)
    open(os.path.join(proj, uuid + ".jsonl"), "w").write('{"type":"user","uuid":"u1","parentUuid":null}\n')
    repo = _bare(tmp_path); cfg = _toml(tmp_path, str(repo))
    cli.main(["checkpoint", uuid, "--cwd", src_cwd, "--config", cfg])
    out1 = capsys.readouterr().out
    assert uuid in out1
    cli.main(["restore", uuid, "--cwd", tgt_cwd, "--config", cfg])
    out2 = capsys.readouterr().out
    assert "claude --resume" in out2
    assert uuid in out2
