package memorysync

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Checkpoint reads the session transcript + memory from the src project dir,
// stores them (raw) + a manifest in the sync-store, pushes, returns the
// checkpoint ID (= the session UUID).
func Checkpoint(uuid, srcCwd, cfgPath string) (string, error) {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	srcProject := ProjectDir(srcCwd)
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	workDir := filepath.Join(home, ".memory-sync", "work", cfg.ProjectID)
	store, err := NewGitStore(cfg.Store.URL, workDir)
	if err != nil {
		return "", fmt.Errorf("new git store: %w", err)
	}

	// build a temp bundle (atomic)
	bundle, err := os.MkdirTemp("", "msync-bundle-")
	if err != nil {
		return "", fmt.Errorf("mktemp bundle: %w", err)
	}
	defer os.RemoveAll(bundle)

	// manifest
	m := Manifest{
		ProjectID:    cfg.ProjectID,
		OriginRoot:   srcCwd,
		OriginDashed: Dashed(srcCwd),
		SessionUUID:  uuid,
		GitHead:      gitHead(srcCwd),
	}

	// transcript: gzip if > 1MB, else copy. A missing transcript is a hard
	// error — without it the checkpoint would be manifest-only and silently
	// lie about restoring a session.
	srcJSONL := filepath.Join(srcProject, uuid+".jsonl")
	fi, err := os.Stat(srcJSONL)
	if err != nil {
		return "", fmt.Errorf("session transcript not found at %s (wrong --cwd or uuid?)", srcJSONL)
	}
	if fi.Size() > 1024*1024 {
		if err := gzipFile(srcJSONL, filepath.Join(bundle, uuid+".jsonl.gz")); err != nil {
			return "", fmt.Errorf("gzip transcript: %w", err)
		}
		m.TranscriptCompressed = true
	} else {
		if err := copyFile(srcJSONL, filepath.Join(bundle, uuid+".jsonl")); err != nil {
			return "", fmt.Errorf("copy transcript: %w", err)
		}
	}
	// memory
	srcMem := filepath.Join(srcProject, "memory")
	if _, err := os.Stat(srcMem); err == nil {
		if err := copyDir(srcMem, filepath.Join(bundle, "memory")); err != nil {
			return "", fmt.Errorf("copy memory: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), []byte(Dump(m)), 0644); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	if err := store.Put(uuid, bundle); err != nil {
		return "", fmt.Errorf("store put: %w", err)
	}
	return uuid, nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	_, err = io.Copy(gz, in)
	return err
}

func gitHead(cwd string) string {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
