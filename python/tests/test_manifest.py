from memory_sync import manifest


def test_manifest_roundtrip_minimal():
    m = manifest.Manifest(
        project_id="p",
        origin_root="/home/alice/code/h100",
        origin_dashed="-home-alice-code-h100",
        session_uuid="11111111-2222-3333-4444-555555555555",
        git_head="abc123",
    )
    text = manifest.dump(m)
    m2 = manifest.load(text)
    assert m2 == m
    assert m2.schema_version == 1
    assert m2.claude_version == "v2.1.202"


def test_manifest_roundtrip_with_optionals():
    m = manifest.Manifest(
        project_id="p", origin_root="/a", origin_dashed="-a",
        session_uuid="11111111-2222-3333-4444-555555555555", git_head="h",
        parent_checkpoint="prev-sha",
        layer_manifest={"L1": 928, "L2": 8},
        redaction_ref="redact-blob-sha",
    )
    m2 = manifest.load(manifest.dump(m))
    assert m2 == m
    assert m2.layer_manifest == {"L1": 928, "L2": 8}


def test_manifest_load_ignores_unknown_fields():
    m = manifest.Manifest(
        project_id="p", origin_root="/a", origin_dashed="-a",
        session_uuid="11111111-2222-3333-4444-555555555555", git_head="h",
    )
    text = manifest.dump(m)
    import json
    d = json.loads(text)
    d["unknown_future_field"] = "should be ignored"
    d["another"] = 99
    m2 = manifest.load(json.dumps(d))
    assert m2 == m  # loads fine, ignores extras
