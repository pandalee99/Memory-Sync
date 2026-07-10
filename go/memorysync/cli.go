package memorysync

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// flagsFirst reorders args so flags (and their values) precede positional
// arguments. The Go flag package stops parsing at the first non-flag argument,
// so `<positional> --flag value` would leave --flag unparsed; this lets the
// CLI accept the interspersed ordering that the Python original's argparse
// handled natively. The CLI's flags (--cwd, --config) are all value-taking,
// so a flag token without an inline `=` consumes the next non-flag token as
// its value.
func flagsFirst(args []string) []string {
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			if strings.Contains(a, "=") {
				continue // --flag=value: value is inline
			}
			if i+1 < len(args) && !(len(args[i+1]) > 1 && args[i+1][0] == '-') {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, a)
	}
	return append(flags, positionals...)
}

// RunCLI runs the CLI with the given args + writers. Returns the exit code.
// stdout: results (checkpoint id, "claude --resume <uuid>", status).
// stderr: progress, warnings, errors.
func RunCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: memory-sync <checkpoint|restore|status> [args]")
		return 1
	}
	switch args[0] {
	case "checkpoint":
		return cmdCheckpoint(args[1:], stdout, stderr)
	case "restore":
		return cmdRestore(args[1:], stdout, stderr)
	case "step0":
		return cmdStep0(args[1:], stdout, stderr)
	case "install":
		return cmdInstall(args[1:], stdout, stderr)
	case "status":
		fmt.Fprintln(stdout, "memory-sync v0.1.0 (unencrypted)")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}

