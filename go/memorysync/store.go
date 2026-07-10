package memorysync

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrGitNotConfigured = errors.New("git not configured — please set user.name and user.email")
	ErrGitPushFailed    = errors.New("git push failed")
	ErrPermissionDenied = errors.New("permission denied")
)

const (
	warnThreshold  int64 = 50 * 1024 * 1024  // 50MB — GitHub warns
	abortThreshold int64 = 100 * 1024 * 1024 // 100MB — GitHub rejects
)

// GitStore is a content store backed by a git repo (the sync-store).
type GitStore struct {
	url     string
	workDir string
}

// validateStoreURL enforces an allowlist of sync-store URL schemes to prevent
// git transport RCE (e.g. ext::) and option injection (a leading "-"). Only
// https://, ssh://git@, git@github.com:, and file:// are permitted; everything
// else is rejected. Bare local filesystem paths are NOT allowed — use the
// file:// scheme for local sync-stores.
func validateStoreURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("store URL is empty")
	}
	if url[0] == '-' {
		return fmt.Errorf("store URL must not start with '-' (got %q)", url)
	}
	for _, p := range []string{"https://", "ssh://git@", "git@github.com:", "file://"} {
		if strings.HasPrefix(url, p) {
			return nil
		}
	}
	return fmt.Errorf("store URL %q uses an unsupported scheme; allowed: https://, ssh://git@, git@github.com:, file:// (ext:: and other transports are blocked)", url)
}

// NewGitStore creates a GitStore, cloning or pulling the sync-store.
func NewGitStore(url, workDir string) (*GitStore, error) {
	if err := validateStoreURL(url); err != nil {
		return nil, fmt.Errorf("invalid store URL: %w", err)
	}
	s := &GitStore{url: url, workDir: workDir}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir work dir: %w", err)
	}
	if err := s.checkGitVersion(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if err := s.sync(); err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	return s, nil
}

func (s *GitStore) checkGitVersion() error {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	re := regexp.MustCompile(`git version (\d+)\.(\d+)`)
	m := re.FindStringSubmatch(strings.TrimSpace(string(out)))
	if m == nil {
		return fmt.Errorf("could not parse git version: %s", strings.TrimSpace(string(out)))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 2 || (major == 2 && minor < 20) {
		return fmt.Errorf("git version %s.%s is old (< 2.20); some features may not work", m[1], m[2])
	}
	return nil
}

func (s *GitStore) run(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (s *GitStore) sync() error {
	if _, err := os.Stat(filepath.Join(s.workDir, ".git")); err == nil {
		_, err := s.run("pull", "--ff-only")
		return err
	}
	// clone the sync-store into the work dir. "--" separates git options from
	// the user-influenced URL/path so a URL cannot masquerade as a flag. On
	// clone failure (auth, network, wrong URL) surface the real error rather
	// than silently falling back to an empty init (which would produce a
	// manifest-only/unsynced store).
	out, err := exec.Command("git", "clone", "--", s.url, s.workDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone %s: %w (%s)", s.url, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Put stores a bundle directory under <key>, atomically. Uses the USER's
// own git config for the commit (NOT hardcoded). Pull --rebase before push
// to reduce conflicts. Idempotent overwrite.
func (s *GitStore) Put(key, bundleDir string) error {
	dest := filepath.Join(s.workDir, key)
	if _, err := os.Stat(dest); err == nil {
		os.RemoveAll(dest) // idempotent overwrite
	}
	// atomic: copy to temp, then rename
	tmpDest, err := os.MkdirTemp(s.workDir, ".tmp-put-")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	if err := copyDir(bundleDir, tmpDest); err != nil {
		os.RemoveAll(tmpDest)
		return fmt.Errorf("copy bundle: %w", err)
	}
	if err := os.Rename(tmpDest, dest); err != nil {
		os.RemoveAll(tmpDest)
		return fmt.Errorf("rename: %w", err)
	}
	if err := s.checkBundleSize(dest); err != nil {
		return fmt.Errorf("bundle size check: %w", err)
	}
	if _, err := s.run("add", key); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	// commit — uses the USER's git config (no -c override)
	if _, err := s.run("commit", "-m", "checkpoint "+key); err != nil {
		if strings.Contains(err.Error(), "who you are") || strings.Contains(err.Error(), "user.name") {
			return ErrGitNotConfigured
		}
		// "nothing to commit" is OK (idempotent re-put of identical content)
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %w", err)
	}
	// pull --rebase before push (reduce non-fast-forward failures)
	s.run("pull", "--rebase", "origin", "HEAD") // best-effort; ignore error on first push
	if _, err := s.run("push", "origin", "HEAD"); err != nil {
		return fmt.Errorf("%w: %v", ErrGitPushFailed, err)
	}
	return nil
}

// checkBundleSize walks bundleDir and aborts if any file exceeds GitHub's
// 100MB hard limit, warning on files over 50MB.
func (s *GitStore) checkBundleSize(bundleDir string) error {
	return filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		size := info.Size()
		rel, _ := filepath.Rel(bundleDir, path)
		if size > abortThreshold {
			return fmt.Errorf("file %s is %d MB, exceeds GitHub's 100MB limit", rel, size/(1024*1024))
		}
		if size > warnThreshold {
			fmt.Fprintf(os.Stderr, "warning: %s is %d MB (GitHub warns at 50MB)\n", rel, size/(1024*1024))
		}
		return nil
	})
}

// Get pulls + returns the path to the bundle for <key>.
func (s *GitStore) Get(key string) (string, error) {
	s.run("pull", "--ff-only") // best-effort
	path := filepath.Join(s.workDir, key)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("key %q not found: %w", key, err)
	}
	return path, nil
}

// copyDir recursively copies src to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
