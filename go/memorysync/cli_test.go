package memorysync

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLICheckpointThenRestorePrintsResumeCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	srcCwd := filepath.Join(t.TempDir(), "o", "h100")
	os.MkdirAll(srcCwd, 0755)
	tgtCwd := filepath.Join(t.TempDir(), "t", "h100")
	os.MkdirAll(tgtCwd, 0755)

	uuid := "11111111-2222-3333-4444-555555555555"
	projDir := ProjectDir(srcCwd)
	os.MkdirAll(projDir, 0755)
	os.WriteFile(filepath.Join(projDir, uuid+".jsonl"),
		[]byte(`{"type":"user","uuid":"u1","parentUuid":null}`+"\n"), 0644)

	repo := filepath.Join(t.TempDir(), "ss.git")
	exec.Command("git", "init", "--bare", repo).Run()
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	os.WriteFile(cfgPath, []byte(`project_id = "test"
[store]
backend = "git"
url = "file://`+repo+`"
`), 0644)

	// checkpoint
	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{"checkpoint", uuid, "--cwd", srcCwd, "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("checkpoint exit %d stderr %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), uuid) {
		t.Errorf("checkpoint stdout: %q want %q", stdout.String(), uuid)
	}

	// restore
	stdout.Reset()
	stderr.Reset()
	code = RunCLI([]string{"restore", uuid, "--cwd", tgtCwd, "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("restore exit %d stderr %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "claude --resume") || !strings.Contains(out, uuid) {
		t.Errorf("restore stdout: %q want 'claude --resume %s'", out, uuid)
	}
}

func TestCLINotConfiguredError(t *testing.T) {
	// verify the sentinel is surfaced via errors.Is
	if !errors.Is(ErrGitNotConfigured, ErrGitNotConfigured) {
		t.Error("errors.Is should match")
	}
}

func TestStep0MissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{"step0"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("stderr should contain usage, got: %s", stderr.String())
	}
}

func TestStep0BogusModeNoMkdir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dstCwd := filepath.Join(t.TempDir(), "tgt")
	os.MkdirAll(dstCwd, 0755)

	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{"step0", "--src", "x.jsonl", "--uuid", "u", "--mode", "bogus", "--dst-cwd", dstCwd}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit: want 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown mode") {
		t.Errorf("stderr: want 'unknown mode', got %q", stderr.String())
	}
	// validate-before-MkdirAll: no project dir should have been created on a bad mode
	if _, err := os.Stat(ProjectDir(dstCwd)); !os.IsNotExist(err) {
		t.Errorf("project dir should NOT exist on bogus mode; stat err=%v", err)
	}
}

func TestCmdInstallWritesToml(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	// local bare repo as the sync-store URL → checkEmpty/checkPrivate probe
	// offline (no network, no github SSH probe). Mirrors TestCLICheckpointThenRestore.
	repo := filepath.Join(t.TempDir(), "ss.git")
	if err := exec.Command("git", "init", "--bare", repo).Run(); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{"install", "--url", "file://" + repo, "--project-id", "p", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr %s", code, stderr.String())
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ProjectID != "p" {
		t.Errorf("ProjectID: want p, got %q", cfg.ProjectID)
	}
	if cfg.Store.URL != "file://"+repo {
		t.Errorf("Store.URL: %q", cfg.Store.URL)
	}
	if cfg.Store.Backend != "git" {
		t.Errorf("Store.Backend: want git, got %q", cfg.Store.Backend)
	}
}

func TestCmdCheckpointReadsEnvSessionID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "env-uuid-123")

	srcCwd := filepath.Join(t.TempDir(), "o")
	os.MkdirAll(srcCwd, 0755)
	projDir := ProjectDir(srcCwd)
	os.MkdirAll(projDir, 0755)
	// a transcript for the env-uuid so checkpoint has something to bundle
	os.WriteFile(filepath.Join(projDir, "env-uuid-123.jsonl"),
		[]byte(`{"type":"user","uuid":"u1","parentUuid":null}`+"\n"), 0644)

	repo := filepath.Join(t.TempDir(), "ss.git")
	exec.Command("git", "init", "--bare", repo).Run()
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	os.WriteFile(cfgPath, []byte(`project_id = "t"
[store]
backend = "git"
url = "file://`+repo+`"
`), 0644)

	var stdout, stderr bytes.Buffer
	// NO session-uuid arg → must read $CLAUDE_CODE_SESSION_ID
	code := RunCLI([]string{"checkpoint", "--cwd", srcCwd, "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "env-uuid-123") {
		t.Errorf("stdout: want to contain env-uuid-123, got %q", stdout.String())
	}
}

func TestConfigNotFoundMentionsInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate: ~/.memory-sync.toml candidate → nonexistent temp
	t.Setenv("MSYNC_CONFIG", "")  // isolate: no leaked MSYNC_CONFIG
	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{"checkpoint", "u", "--config", filepath.Join(t.TempDir(), "nope.toml")}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit: want 1, got %d", code)
	}
	msg := stderr.String()
	if !strings.Contains(msg, "install") {
		t.Errorf("error should mention `install`, got: %s", msg)
	}
	if strings.Contains(msg, "init ") { // "init" as a standalone command word
		t.Errorf("error still says `init`, got: %s", msg)
	}
}

func TestCmdInstallDefaultProjectID(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := filepath.Join(t.TempDir(), "ss.git")
	if err := exec.Command("git", "init", "--bare", repo).Run(); err != nil {
		t.Fatal(err)
	}
	// no --project-id → install defaults to projectIDFromURL(url) = sha256(url)[:12]
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	var stdout, stderr bytes.Buffer
	if code := RunCLI([]string{"install", "--url", "file://" + repo, "--config", cfgPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d stderr %s", code, stderr.String())
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.ProjectID) != 12 {
		t.Errorf("default ProjectID len: want 12, got %d (%q)", len(cfg.ProjectID), cfg.ProjectID)
	}
	for _, c := range cfg.ProjectID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("default ProjectID not hex: %q", cfg.ProjectID)
			break
		}
	}
	// stable: same url → same id
	cfg2Path := filepath.Join(t.TempDir(), ".memory-sync.toml")
	RunCLI([]string{"install", "--url", "file://" + repo, "--config", cfg2Path}, &stdout, &stderr)
	cfg2, _ := LoadConfig(cfg2Path)
	if cfg2.ProjectID != cfg.ProjectID {
		t.Errorf("projectIDFromURL not stable: %q vs %q", cfg.ProjectID, cfg2.ProjectID)
	}
}

func TestNewestSessionUUID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := filepath.Join(t.TempDir(), "proj")
	os.MkdirAll(cwd, 0755)
	projDir := ProjectDir(cwd)
	os.MkdirAll(projDir, 0755)
	// two transcripts; the newer one wins
	os.WriteFile(filepath.Join(projDir, "older-uuid.jsonl"), []byte("{}\n"), 0644)
	newer := filepath.Join(projDir, "newer-uuid.jsonl")
	os.WriteFile(newer, []byte("{}\n"), 0644)
	now := time.Now()
	os.Chtimes(newer, now.Add(2*time.Hour), now.Add(2*time.Hour))

	got, err := newestSessionUUID(cwd)
	if err != nil {
		t.Fatalf("newestSessionUUID: %v", err)
	}
	if got != "newer-uuid" {
		t.Errorf("want newer-uuid, got %q", got)
	}
}
