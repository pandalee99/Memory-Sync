import os
from memory_sync import encoding

# Use endswith (not ==) for path-segment checks so tests are realpath-agnostic and
# cross-platform: macOS resolves some roots (/home, /tmp) via firmlinks, adding a
# prefix to realpath; the segment encoding (incl. '_' -> '-') is still verified.


def test_dashed_basic():
    assert encoding.dashed("/home/alice/code/h100").endswith("-home-alice-code-h100")


def test_dashed_underscore_becomes_hyphen():
    # The non-obvious rule discovered in Step 0: '_' -> '-' (not just '/' -> '-').
    assert encoding.dashed("/home/alice/msync_origin/h100").endswith("-home-alice-msync-origin-h100")


def test_dashed_dots_and_spaces():
    assert encoding.dashed("/home/alice/my proj/v1.2").endswith("-home-alice-my-proj-v1-2")


def test_dashed_tmp_uses_realpath():
    # macOS /tmp -> /private/tmp: realpath must be resolved BEFORE encoding.
    real = os.path.realpath("/tmp/fakehome/code/h100")
    assert encoding.dashed("/tmp/fakehome/code/h100") == real.replace("/", "-")


def test_project_dir_path():
    d = encoding.project_dir("/home/alice/code/h100")
    assert d.startswith(os.path.expanduser("~") + "/.claude/projects/")
    assert d.endswith("-home-alice-code-h100")
