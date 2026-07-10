from memory_sync import encoding, rewriter

# Compute dashed names via encoding.dashed() (not hardcoded strings) so tests are
# cross-platform: macOS resolves /home via a firmlink, adding a realpath prefix.


def test_build_subs_orders_most_specific_first():
    subs = rewriter.build_subs("/home/alice/code/h100", "/home/alice/msync_test/h100")
    assert ".claude/projects" in subs[0][0]
    # bare dashed name is last (catch-all)
    assert subs[-1] == (encoding.dashed("/home/alice/code/h100"), encoding.dashed("/home/alice/msync_test/h100"))


def test_rewrite_str_replaces_paths():
    subs = [("/home/alice/code/h100", "/home/bob/h100")]
    assert rewriter.rewrite_str("see /home/alice/code/h100/src/main.py", subs) == "see /home/bob/h100/src/main.py"


def test_rewrite_str_handles_underscore_via_dashed():
    src, dst = "/home/alice/msync_origin/h100", "/home/alice/msync_test/h100"
    subs = rewriter.build_subs(src, dst)
    src_d = encoding.dashed(src)  # underscore -> hyphen via the encoding rule
    dst_d = encoding.dashed(dst)
    out = rewriter.rewrite_str(f"project dir {src_d} done", subs)
    assert src_d not in out
    assert dst_d in out


def test_rewrite_obj_recurses_into_dict_and_list():
    subs = [("/home/alice/code/h100", "/home/bob/h100")]
    o = {"cwd": "/home/alice/code/h100", "msg": {"text": ["a", "/home/alice/code/h100/x"]}}
    r = rewriter.rewrite_obj(o, subs)
    assert r == {"cwd": "/home/bob/h100", "msg": {"text": ["a", "/home/bob/h100/x"]}}


def test_rewrite_obj_preserves_non_strings():
    subs = [("/a", "/b")]
    assert rewriter.rewrite_obj(42, subs) == 42
    assert rewriter.rewrite_obj(None, subs) is None
    assert rewriter.rewrite_obj(True, subs) is True


def test_strip_types():
    assert "file-history-snapshot" in rewriter.STRIP_TYPES
    assert "queue-operation" in rewriter.STRIP_TYPES
    assert "user" not in rewriter.STRIP_TYPES


import json
import os
import pytest

FIXTURE = os.path.join(os.path.dirname(__file__), "fixtures", "synthetic_session.json")


def test_rewrite_session_synthetic_fixture(tmp_path):
    events = json.load(open(FIXTURE))
    src = tmp_path / "src.jsonl"
    with open(src, "w") as f:
        for e in events:
            f.write(json.dumps(e) + "\n")
    dst = tmp_path / "out.jsonl"
    stats = rewriter.rewrite_session(str(src), str(dst), "/home/alice/code/h100", "/home/alice/msync_test/h100")
    assert stats.kept == 3
    assert stats.stripped == 2
    assert stats.orphans == 0
    assert stats.residue_src_cwd == 0
    assert stats.residue_src_dashed == 0
    out = open(dst).read()
    assert "/home/alice/code/h100" not in out
    assert "/home/alice/msync_test/h100" in out
    for line in open(dst):
        if line.strip():
            json.loads(line)


def test_rewrite_session_real_corpus_smoke(golden_corpus, tmp_path):
    # derive src_cwd from the corpus's first cwd event — no hardcoded path
    src_cwd = None
    for line in open(golden_corpus):
        o = json.loads(line)
        if o.get("cwd"):
            src_cwd = o["cwd"]
            break
    if not src_cwd:
        pytest.skip("no cwd in corpus")
    dst = tmp_path / "out.jsonl"
    stats = rewriter.rewrite_session(golden_corpus, str(dst), src_cwd, "/home/dev/msync_test/proj")
    assert stats.orphans == 0
    assert stats.residue_src_cwd == 0
    assert stats.residue_src_dashed == 0


def test_rewrite_session_residue_no_false_positive_when_dst_contains_src(tmp_path):
    # src_dashed is a substring of dst_dashed (-code contained in -codes); residue must be 0
    src = tmp_path / "src.jsonl"
    src.write_text('{"type":"user","uuid":"u1","parentUuid":null,"cwd":"/home/alice/code","message":{"content":[{"type":"text","text":"see /home/alice/code/x"}]}}\n')
    dst = tmp_path / "out.jsonl"
    stats = rewriter.rewrite_session(str(src), str(dst), "/home/alice/code", "/home/alice/codes")
    assert stats.residue_src_dashed == 0  # would be 1 with the buggy substring check
    assert stats.residue_src_cwd == 0


def test_rewrite_session_residue_dashed_no_false_positive_when_dst_contains_src(tmp_path):
    # exercise the DASHED-form masking: a JSONL mentioning the src dashed name, where
    # dst_d contains src_d as a substring; residue_src_dashed must be 0
    from memory_sync import encoding
    src_cwd = "/home/alice/code"
    dst_cwd = "/home/alice/codes"
    src_d = encoding.dashed(src_cwd)
    dst_d = encoding.dashed(dst_cwd)
    line = '{"type":"user","uuid":"u1","parentUuid":null,"message":{"content":[{"type":"text","text":"see ' + src_d + '/foo"}]}}'
    src = tmp_path / "src.jsonl"
    src.write_text(line + "\n")
    dst = tmp_path / "out.jsonl"
    stats = rewriter.rewrite_session(str(src), str(dst), src_cwd, dst_cwd)
    assert stats.residue_src_dashed == 0
    assert stats.residue_src_cwd == 0
    assert dst_d in open(dst).read()  # the rewrite happened (src_d -> dst_d)


def test_rewrite_session_counts_malformed_lines(tmp_path):
    src = tmp_path / "src.jsonl"
    src.write_text(
        '{"type":"user","uuid":"u1","parentUuid":null,"message":{"content":[{"type":"text","text":"ok"}]}}\n'
        'THIS IS NOT JSON\n'
        '{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"content":[{"type":"text","text":"hi"}]}}\n'
    )
    dst = tmp_path / "out.jsonl"
    stats = rewriter.rewrite_session(str(src), str(dst), "/home/alice/code/h100", "/home/alice/msync_test/h100")
    assert stats.malformed == 1
    assert stats.kept == 2  # the two valid events