func cmdCheckpoint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cwd := fs.String("cwd", ".", "the session's project cwd")
	cfgPath := fs.String("config", "", "path to .memory-sync.toml")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return 1
	}
	uuid := ""
	if fs.NArg() >= 1 {
		uuid = fs.Arg(0)
	} else if env := os.Getenv("CLAUDE_CODE_SESSION_ID"); env != "" {
		uuid = env
	}
	if uuid == "" {
		// auto-detect: newest .jsonl in ProjectDir(cwd) — no hook needed (decision A)
		var err error
		uuid, err = newestSessionUUID(*cwd)
		if err != nil {
			fmt.Fprintln(stderr, "usage: memory-sync checkpoint <session-uuid> [--cwd] [--config]\n  (or set $CLAUDE_CODE_SESSION_ID; auto-detect failed: "+err.Error()+")")
			return 1
		}
	}
	resolvedCfg, err := FindConfig(*cfgPath)
	if err != nil {
		if errors.Is(err, ErrConfigNotFound) {
			fmt.Fprintln(stderr, "No .memory-sync.toml found. Run `memory-sync install` to create one, or use --config <path>.")
			return 1
		}
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "checkpointing %s...\n", uuid)
	cid, err := Checkpoint(uuid, *cwd, resolvedCfg)
	if err != nil {
		if errors.Is(err, ErrGitNotConfigured) {
			fmt.Fprintln(stderr, "Git not configured. Please set your git identity:\n  git config --global user.name 'Your Name'\n  git config --global user.email 'you@example.com'")
			return 1
		}
		fmt.Fprintf(stderr, "checkpoint error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, cid) // result to stdout
	return 0
}

func cmdRestore(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cwd := fs.String("cwd", ".", "the target project cwd")
	cfgPath := fs.String("config", "", "path to .memory-sync.toml")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: memory-sync restore <checkpoint-id> [--cwd] [--config]")
		return 1
	}
	id := fs.Arg(0)
	resolvedCfg, err := FindConfig(*cfgPath)
	if err != nil {
		if errors.Is(err, ErrConfigNotFound) {
			fmt.Fprintln(stderr, "No .memory-sync.toml found. Run `memory-sync install` to create one, or use --config <path>.")
			return 1
		}
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "restoring %s...\n", id)
	uuid, err := Restore(id, *cwd, resolvedCfg)
	if err != nil {
		fmt.Fprintf(stderr, "restore error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "claude --resume %s\n", uuid) // result to stdout
	return 0
}

func cmdStep0(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("step0", flag.ContinueOnError)
	fs.SetOutput(stderr)
	src := fs.String("src", "", "source session JSONL")
	dstCwd := fs.String("dst-cwd", ".", "target cwd (project dir to place the stub)")
	mode := fs.String("mode", "filtered", "filtered | synthetic")
	uuid := fs.String("uuid", "", "stub session UUID (becomes the .jsonl filename)")
	sidecar := fs.String("sidecar", "", "sidecar path (synthetic mode points the LLM here)")
	prompt := fs.String("prompt", "复述 stub 里的内容", "probe prompt")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return 1
	}
	if *src == "" || *uuid == "" {
		fmt.Fprintln(stderr, "usage: memory-sync step0 --src <jsonl> --dst-cwd <dir> --mode filtered|synthetic --uuid <uuid> [--sidecar <path>] [--prompt <text>]")
		return 1
	}
	// validate mode + sidecar BEFORE any filesystem mutation (avoid stray empty project dir on bad input)
	switch *mode {
	case "filtered", "synthetic":
		if *mode == "synthetic" && *sidecar == "" {
			fmt.Fprintln(stderr, "synthetic mode requires --sidecar")
			return 1
		}
	default:
		fmt.Fprintf(stderr, "unknown mode: %s\n", *mode)
		return 1
	}
	targetProject := ProjectDir(*dstCwd)
	if err := os.MkdirAll(targetProject, 0755); err != nil {
		fmt.Fprintf(stderr, "mkdir target project: %v\n", err)
		return 1
	}
	dstJSONL := filepath.Join(targetProject, *uuid+".jsonl")
	switch *mode {
	case "filtered":
		if _, err := BuildFilteredStub(*src, dstJSONL, *uuid); err != nil {
			fmt.Fprintf(stderr, "filtered stub: %v\n", err)
			return 1
		}
	case "synthetic":
		if err := BuildSyntheticStub(dstJSONL, *uuid, *sidecar); err != nil {
			fmt.Fprintf(stderr, "synthetic stub: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stderr, "stub written: %s\n", dstJSONL)
	r, err := RunResumeProbe(*uuid, *dstCwd, *prompt)
	if err != nil {
		fmt.Fprintf(stderr, "probe error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "exit=%d\nstdout=%s\nstderr=%s\n", r.ExitCode, r.Stdout, r.Stderr)
	if r.ExitCode == 0 && r.Stdout != "" {
		fmt.Fprintln(stderr, "VERDICT: PASS (loaded + responded)")
		return 0
	}
	fmt.Fprintln(stderr, "VERDICT: FAIL (non-zero exit or empty response)")
	return 2
}

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "", "sync-store git URL (a private empty repo)")
	projectID := fs.String("project-id", "", "project id (defaults to a hash of the url)")
	cfgPath := fs.String("config", "", "path to write .memory-sync.toml (default: ./.memory-sync.toml)")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return 1
	}
	if *url == "" {
		fmt.Fprintln(stderr, "usage: memory-sync install --url <sync-store-url> [--project-id <id>] [--config <path>]")
		return 1
	}
	if err := validateStoreURL(*url); err != nil {
		fmt.Fprintf(stderr, "invalid store URL: %v\n", err)
		return 1
	}
	if *projectID == "" {
		*projectID = projectIDFromURL(*url)
	}
	// checks: empty/non-empty is a warn (preferred first-time-empty, not enforced);
	// a PUBLIC sync-store is a hard abort — transcripts may contain secrets.
	if empty, err := checkEmpty(*url); err != nil {
		fmt.Fprintf(stderr, "warning: could not probe sync-store (%v); continuing\n", err)
	} else if !empty {
		fmt.Fprintf(stderr, "warning: sync-store is not empty; existing content may conflict\n")
	}
	if vis, err := checkPrivate(*url); err != nil {
		fmt.Fprintf(stderr, "warning: could not check visibility (%v); continuing\n", err)
	} else if vis == "public" {
		fmt.Fprintln(stderr, "sync-store is PUBLIC — transcripts may contain secrets; refusing. Use a private repo.")
		return 1
	}
	cfg := Config{ProjectID: *projectID, Store: StoreConfig{Backend: "git", URL: *url}}
	path := *cfgPath
	if path == "" {
		path = ".memory-sync.toml"
	}
	if err := SaveConfig(path, cfg); err != nil {
		fmt.Fprintf(stderr, "save config: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (project_id=%s, store=%s)\n", path, cfg.ProjectID, cfg.Store.URL)
	return 0
}
