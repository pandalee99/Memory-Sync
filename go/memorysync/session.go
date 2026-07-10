package memorysync

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// newestSessionUUID returns the uuid (filename stem) of the most-recently-
// modified <uuid>.jsonl in ProjectDir(cwd) — the current session's actively-
// written transcript. Used by cmdCheckpoint's auto-detect fallback (no uuid arg
// + no $CLAUDE_CODE_SESSION_ID env). Returns an error if no .jsonl found.
func newestSessionUUID(cwd string) (string, error) {
	projDir := ProjectDir(cwd)
	entries, err := os.ReadDir(projDir)
	if err != nil {
		return "", fmt.Errorf("read project dir %s: %w", projDir, err)
	}
	type f struct {
		name  string
		mtime int64
	}
	var files []f
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, f{e.Name(), fi.ModTime().UnixNano()})
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no session transcript found in %s", projDir)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mtime > files[j].mtime })
	return strings.TrimSuffix(files[0].name, ".jsonl"), nil
}
