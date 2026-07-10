package memorysync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func bareRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "sync-store.git")
	if err := exec.Command("git", "init", "--bare", repo).Run(); err != nil {
		t.Fatal(err)
	}
	return repo
}

func makeBundle(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bundle")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "transcript.jsonl"), []byte("{\"type\":\"user\",\"text\":\"hi\"}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{\"project_id\":\"p\"}\n"), 0644)
	return dir
}

func TestGitStorePutGetRoundtrip(t *testing.T) {
	repo := bareRepo(t)
	work1 := filepath.Join(t.TempDir(), "work1")
	s1, err := NewGitStore("file://"+repo, work1)
	if err != nil {
		t.Fatal(err)
	}
	bundle := makeBundle(t)
	if err := s1.Put("ckpt-1", bundle); err != nil {
		t.Fatal(err)
	}
	// fresh work dir — must re-clone from the bare repo
	work2 := filepath.Join(t.TempDir(), "work2")
	s2, err := NewGitStore("file://"+repo, work2)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Get("ckpt-1")
	if err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(got, "transcript.jsonl")) {
		t.Error("transcript.jsonl not found in restored bundle")
	}
	data, _ := os.ReadFile(filepath.Join(got, "transcript.jsonl"))
	if string(data) != "{\"type\":\"user\",\"text\":\"hi\"}\n" {
		t.Errorf("content mismatch: %q", data)
	}
}

func TestGitStorePutIdempotentOverwrite(t *testing.T) {
	repo := bareRepo(t)
	work := filepath.Join(t.TempDir(), "work")
	s, _ := NewGitStore("file://"+repo, work)
	b1 := makeBundle(t)
	os.WriteFile(filepath.Join(b1, "v1"), []byte("one"), 0644)
	s.Put("ckpt", b1)
	// second put with different content — overwrite
	b2 := makeBundle(t)
	os.WriteFile(filepath.Join(b2, "v2"), []byte("two"), 0644)
	if err := s.Put("ckpt", b2); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get("ckpt")
	if fileExists(filepath.Join(got, "v1")) {
		t.Error("v1 should be gone (overwrite)")
	}
	if !fileExists(filepath.Join(got, "v2")) {
		t.Error("v2 should be present (overwrite)")
	}
}

func TestGitStoreNotConfiguredDetection(t *testing.T) {
	// This test verifies the sentinel error is defined + importable.
	// A full end-to-end "no git config" test would need to isolate HOME
	// + remove git config — too fragile for unit tests. The detection
	// logic (checking stderr for "who you are") is verified in the
	// implementation; this test just ensures the sentinel exists.
	if ErrGitNotConfigured == nil {
		t.Error("ErrGitNotConfigured should be non-nil")
	}
	if ErrGitPushFailed == nil {
		t.Error("ErrGitPushFailed should be non-nil")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestCheckBundleSizeWarnUnder100MB(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "big.jsonl"))
	f.Close()
	if err := os.Truncate(f.Name(), 60*1024*1024); err != nil {
		t.Fatal(err)
	}
	s := &GitStore{workDir: t.TempDir()}
	err := s.checkBundleSize(dir)
	if err != nil {
		t.Errorf("60MB should warn, not abort: %v", err)
	}
}

func TestCheckBundleSizeAbortOver100MB(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "huge.jsonl"))
	f.Close()
	if err := os.Truncate(f.Name(), 110*1024*1024); err != nil {
		t.Fatal(err)
	}
	s := &GitStore{workDir: t.TempDir()}
	err := s.checkBundleSize(dir)
	if err == nil {
		t.Error("110MB should abort")
	}
}

func TestCheckBundleSizeSmallOK(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "small.jsonl"), []byte("{}"), 0644)
	s := &GitStore{workDir: t.TempDir()}
	err := s.checkBundleSize(dir)
	if err != nil {
		t.Errorf("small file should pass: %v", err)
	}
}
