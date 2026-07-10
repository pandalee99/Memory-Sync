package memorysync

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckpointStoresTranscriptMemoryManifest(t *testing.T) {
	// isolate HOME so we control ~/.claude/projects
	home := t.TempDir()
	t.Setenv("HOME", home)

	srcCwd := filepath.Join(t.TempDir(), "origin", "h100")
	os.MkdirAll(srcCwd, 0755)

	uuid := "11111111-2222-3333-4444-555555555555"
	// create a fake session in the project dir
	projDir := ProjectDir(srcCwd)
	os.MkdirAll(filepath.Join(projDir, "memory"), 0755)
	os.WriteFile(filepath.Join(projDir, uuid+".jsonl"),
		[]byte(`{"type":"user","uuid":"u1","parentUuid":null,"cwd":"`+srcCwd+`","message":{"content":[{"type":"text","text":"working in `+srcCwd+`"}]}}`+"\n"), 0644)
	os.WriteFile(filepath.Join(projDir, "memory", "fact.md"), []byte("# a fact\n"), 0644)

	// bare repo as sync-store
	repo := filepath.Join(t.TempDir(), "ss.git")
	exec.Command("git", "init", "--bare", repo).Run()
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	os.WriteFile(cfgPath, []byte(`project_id = "test"
[store]
backend = "git"
url = "file://`+repo+`"
`), 0644)

	cid, err := Checkpoint(uuid, srcCwd, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cid != uuid {
		t.Errorf("checkpoint id: %q want %q", cid, uuid)
	}

	// verify the store has the bundle
	work := filepath.Join(t.TempDir(), "work")
	s, _ := NewGitStore("file://"+repo, work)
	got, _ := s.Get(uuid)
	if !fileExists(filepath.Join(got, uuid+".jsonl")) {
		t.Error("transcript not in store")
	}
	if !fileExists(filepath.Join(got, "memory", "fact.md")) {
		t.Error("memory not in store")
	}
	mData, _ := os.ReadFile(filepath.Join(got, "manifest.json"))
	var m map[string]any
	json.Unmarshal(mData, &m)
	if m["session_uuid"] != uuid || m["origin_root"] != srcCwd {
		t.Errorf("manifest: %+v", m)
	}
}

func TestCheckpointCompressesLargeTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srcCwd := filepath.Join(t.TempDir(), "origin", "h100")
	os.MkdirAll(srcCwd, 0755)
	uuid := "11111111-2222-3333-4444-555555555555"
	projDir := ProjectDir(srcCwd)
	os.MkdirAll(projDir, 0755)
	// create a > 1MB transcript
	largeContent := strings.Repeat(`{"type":"user","uuid":"u1","parentUuid":null,"message":{"content":[{"type":"text","text":"x"}]}}`+"\n", 20000)
	os.WriteFile(filepath.Join(projDir, uuid+".jsonl"), []byte(largeContent), 0644)
	repo := filepath.Join(t.TempDir(), "ss.git")
	exec.Command("git", "init", "--bare", repo).Run()
	cfgPath := filepath.Join(t.TempDir(), ".memory-sync.toml")
	os.WriteFile(cfgPath, []byte(`project_id = "test"`+"\n[store]\nbackend = \"git\"\nurl = \"file://"+repo+`"`+"\n"), 0644)
	cid, err := Checkpoint(uuid, srcCwd, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cid != uuid {
		t.Errorf("cid: %q want %q", cid, uuid)
	}
	// verify the store has the gzipped transcript + manifest says compressed
	work := filepath.Join(t.TempDir(), "work")
	s, _ := NewGitStore("file://"+repo, work)
	got, _ := s.Get(uuid)
	if !fileExists(filepath.Join(got, uuid+".jsonl.gz")) {
		t.Error("gzipped transcript not in store")
	}
	if fileExists(filepath.Join(got, uuid+".jsonl")) {
		t.Error("uncompressed transcript should not be in store")
	}
	mData, _ := os.ReadFile(filepath.Join(got, "manifest.json"))
	var m map[string]any
	json.Unmarshal(mData, &m)
	if m["transcript_compressed"] != true {
		t.Error("manifest should have transcript_compressed = true")
	}
	// content integrity: gunzip the stored .gz + verify content matches
	gzPath := filepath.Join(got, uuid+".jsonl.gz")
	gzFile, _ := os.Open(gzPath)
	defer gzFile.Close()
	gzReader, _ := gzip.NewReader(gzFile)
	defer gzReader.Close()
	decompressed, _ := io.ReadAll(gzReader)
	if !strings.Contains(string(decompressed), `"type":"user"`) {
		t.Error("gunzipped transcript content doesn't match original")
	}
}
