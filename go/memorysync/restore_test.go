package memorysync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreRewritesPathsToTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	srcCwd := filepath.Join(t.TempDir(), "origin", "h100")
	os.MkdirAll(srcCwd, 0755)
	targetCwd := filepath.Join(t.TempDir(), "target", "h100")
	os.MkdirAll(targetCwd, 0755)

	uuid := "11111111-2222-3333-4444-555555555555"
	// checkpoint a session first (reuse Checkpoint)
	projDir := ProjectDir(srcCwd)
	os.MkdirAll(projDir, 0755)
	os.WriteFile(filepath.Join(projDir, uuid+".jsonl"),
		[]byte(`{"type":"user","uuid":"u1","parentUuid":null,"cwd":"`+srcCwd+`","message":{"content":[{"type":"text","text":"see `+srcCwd+`/x"}]}}`+"\n"), 0644)

	repo := filepath.Join(t.TempDir(), "ss.git")
	exec.Command("git", "init", "--bare", repo).Run()
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	os.WriteFile(cfgPath, []byte(`project_id = "test"
[store]
backend = "git"
url = "file://`+repo+`"
`), 0644)

	Checkpoint(uuid, srcCwd, cfgPath)

	// fresh work dir for restore
	t.Setenv("HOME", home) // keep same HOME
	uuid2, err := Restore(uuid, targetCwd, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if uuid2 != uuid {
		t.Errorf("uuid: %q want %q", uuid2, uuid)
	}
	// verify the target project dir has the rewritten transcript
	tProj := ProjectDir(targetCwd)
	out, _ := os.ReadFile(filepath.Join(tProj, uuid+".jsonl"))
	if strings.Contains(string(out), srcCwd) {
		t.Error("origin path still in output")
	}
	if !strings.Contains(string(out), targetCwd) {
		t.Error("target path not in output")
	}
}

func TestRestoreGunzipsCompressedTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srcCwd := filepath.Join(t.TempDir(), "origin", "h100")
	os.MkdirAll(srcCwd, 0755)
	tgtCwd := filepath.Join(t.TempDir(), "target", "h100")
	os.MkdirAll(tgtCwd, 0755)
	uuid := "11111111-2222-3333-4444-555555555555"
	projDir := ProjectDir(srcCwd)
	os.MkdirAll(projDir, 0755)
	largeContent := strings.Repeat(`{"type":"user","uuid":"u1","parentUuid":null,"cwd":"`+srcCwd+`","message":{"content":[{"type":"text","text":"see `+srcCwd+`/x"}]}}`+"\n", 20000)
	os.WriteFile(filepath.Join(projDir, uuid+".jsonl"), []byte(largeContent), 0644)
	repo := filepath.Join(t.TempDir(), "ss.git")
	exec.Command("git", "init", "--bare", repo).Run()
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	os.WriteFile(cfgPath, []byte(`project_id = "test"`+"\n[store]\nbackend = \"git\"\nurl = \"file://"+repo+`"`+"\n"), 0644)
	Checkpoint(uuid, srcCwd, cfgPath) // checkpoint with gzip
	t.Setenv("HOME", home)
	uuid2, err := Restore(uuid, tgtCwd, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if uuid2 != uuid {
		t.Errorf("uuid: %q want %q", uuid2, uuid)
	}
	tProj := ProjectDir(tgtCwd)
	out, _ := os.ReadFile(filepath.Join(tProj, uuid+".jsonl"))
	if strings.Contains(string(out), srcCwd) {
		t.Error("origin path in restored output")
	}
	if !strings.Contains(string(out), tgtCwd) {
		t.Error("target path not in restored output")
	}
	if !strings.Contains(string(out), `"type":"user"`) {
		t.Error("transcript content missing — gunzip failed?")
	}
}
