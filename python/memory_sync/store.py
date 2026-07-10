"""Content store backed by a git repo (the sync-store). Stdlib + subprocess git only."""
import os
import shutil
import subprocess


class GitStore:
    def __init__(self, url: str, work_dir: str):
        self.url = url
        self.work_dir = work_dir
        os.makedirs(self.work_dir, exist_ok=True)
        self._sync()

    def _run(self, *args, cwd=None):
        return subprocess.run(args, cwd=cwd or self.work_dir, check=True, capture_output=True, text=True)

    def _sync(self):
        if os.path.isdir(os.path.join(self.work_dir, ".git")):
            self._run("git", "pull", "--ff-only")
        else:
            # clone (for a bare/local repo) or init then add remote (for empty dir)
            r = subprocess.run(["git", "clone", self.url, self.work_dir], capture_output=True, text=True)
            if r.returncode != 0:
                # fall back: init + set remote (handles non-bare local paths gracefully)
                self._run("git", "init")
                self._run("git", "remote", "add", "origin", self.url)

    def put(self, key: str, bundle_dir: str) -> None:
        dest = os.path.join(self.work_dir, key)
        if os.path.exists(dest):
            shutil.rmtree(dest)
        shutil.copytree(bundle_dir, dest)
        self._run("git", "add", key)
        self._run("git", "-c", "user.name=PAN", "-c", "user.email=pandali.kk@qq.com", "commit", "-m", f"checkpoint {key}")
        self._run("git", "push", "origin", "HEAD")

    def get(self, key: str) -> str:
        self._run("git", "pull", "--ff-only")
        return os.path.join(self.work_dir, key)
