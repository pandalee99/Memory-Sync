package memorysync

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Restore pulls the bundle, rewrites origin→target via RewriteSession,
// writes to the target project dir, returns the session UUID.
func Restore(checkpointID, targetCwd, cfgPath string) (string, error) {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	workDir := filepath.Join(home, ".memory-sync", "work", cfg.ProjectID)
	store, err := NewGitStore(cfg.Store.URL, workDir)
	if err != nil {
		return "", fmt.Errorf("new git store: %w", err)
	}
	bundle, err := store.Get(checkpointID)
	if err != nil {
		return "", fmt.Errorf("store get: %w", err)
	}

	// read manifest
	mData, err := os.ReadFile(filepath.Join(bundle, "manifest.json"))
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	m, err := Load(string(mData))
	if err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}
	uuid := m.SessionUUID

	targetProject := ProjectDir(targetCwd)
	if err := os.MkdirAll(targetProject, 0755); err != nil {
		return "", fmt.Errorf("mkdir target project: %w", err)
	}

	// transcript: gunzip if compressed, then RewriteSession (atomic)
	dstJSONL := filepath.Join(targetProject, uuid+".jsonl")
	tmpJSONL := dstJSONL + ".tmp"
	var srcTranscript string
	if m.TranscriptCompressed {
		srcGz := filepath.Join(bundle, uuid+".jsonl.gz")
		srcTranscript = dstJSONL + ".decompressed"
		if err := gunzipFile(srcGz, srcTranscript); err != nil {
			return "", fmt.Errorf("gunzip transcript: %w", err)
		}
		defer os.RemoveAll(srcTranscript)
	} else {
		srcTranscript = filepath.Join(bundle, uuid+".jsonl")
	}
	if _, err := os.Stat(srcTranscript); err == nil {
		if _, err := RewriteSession(srcTranscript, tmpJSONL, m.OriginRoot, targetCwd); err != nil {
			return "", fmt.Errorf("rewrite session: %w", err)
		}
		if err := os.Rename(tmpJSONL, dstJSONL); err != nil {
			return "", fmt.Errorf("rename transcript: %w", err)
		}
	}
	// skipped files warning
	if len(m.SkippedFiles) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d tool-results file(s) were skipped during checkpoint (too large): %v\n", len(m.SkippedFiles), m.SkippedFiles)
	}

	// memory: copy as-is (NOT path-rewritten — spec §13)
	srcMem := filepath.Join(bundle, "memory")
	if _, err := os.Stat(srcMem); err == nil {
		dstMem := filepath.Join(targetProject, "memory")
		if err := os.RemoveAll(dstMem); err != nil {
			return "", fmt.Errorf("remove old memory: %w", err)
		}
		if err := copyDir(srcMem, dstMem); err != nil {
			return "", fmt.Errorf("copy memory: %w", err)
		}
	}

	return uuid, nil
}

func gunzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, gz)
	return err
}
