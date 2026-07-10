package memorysync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".memory-sync.toml")
	os.WriteFile(path, []byte(`project_id = "my-proj"
[store]
backend = "git"
url = "/tmp/sync-store.git"
`), 0644)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectID != "my-proj" || cfg.Store.URL != "/tmp/sync-store.git" {
		t.Errorf("config: %+v", cfg)
	}
}

func TestFindConfigPriorityFlagWins(t *testing.T) {
	dir := t.TempDir()
	// local ./.memory-sync.toml
	localPath := filepath.Join(dir, ".memory-sync.toml")
	os.WriteFile(localPath, []byte(`project_id = "local"`), 0644)
	// flag path (should win)
	flagDir := t.TempDir()
	flagPath := filepath.Join(flagDir, ".memory-sync.toml")
	os.WriteFile(flagPath, []byte(`project_id = "flag"`), 0644)
	got, err := FindConfig(flagPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := LoadConfig(got)
	if cfg.ProjectID != "flag" {
		t.Errorf("priority: %q want flag", got)
	}
}

func TestFindConfigNotFound(t *testing.T) {
	_, err := FindConfig("/nonexistent/path.toml")
	if err == nil {
		t.Error("should error when no config found")
	}
}
