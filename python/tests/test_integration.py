import os
import shutil
import subprocess
import pytest

from memory_sync import cli, encoding


def _have_claude():
    return shutil.which("claude") is not None


pytestmark = pytest.mark.skipif(
    not os.environ.get("MSYNC_INTEGRATION") or not _have_claude(),
    reason="set MSYNC_INTEGRATION=1 and have `claude` on PATH to run the live integration test",
)


def _bare(tmp_path):
    repo = tmp_path / "ss.git"
    subprocess.run(["git", "init", "--bare", str(repo)], check=True, capture_output=True)
    return repo


def _toml(tmp_path, store_url):
    t = tmp_path / ".memory-sync.toml"
    t.write_text(f'project_id = "p"\n[store]\nbackend = "git"\nurl = "{store_url}"\n')
    return str(t)


def _create_session(cwd, facts):
    r = subprocess.run(
        ["claude", "-p", facts + " Reply only with the word OK."],
        cwd=cwd, capture_output=True, text=True,
    )
    assert r.returncode == 0, r.stderr
    # find the session uuid (newest .jsonl in the project dir)
    proj = encoding.project_dir(cwd)
    newest = sorted(
        (f for f in os.listdir(proj) if f.endswith(".jsonl")),
        key=lambda f: os.path.getmtime(os.path.join(proj, f)), reverse=True,
    )[0]
    return newest[:-6]  # strip .jsonl


def _carry_claude_config(real_claude, home):
    """Carry claude's auth/model config into the isolated HOME.

    monkeypatching HOME isolates ~/.claude/projects (the test's goal) but also
    hides ~/.claude/settings.json, where claude's working auth (env block: base
    URL, API key, model) typically lives. Without it, `claude -p` falls back to
    whatever the parent shell's env vars say, which may point at an unreachable
    model. Copy just the auth/config files so claude authenticates exactly as it
    does interactively, while session project dirs stay isolated under home.
    """
    if not os.path.isdir(real_claude):
        return
    dst = home / ".claude"; dst.mkdir(exist_ok=True)
    for name in ("settings.json", "settings.local.json", ".credentials.json"):
        src = os.path.join(real_claude, name)
        if os.path.isfile(src):
            shutil.copy2(src, dst / name)


def test_step0_create_checkpoint_restore_resume(tmp_path, monkeypatch):
    real_claude = os.path.join(os.path.expanduser("~"), ".claude")
    home = tmp_path / "home"; home.mkdir(); monkeypatch.setenv("HOME", str(home))
    _carry_claude_config(real_claude, home)
    origin = tmp_path / "origin" / "h100"; origin.mkdir(parents=True)
    target = tmp_path / "target" / "h100"; target.mkdir(parents=True)
    subprocess.run(["git", "init", str(origin)], check=True, capture_output=True)
    subprocess.run(["git", "init", str(target)], check=True, capture_output=True)
    repo = _bare(tmp_path); cfg = _toml(tmp_path, str(repo))

    facts = ("Remember these two facts. Fact 1: the deployment token is TOKEN-FAKE-XYZ-999. "
             "Fact 2: the config file path is " + str(origin) + "/deploy/config.yaml.")
    uuid = _create_session(str(origin), facts)

    cli.main(["checkpoint", uuid, "--cwd", str(origin), "--config", cfg])
    cli.main(["restore", uuid, "--cwd", str(target), "--config", cfg])

    r = subprocess.run(
        ["claude", "--resume", uuid, "-p",
         "What two facts did I tell you to remember? Quote them exactly, one per line."],
        cwd=str(target), capture_output=True, text=True,
    )
    assert r.returncode == 0, r.stderr
    out = r.stdout
    assert "TOKEN-FAKE-XYZ-999" in out
    # the path must come back in TARGET form (proves rewrite + resume + history load)
    assert str(target) + "/deploy/config.yaml" in out
    assert str(origin) not in out
