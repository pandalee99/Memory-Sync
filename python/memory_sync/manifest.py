"""Checkpoint manifest schema + (de)serialization."""
import json
from dataclasses import asdict, dataclass


@dataclass
class Manifest:
    project_id: str
    origin_root: str
    origin_dashed: str
    session_uuid: str
    git_head: str
    claude_version: str = "v2.1.202"
    schema_version: int = 1
    hlc_ts: str = ""
    parent_checkpoint: str | None = None
    layer_manifest: dict | None = None
    redaction_ref: str | None = None


def dump(m: Manifest) -> str:
    return json.dumps(asdict(m), indent=2, ensure_ascii=False)


def load(text: str) -> Manifest:
    d = json.loads(text)
    valid = {k: d[k] for k in Manifest.__dataclass_fields__ if k in d}
    return Manifest(**valid)
