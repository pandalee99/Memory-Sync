package memorysync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGithubOwnerRepo(t *testing.T) {
	cases := []struct{ in, want string }{
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"git@gitlab.com:owner/repo.git", ""}, // not github
	}
	for _, c := range cases {
		got, ok := githubOwnerRepo(c.in)
		if c.want == "" && ok {
			t.Errorf("%q: want not-ok, got %q", c.in, got)
		}
		if c.want != "" && got != c.want {
			t.Errorf("%q: want %q, got %q", c.in, c.want, got)
		}
	}
}

func TestCheckEmpty(t *testing.T) {
	// only run if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	empty := filepath.Join(t.TempDir(), "empty.git")
	if err := exec.Command("git", "init", "--bare", empty).Run(); err != nil {
		t.Fatal(err)
	}
	got, err := checkEmpty(empty)
	if err != nil {
		t.Fatalf("checkEmpty: %v", err)
	}
	if !got {
		t.Errorf("empty bare repo: want empty=true, got false")
	}
}

func TestExampleTomlIsPlaintext(t *testing.T) {
	b, err := os.ReadFile("../../.memory-sync.toml.example")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{"age_pubkey", "[[members]]", "age-keygen", "age-encrypted"} {
		if strings.Contains(s, bad) {
			t.Errorf("example still contains %q (decision ②A: plaintext, no age)", bad)
		}
	}
	// must parse + have the plaintext fields
	cfg, err := LoadConfig("../../.memory-sync.toml.example")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Store.Backend == "" || cfg.Store.URL == "" {
		t.Errorf("example missing [store] backend/url: %+v", cfg)
	}
}
