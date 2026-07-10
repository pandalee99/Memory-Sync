package memorysync

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain makes the test suite self-contained w.r.t. git so it runs green in
// a clean CI runner (no global git identity, master-default git):
//
//   - GIT_AUTHOR_*/GIT_COMMITTER_* env vars provide a commit identity that
//     overrides any (absent) user.name/user.email config — otherwise `git commit`
//     in the temp bare repos fails "git not configured".
//   - A temp global git config pins init.defaultBranch=main — otherwise
//     `git init --bare` defaults to `master` and mismatches the branch the
//     restore/GitStore tests expect to pull.
//   - GIT_CONFIG_NOSYSTEM=1 ignores /etc/gitconfig (keeps the suite hermetic).
//
// This is test-process-scoped (os.Setenv, not t.Setenv): the vars live only for
// the `go test` process and don't leak to the caller's shell.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "msync-test-gitconfig-")
	if err != nil {
		panic(err)
	}
	globalCfg := filepath.Join(dir, ".gitconfig")
	if err := os.WriteFile(globalCfg, []byte("[init]\n\tdefaultBranch = main\n[core]\n\tfsmonitor = false\n"), 0644); err != nil {
		panic(err)
	}

	os.Setenv("GIT_AUTHOR_NAME", "memory-sync-test")
	os.Setenv("GIT_AUTHOR_EMAIL", "test@memory-sync.local")
	os.Setenv("GIT_COMMITTER_NAME", "memory-sync-test")
	os.Setenv("GIT_COMMITTER_EMAIL", "test@memory-sync.local")
	os.Setenv("GIT_CONFIG_GLOBAL", globalCfg)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
