"""Regex secret scanner + redaction (Shannon-entropy deferred to M3)."""
import re

# (kind, compiled pattern). Order only matters for overlap resolution in redact.
PATTERNS = [
    ("anthropic", re.compile(r"(?:sk-ant-|bsk-)[A-Za-z0-9_-]{20,}")),
    ("openai", re.compile(r"sk-[A-Za-z0-9]{20,}")),
    ("aws", re.compile(r"AKIA[0-9A-Z]{16}")),
    ("google", re.compile(r"AIza[0-9A-Za-z_\-]{35}")),
    ("github_pat", re.compile(r"gh[pousr]_[A-Za-z0-9]{36,}")),
    ("private_key", re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH |DSA |)PRIVATE KEY-----")),
    ("bearer", re.compile(r"Bearer\s+[A-Za-z0-9_\-\.=]{20,}")),
    ("env_assign", re.compile(r"(?:password|passwd|secret|token|api_key|apikey|access_key)\s*[=:]\s*\S+")),
]

# env_assign matches the whole "key=value" assignment; for redaction we redact
# (and store in the map) only the secret VALUE, leaving the "key=" label visible
# so scrubbed text stays readable and placeholder->original round-trips exactly.
# The env_assign pattern always contains a [:=] separator, so this always finds it.
_ENV_VALUE = re.compile(r"[=:]\s*(\S+)")


def scan_text(text: str) -> list[tuple[str, str]]:
    """Return [(kind, matched_string), ...] for all regex hits."""
    out = []
    for kind, pat in PATTERNS:
        for m in pat.finditer(text):
            out.append((kind, m.group(0)))
    return out


def _redact(text: str, counter: list[int]) -> tuple[str, dict]:
    """Redact secrets -> `<redacted:kind:N>` with a shared counter (placeholders
    globally unique across a whole object). counter is [next_index], incremented
    per placeholder. Overlaps resolved first-match-wins (sorted by start). For
    env_assign, only the value is redacted; the "key=" label is preserved.
    """
    matches = []
    for kind, pat in PATTERNS:
        for m in pat.finditer(text):
            if kind == "env_assign":
                vm = _ENV_VALUE.search(m.group(0))
                s = m.start() + vm.start(1)
                e = m.start() + vm.end(1)
                val = vm.group(1)
            else:
                s, e, val = m.start(), m.end(), m.group(0)
            matches.append((s, e, kind, val))
    matches.sort()

    chosen = []
    last_end = -1
    for s, e, kind, val in matches:
        if s < last_end:
            continue  # overlap; skip
        chosen.append((s, e, kind, val))
        last_end = e

    out = []
    mapping = {}
    pos = 0
    for s, e, kind, val in chosen:
        counter[0] += 1
        ph = f"<redacted:{kind}:{counter[0]}>"
        mapping[ph] = val
        out.append(text[pos:s])
        out.append(ph)
        pos = e
    out.append(text[pos:])
    return "".join(out), mapping


def redact_text(text: str) -> tuple[str, dict]:
    """Redact secrets in a single string (counter starts at 1)."""
    return _redact(text, [0])


def redact_obj(o) -> tuple[object, dict]:
    """Recursively redact all strings in o with a SHARED counter so placeholders
    are globally unique (no clobber across sibling strings)."""
    mapping: dict = {}
    counter = [0]

    def walk(x):
        if isinstance(x, str):
            red, m = _redact(x, counter)
            mapping.update(m)
            return red
        if isinstance(x, list):
            return [walk(i) for i in x]
        if isinstance(x, dict):
            return {k: walk(v) for k, v in x.items()}
        return x

    return walk(o), mapping
