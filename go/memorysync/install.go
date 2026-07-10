package memorysync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// projectIDFromURL returns a stable id derived from the sync-store URL
// (hash of the url; matches .memory-sync.toml.example's "hash of the git remote").
func projectIDFromURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:6]) // 12 hex chars
}

// checkEmpty reports whether the git url has no refs (empty repo). A non-nil
// error means ls-remote failed (auth failure / no repo).
func checkEmpty(url string) (bool, error) {
	out, err := exec.Command("git", "ls-remote", "--", url).Output()
	if err != nil {
		return false, fmt.Errorf("git ls-remote: %w", err)
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

// githubOwnerRepo extracts "owner/repo" from a github git URL, else ok=false.
func githubOwnerRepo(url string) (string, bool) {
	for _, p := range []string{"git@github.com:", "https://github.com/", "ssh://git@github.com/"} {
		if strings.HasPrefix(url, p) {
			tail := strings.TrimSuffix(strings.TrimPrefix(url, p), ".git")
			tail = strings.TrimSuffix(tail, "/")
			return tail, tail != ""
		}
	}
	return "", false
}

// checkPrivate returns the repo visibility ("public"/"private") if checkable.
// A non-nil error means not checkable (not github, or gh not on PATH) — the
// caller treats this as a graceful skip (warn), not a failure.
func checkPrivate(url string) (string, error) {
	ownerRepo, ok := githubOwnerRepo(url)
	if !ok {
		return "", fmt.Errorf("not a github url")
	}
	gh, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh not on PATH")
	}
	out, err := exec.Command(gh, "repo", "view", ownerRepo, "--json", "visibility").Output()
	if err != nil {
		return "", fmt.Errorf("gh repo view: %w", err)
	}
	var v struct {
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", err
	}
	return v.Visibility, nil
}
