package memorysync

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// StubStats reports filter counts for BuildFilteredStub.
type StubStats struct {
	Total   int
	Kept    int
	Dropped int
}

// eventUUID returns e["uuid"] as a string ("", if absent/non-string).
func eventUUID(e map[string]any) string {
	s, _ := e["uuid"].(string)
	return s
}

// eventParent returns e["parentUuid"] as a string; nil/null -> "" (root).
func eventParent(e map[string]any) string {
	if e["parentUuid"] == nil {
		return ""
	}
	s, _ := e["parentUuid"].(string)
	return s
}

// isPureTextEvent reports whether e is a user/assistant event whose
// message.content is a non-empty list of blocks ALL of type "text".
// (Drops tool_use / tool_result / attachment / base64 events.)
func isPureTextEvent(e map[string]any) bool {
	t, _ := e["type"].(string)
	if t != "user" && t != "assistant" {
		return false
	}
	msg, ok := e["message"].(map[string]any)
	if !ok {
		return false
	}
	content, ok := msg["content"].([]any)
	if !ok || len(content) == 0 {
		return false
	}
	for _, b := range content {
		blk, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if bt, _ := blk["type"].(string); bt != "text" {
			return false
		}
	}
	return true
}

// readAllEvents parses a JSONL file into a slice of event objects.
// 16MB per-line buffer matches rewriter.go:99 (long tool-result lines).
func readAllEvents(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var all []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 16*1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		all = append(all, e)
	}
	return all, sc.Err()
}

// ValidateDAG returns nil iff every event's parentUuid is empty (root) or
// references a uuid present in the set. Reuses step0.py's orphan check.
func ValidateDAG(events []map[string]any) error {
	uuids := map[string]struct{}{}
	for _, e := range events {
		if u := eventUUID(e); u != "" {
			uuids[u] = struct{}{}
		}
	}
	for _, e := range events {
		p := eventParent(e)
		if p == "" {
			continue
		}
		if _, ok := uuids[p]; !ok {
			return fmt.Errorf("orphan parentUuid %q not in event set", p)
		}
	}
	return nil
}

// randomUUIDString returns a v4 uuid using crypto/rand (no external dep).
func randomUUIDString() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// BuildSyntheticStub writes a minimal 1-event stub transcript: a single user
// message pointing the LLM at the sidecar. Sub-test B probe.
func BuildSyntheticStub(dstJSONL, stubUUID, sidecarPath string) error {
	text := fmt.Sprintf(
		"Prior session compacted. Full transcript at %s. Run the restore-adjust skill to pull relevant prior turns within your context budget; durable facts are in memory/.",
		sidecarPath,
	)
	event := map[string]any{
		"type":       "user",
		"uuid":       randomUUIDString(),
		"parentUuid": "",
		"sessionId":  stubUUID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "text",
				"text": text,
			}},
		},
	}
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal stub event: %w", err)
	}
	return os.WriteFile(dstJSONL, append(line, '\n'), 0644)
}

// BuildFilteredStub reads srcJSONL, keeps user/assistant pure-text events
// (drops tool_use / tool_result / attachment events — and thus base64, which
// lives in those events), splices the DAG (repoints each kept event's
// parentUuid to its nearest KEPT ancestor, walking through dropped nodes),
// rewrites sessionId to stubUUID, and writes dstJSONL. Sub-test A probe.
// No path rewriting (orthogonal to the acceptance question; proven Step 0).
func BuildFilteredStub(srcJSONL, dstJSONL, stubUUID string) (StubStats, error) {
	var stats StubStats
	all, err := readAllEvents(srcJSONL)
	if err != nil {
		return stats, fmt.Errorf("read src: %w", err)
	}
	stats.Total = len(all)

	// Full uuid -> parentUuid map (kept + dropped) for chain walking.
	allParent := map[string]string{}
	for _, e := range all {
		allParent[eventUUID(e)] = eventParent(e)
	}

	// Filter + rewrite sessionId.
	var kept []map[string]any
	keptSet := map[string]bool{}
	for _, e := range all {
		if !isPureTextEvent(e) {
			stats.Dropped++
			continue
		}
		e["sessionId"] = stubUUID
		kept = append(kept, e)
		keptSet[eventUUID(e)] = true
		stats.Kept++
	}

	// Splice: repoint each kept event's parent to nearest kept ancestor.
	for _, e := range kept {
		p := eventParent(e)
		visited := map[string]bool{}
		for p != "" && !keptSet[p] {
			if visited[p] {
				p = "" // cycle guard: treat as root
				break
			}
			visited[p] = true
			p = allParent[p] // walk up through dropped nodes
		}
		e["parentUuid"] = p // "" -> root
	}

	// Write.
	out, err := os.Create(dstJSONL)
	if err != nil {
		return stats, fmt.Errorf("create dst: %w", err)
	}
	defer out.Close()
	for _, e := range kept {
		line, err := json.Marshal(e)
		if err != nil {
			return stats, fmt.Errorf("marshal: %w", err)
		}
		if _, err := fmt.Fprintln(out, string(line)); err != nil {
			return stats, fmt.Errorf("write: %w", err)
		}
	}
	return stats, nil
}

// ProbeResult captures a `claude --resume -p` probe outcome.
type ProbeResult struct {
	UUID     string
	Prompt   string
	Stdout   string
	Stderr   string
	ExitCode int
}

// resumeProbeArgs builds the claude CLI args for a non-interactive resume probe.
func resumeProbeArgs(uuid, prompt string) []string {
	return []string{"--resume", uuid, "-p", prompt}
}

// RunResumeProbe runs `claude --resume <uuid> -p <prompt>` from cwd and returns
// captured stdout/stderr + exit code. A non-zero exit is a probe RESULT (no
// error returned); a failure to start claude is an error. Exercised live in
// Task 5 (integration); unit-tested here only via resumeProbeArgs.
func RunResumeProbe(uuid, cwd, prompt string) (ProbeResult, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return ProbeResult{}, fmt.Errorf("claude not on PATH: %w", err)
	}
	cmd := exec.Command("claude", resumeProbeArgs(uuid, prompt)...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	r := ProbeResult{UUID: uuid, Prompt: prompt, Stdout: stdout.String(), Stderr: stderr.String()}
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			r.ExitCode = ee.ExitCode()
			return r, nil
		}
		return r, fmt.Errorf("run claude: %w", runErr)
	}
	r.ExitCode = 0
	return r, nil
}
