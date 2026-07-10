import os
import pytest

# Optional real-corpus smoke test: set MSYNC_GOLDEN_CORPUS to a local session
# JSONL to exercise the rewriter on a real session. NEVER committed; the path is
# not hardcoded here — the developer sets the env var locally. Skips if unset.


@pytest.fixture(scope="session")
def golden_corpus():
    path = os.environ.get("MSYNC_GOLDEN_CORPUS")
    if not path or not os.path.exists(path):
        pytest.skip("MSYNC_GOLDEN_CORPUS not set or absent")
    return path
