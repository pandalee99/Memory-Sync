"""Claude Code project-dir encoding (Step 0 verified, 2026-07-07)."""
import os
import re


def dashed(cwd: str) -> str:
    """Encode a cwd to Claude Code's project-dir name.

    Rule (verified): resolve realpath (macOS /tmp -> /private/tmp), then replace
    EVERY non-alphanumeric char (including '/' AND '_') with '-'. Case preserved.
    NOT just '/' -> '-'.
    """
    return re.sub(r"[^a-zA-Z0-9]", "-", os.path.realpath(cwd))


def project_dir(cwd: str) -> str:
    """The on-disk project directory path for a cwd."""
    return os.path.join(os.path.expanduser("~"), ".claude", "projects", dashed(cwd))
