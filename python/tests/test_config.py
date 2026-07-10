from memory_sync import config


def test_config_load(tmp_path):
    toml = tmp_path / ".memory-sync.toml"
    toml.write_text(
        'project_id = "my-proj"\n'
        '[store]\n'
        'backend = "git"\n'
        'url = "/tmp/sync-store.git"\n'
    )
    cfg = config.load(str(toml))
    assert cfg.project_id == "my-proj"
    assert cfg.store_url == "/tmp/sync-store.git"
