package memorysync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashedBasic(t *testing.T) {
	if got := Dashed("/home/alice/code/h100"); !strings.HasSuffix(got, "-home-alice-code-h100") {
		t.Errorf("Dashed: %q want suffix -home-alice-code-h100", got)
	}
}

func TestDashedUnderscoreBecomesHyphen(t *testing.T) {
	if got := Dashed("/home/alice/msync_origin/h100"); !strings.HasSuffix(got, "-home-alice-msync-origin-h100") {
		t.Errorf("Dashed underscore: %q want suffix -home-alice-msync-origin-h100", got)
	}
}

func TestDashedDotsAndSpaces(t *testing.T) {
	if got := Dashed("/home/alice/my proj/v1.2"); !strings.HasSuffix(got, "-home-alice-my-proj-v1-2") {
		t.Errorf("Dashed dots/spaces: %q want suffix -home-alice-my-proj-v1-2", got)
	}
}

func TestDashedTmpUsesRealpath(t *testing.T) {
	// realpath-agnostic: compute expected via the same EvalSymlinks-+-fallback logic
	real, err := filepath.EvalSymlinks("/tmp/fakehome/code/h100")
	if err != nil {
		real = "/tmp/fakehome/code/h100"
	}
	want := strings.ReplaceAll(real, "/", "-")
	if got := Dashed("/tmp/fakehome/code/h100"); got != want {
		t.Errorf("Dashed /tmp: got %q want %q", got, want)
	}
}

func TestProjectDir(t *testing.T) {
	d := ProjectDir("/home/alice/code/h100")
	home, _ := os.UserHomeDir()
	if !strings.HasPrefix(d, filepath.Join(home, ".claude", "projects")) {
		t.Errorf("ProjectDir: %q want prefix %q", d, filepath.Join(home, ".claude", "projects"))
	}
	if !strings.HasSuffix(d, "-home-alice-code-h100") {
		t.Errorf("ProjectDir: %q want suffix -home-alice-code-h100", d)
	}
}

func TestDashedWindowsPath(t *testing.T) {
	// synthetic Windows path: drive letter + colon + backslashes → all non-alnum → -
	// per-char rule (match Python): `:` + `\` are consecutive non-alnum → `C--Users-bob-proj` (double dash).
	// Real Windows encoding (collapse vs per-char) is UNVERIFIED — deferred to a Windows machine/CI.
	if got := Dashed(`C:\Users\bob\proj`); !strings.HasSuffix(got, "C--Users-bob-proj") {
		t.Errorf("Dashed Windows: %q want suffix C--Users-bob-proj (per-char)", got)
	}
}

func TestDashedGoldenCorpusSmoke(t *testing.T) {
	path := os.Getenv("MSYNC_GOLDEN_CORPUS")
	if path == "" {
		t.Skip("MSYNC_GOLDEN_CORPUS not set")
	}
	// the corpus exists + Dashed of its project dir is non-empty
	projDir := filepath.Dir(path)
	if Dashed(projDir) == "" {
		t.Errorf("Dashed of corpus project dir is empty")
	}
}
