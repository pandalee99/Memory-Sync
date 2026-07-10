from memory_sync import scanner


def test_scan_finds_anthropic_key():
    text = "my key is bsk-ant-FAKEFAKEFAKEFAKEFAKE2024 plus sk-ant-FAKEFAKEFAKEFAKEFAKE2024"
    kinds = {k for k, _ in scanner.scan_text(text)}
    assert "anthropic" in kinds


def test_scan_finds_aws_and_github():
    text = "aws AKIAIOSFODNN7EXAMPLE and github ghp_FAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKE"
    kinds = {k for k, _ in scanner.scan_text(text)}
    assert "aws" in kinds
    assert "github_pat" in kinds


def test_scan_finds_private_key_block():
    text = "-----BEGIN RSA PRIVATE KEY-----\nMIIBfake...\n-----END RSA PRIVATE KEY-----"
    kinds = {k for k, _ in scanner.scan_text(text)}
    assert "private_key" in kinds


def test_redact_replaces_and_keeps_map():
    text = "token=FAKESECRET12345 password=FAKEPASS2abc"
    red, mapping = scanner.redact_text(text)
    assert "FAKESECRET12345" not in red
    assert "FAKEPASS2abc" not in red
    assert "<redacted:" in red
    originals = set(mapping.values())
    assert "FAKESECRET12345" in originals
    assert "FAKEPASS2abc" in originals


def test_redact_no_false_positive_on_plain_text():
    red, mapping = scanner.redact_text("just a normal sentence with no secrets")
    assert red == "just a normal sentence with no secrets"
    assert mapping == {}


def test_redact_obj_recurses():
    o = {"cwd": "/a", "msg": {"text": ["token=FAKESECRET12345", "plain"]}, "n": 3}
    red, mapping = scanner.redact_obj(o)
    assert "FAKESECRET12345" not in str(red)
    assert red["n"] == 3
    assert any("FAKESECRET12345" in v for v in mapping.values())


def test_redact_obj_no_collision_across_strings():
    # two same-kind secrets in sibling strings -> distinct placeholders, both originals kept
    o = {"a": "token=FAKESECRET1", "b": "token=FAKESECRET2"}
    red, mapping = scanner.redact_obj(o)
    assert red["a"] != red["b"]
    assert len(mapping) == 2
    assert set(mapping.values()) == {"FAKESECRET1", "FAKESECRET2"}
    assert "FAKESECRET1" not in str(red)
    assert "FAKESECRET2" not in str(red)
