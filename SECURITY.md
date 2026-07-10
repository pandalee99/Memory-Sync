# Security

## Transcript storage

Memory-Sync checkpoints store Claude Code session transcripts **in plaintext** in your
private git sync-store. **There is no secret redaction in v0.1.0.** A checkpoint may
contain anything present in the session — including API keys, tokens, or other secrets
typed or rendered during the run.

**Do not checkpoint sessions that contain live secrets to a public repo.** If a session
may have touched secrets, keep its sync-store private.

## Use a private sync-store

`/memory-sync install` asks for a sync-store git repo URL and **refuses a public
sync-store** — the install checks the repo's visibility and aborts if the store is
public. Use a private repo (e.g. a private GitHub repository).

## Planned hardening

- **age-encryption** of transcripts at rest in the sync-store is a planned hardening
  for a later release. Until then, plaintext + a private repo is the trust boundary.

## Vulnerability reporting

Found a security issue? Please **open a GitHub issue** at
<https://github.com/pandalee99/Memory-Sync/issues> or email **pandali.kk@qq.com**.
