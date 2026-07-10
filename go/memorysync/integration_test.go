//go:build integration

package memorysync

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestStep0CreateCheckpointRestoreResume is the integration
// regression: it drives the full Step-0 flow end-to-end against the live
// `claude` CLI — create a session carrying fake facts (a token + a path),
// checkpoint it into the git sync-store, restore it into a fresh target cwd
// (origin→target path rewrite), then `claude --resume` and verify the token is
// recalled and the path comes back in TARGET form (origin absent).
//
// Triple-gated: runs only under `go test -tags integration` (build tag) AND
// with MSYNC_INTEGRATION=1 (env) AND with `claude` on PATH. Skips otherwise, so
// the default `go test ./...` never touches the network or the live CLI.
func TestStep0CreateCheckpointRestoreResume(t *testing.T) {
	if os.Getenv("MSYNC_INTEGRATION") == "" {
		t.Skip("set MSYNC_INTEGRATION=1 to run")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not on PATH")
	}

	// Capture the real HOME before monkeypatching. `claude`'s auth/config live
	// under ~/.claude/ + ~/.claude.json; with HOME=temp they are invisible, so
	// we best-effort copy the auth-essential bits into the temp home. (On
	// machines where `claude` authenticates via the OS keychain, the copy is a
	// harmless no-op and keychain auth still works.)
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("determine real HOME: %v", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	copyClaudeConfig(t, realHome, home)

	// The sync-store commits via the USER's git config, which is invisible
	// under a temp HOME (no ~/.gitconfig). GIT_AUTHOR_*/GIT_COMMITTER_* env
	// vars provide identity without a config file. Placeholders only — no PII.
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	origin := filepath.Join(t.TempDir(), "origin", "h100")
	os.MkdirAll(origin, 0755)
	target := filepath.Join(t.TempDir(), "target", "h100")
	os.MkdirAll(target, 0755)
	exec.Command("git", "init", origin).Run()
	exec.Command("git", "init", target).Run()

	repo := filepath.Join(t.TempDir(), "ss.git")
	exec.Command("git", "init", "--bare", repo).Run()
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	os.WriteFile(cfgPath, []byte(`project_id = "test"
[store]
backend = "git"
url = "file://`+repo+`"
`), 0644)

	facts := "Remember three things. (1) the token is TOKEN-FAKE-XYZ-999. (2) the project is at " + origin + ". (3) next is the Go port. Reply OK."
	// create origin session
	cmd := exec.Command("claude", "-p", facts)
	cmd.Dir = origin
	cmd.Stdin = bytes.NewReader(nil)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("claude -p: %v (%s)", err, out)
	}

	// find the session uuid
	projDir := ProjectDir(origin)
	entries, _ := os.ReadDir(projDir)
	var uuid string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			uuid = strings.TrimSuffix(e.Name(), ".jsonl")
		}
	}
	if uuid == "" {
		t.Fatalf("no session found in %s", projDir)
	}

	// checkpoint + restore
	var stdout, stderr bytes.Buffer
	if code := RunCLI([]string{"checkpoint", uuid, "--cwd", origin, "--config", cfgPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("checkpoint failed (exit %d): %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunCLI([]string{"restore", uuid, "--cwd", target, "--config", cfgPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("restore failed (exit %d): %s", code, stderr.String())
	}

	// resume + verify
	cmd = exec.Command("claude", "--resume", uuid, "-p", "What three things did I tell you? Quote them.")
	cmd.Dir = target
	cmd.Stdin = bytes.NewReader(nil)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("claude --resume: %v (%s)", err, out)
	}
	resp := string(out)
	if !strings.Contains(resp, "TOKEN-FAKE-XYZ-999") {
		t.Errorf("token not recalled: %s", resp)
	}
	if !strings.Contains(resp, target) {
		t.Errorf("target path not in response (rewrite not loaded): %s", resp)
	}
	if strings.Contains(resp, origin) {
		t.Errorf("origin path leaked: %s", resp)
	}
}

// copyClaudeConfig best-effort copies the auth-essential files/dirs from the
// real ~/.claude/ + ~/.claude.json into dstHome so a fresh-HOME `claude -p` can
// still authenticate. Errors are logged but non-fatal (a keychain-authed
// `claude` doesn't need these). The large ~/.claude/projects/ tree is skipped
// (the test creates its own project dir under the temp home).
func copyClaudeConfig(t *testing.T, realHome, dstHome string) {
	t.Helper()
	srcClaude := filepath.Join(realHome, ".claude")
	dstClaude := filepath.Join(dstHome, ".claude")
	os.MkdirAll(dstClaude, 0755)
	for _, name := range []string{"settings.json", "statsig", "backups", "plugins", "cache"} {
		src := filepath.Join(srcClaude, name)
		if _, err := os.Stat(src); err != nil {
			continue // absent on this machine — fine
		}
		dst := filepath.Join(dstClaude, name)
		if out, err := exec.Command("cp", "-R", src, dst).CombinedOutput(); err != nil {
			t.Logf("cp %s: %v (%s)", name, err, out) // non-fatal
		}
	}
	// ~/.claude.json lives in HOME itself (not inside ~/.claude/).
	srcJSON := filepath.Join(realHome, ".claude.json")
	dstJSON := filepath.Join(dstHome, ".claude.json")
	if _, err := os.Stat(srcJSON); err == nil {
		if out, err := exec.Command("cp", srcJSON, dstJSON).CombinedOutput(); err != nil {
			t.Logf("cp .claude.json: %v (%s)", err, out)
		}
	}
}
