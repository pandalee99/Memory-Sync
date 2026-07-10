package memorysync

import (
	"os"
	"path/filepath"
	"regexp"
)

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// Dashed encodes a cwd to Claude Code's project-dir name.
//
// Rule (Step 0 verified): resolve realpath (filepath.EvalSymlinks), then replace
// every non-alphanumeric char (including '/' AND '_') with '-'. Case preserved.
// If the path doesn't exist (e.g. synthetic test paths), EvalSymlinks errors and
// we fall back to the path as-is — tests are self-consistent (use Dashed() to
// compute expected, not hardcoded strings).
func Dashed(cwd string) string {
	real, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		real = cwd
	}
	return nonAlnum.ReplaceAllString(real, "-")
}

// ProjectDir returns the on-disk project directory path for a cwd.
func ProjectDir(cwd string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects", Dashed(cwd))
}
