package memorysync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func parseEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		out = append(out, e)
	}
	return out
}

func TestBuildSyntheticStub(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "stub.jsonl")
	sidecar := "/tmp/sidecar.jsonl"
	stubUUID := "11111111-2222-3333-4444-555555555555"
	if err := BuildSyntheticStub(dst, stubUUID, sidecar); err != nil {
		t.Fatal(err)
	}
	events := parseEvents(t, dst)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	e := events[0]
	if e["type"] != "user" {
		t.Errorf("type: want user, got %v", e["type"])
	}
	if e["sessionId"] != stubUUID {
		t.Errorf("sessionId: want %s, got %v", stubUUID, e["sessionId"])
	}
	if eventParent(e) != "" {
		t.Errorf("parentUuid: want empty (root), got %q", eventParent(e))
	}
	msg, ok := e["message"].(map[string]any)
	if !ok {
		t.Fatalf("message not object: %T", e["message"])
	}
	if msg["role"] != "user" {
		t.Errorf("role: want user, got %v", msg["role"])
	}
	content, _ := msg["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content: want 1 block, got %d", len(content))
	}
	blk, _ := content[0].(map[string]any)
	if !strings.Contains(blk["text"].(string), sidecar) {
		t.Errorf("stub text missing sidecar path %q", sidecar)
	}
	if err := ValidateDAG(events); err != nil {
		t.Errorf("DAG invalid: %v", err)
	}
}

func TestBuildFilteredStub(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "stub.jsonl")
	stubUUID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	stats, err := BuildFilteredStub("testdata/step0_source.jsonl", dst, stubUUID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 4 {
		t.Errorf("Total: want 4, got %d", stats.Total)
	}
	if stats.Kept != 2 || stats.Dropped != 2 {
		t.Errorf("Kept/Dropped: want 2/2, got %d/%d", stats.Kept, stats.Dropped)
	}
	events := parseEvents(t, dst)
	if len(events) != 2 {
		t.Fatalf("want 2 kept events, got %d", len(events))
	}
	if err := ValidateDAG(events); err != nil {
		t.Errorf("DAG invalid: %v", err)
	}
	// a2's parent must be spliced to u1 (its dropped ancestors a1/u2 skipped).
	uuids := map[string]string{} // uuid -> parent
	for _, e := range events {
		uuids[eventUUID(e)] = eventParent(e)
	}
	a2Parent := ""
	for u := range uuids {
		if u == "a2" {
			a2Parent = uuids[u]
		}
	}
	if a2Parent != "u1" {
		t.Errorf("a2 parent after splice: want u1, got %q", a2Parent)
	}
	for _, e := range events {
		if e["sessionId"] != stubUUID {
			t.Errorf("sessionId not rewritten: %v", e["sessionId"])
		}
		if !isPureTextEvent(e) {
			t.Errorf("kept a non-pure-text event: %v", e["type"])
		}
	}
}

// TestBuildFilteredStubCyclic ensures the splice walk terminates on a cyclic
// source. Fixture: u1 (root text, kept) -> d1 (tool_result, dropped, parent=d2)
// and d2 (tool_result, dropped, parent=d1) form a cycle among dropped nodes;
// k1 (text, kept, parent=d1) walks d1->d2->d1. The cycle guard must break and
// treat k1 as a root (parent="").
func TestBuildFilteredStubCyclic(t *testing.T) {
	const src = `{"type":"user","uuid":"u1","parentUuid":null,"sessionId":"sid","timestamp":"2026-07-09T00:00:00Z","message":{"role":"user","content":[{"type":"text","text":"root"}]}}
{"type":"user","uuid":"d1","parentUuid":"d2","sessionId":"sid","timestamp":"2026-07-09T00:00:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"r"}]}}
{"type":"user","uuid":"d2","parentUuid":"d1","sessionId":"sid","timestamp":"2026-07-09T00:00:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"r"}]}}
{"type":"user","uuid":"k1","parentUuid":"d1","sessionId":"sid","timestamp":"2026-07-09T00:00:00Z","message":{"role":"user","content":[{"type":"text","text":"k1"}]}}
`
	srcPath := filepath.Join(t.TempDir(), "cyclic.jsonl")
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "stub.jsonl")
	stubUUID := "cyc-uuid"

	// Run in a goroutine with a timeout so a regression (infinite loop) fails
	// the test instead of hanging the suite.
	type result struct {
		stats StubStats
		err   error
	}
	done := make(chan result, 1)
	go func() {
		s, err := BuildFilteredStub(srcPath, dst, stubUUID)
		done <- result{s, err}
	}()
	var stats StubStats
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("BuildFilteredStub: %v", r.err)
		}
		stats = r.stats
	case <-time.After(2 * time.Second):
		t.Fatal("BuildFilteredStub hung on cyclic input")
	}

	if stats.Total != 4 {
		t.Errorf("Total: want 4, got %d", stats.Total)
	}
	if stats.Kept != 2 || stats.Dropped != 2 {
		t.Errorf("Kept/Dropped: want 2/2, got %d/%d", stats.Kept, stats.Dropped)
	}
	events := parseEvents(t, dst)
	if len(events) != 2 {
		t.Fatalf("want 2 kept events, got %d", len(events))
	}
	if err := ValidateDAG(events); err != nil {
		t.Errorf("DAG invalid: %v", err)
	}
	for _, e := range events {
		if e["sessionId"] != stubUUID {
			t.Errorf("sessionId not rewritten: %v", e["sessionId"])
		}
		if eventUUID(e) == "k1" && eventParent(e) != "" {
			t.Errorf("k1 parent after cycle guard: want empty (root), got %q", eventParent(e))
		}
	}
}

func TestResumeProbeArgs(t *testing.T) {
	got := resumeProbeArgs("abc-123", "复述")
	want := []string{"--resume", "abc-123", "-p", "复述"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resumeProbeArgs: got %v, want %v", got, want)
	}
}

func TestRunResumeProbeCapturesOutput(t *testing.T) {
	// Create a fake `claude` that prints a known marker and exits 0.
	dir := t.TempDir()
	fake := filepath.Join(dir, "claude")
	script := "#!/bin/sh\necho PROBE-MARKER-42\n"
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r, err := RunResumeProbe("any-uuid", t.TempDir(), "any-prompt")
	if err != nil {
		t.Fatalf("RunResumeProbe: %v", err)
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode: want 0, got %d", r.ExitCode)
	}
	if !strings.Contains(r.Stdout, "PROBE-MARKER-42") {
		t.Errorf("stdout not captured: got %q (want to contain PROBE-MARKER-42)", r.Stdout)
	}
}
