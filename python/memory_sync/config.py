"""Parse .memory-sync.toml (tomllib, stdlib)."""
import tomllib
from dataclasses import dataclass


@dataclass
class Config:
    project_id: str
    store_url: str


def load(path: str) -> Config:
    with open(path, "rb") as f:
        d = tomllib.load(f)
    return Config(project_id=d["project_id"], store_url=d["store"]["url"])
