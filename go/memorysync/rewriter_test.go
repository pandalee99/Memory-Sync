package memorysync

import (
	_ "embed"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSubsOrdersMostSpecificFirst(t *testing.T) {
	subs := BuildSubs("/home/alice/code/h100", "/home/alice/msync_test/h100")
	first := subs[0][0]
	if !strings.Contains(first, ".claude/projects") {
		t.Errorf("first sub: %q want .claude/projects", first)
	}
	last := subs[len(subs)-1]
	want := Sub{Dashed("/home/alice/code/h100"), Dashed("/home/alice/msync_test/h100")}
	if last != want {
		t.Errorf("last sub: %v want %v", last, want)
	}
}

func TestRewriteStrReplacesPaths(t *testing.T) {
	subs := []Sub{{"/home/alice/code/h100", "/home/bob/h100"}}
	got := RewriteStr("see /home/alice/code/h100/src/main.py", subs)
	want := "see /home/bob/h100/src/main.py"
	if got != want {
		t.Errorf("RewriteStr: %q want %q", got, want)
	}
}

func TestRewriteObjRecurses(t *testing.T) {
	subs := []Sub{{"/home/alice/code/h100", "/home/bob/h100"}}
	in := map[string]any{
		"cwd": "/home/alice/code/h100",
		"msg": map[string]any{"text": []any{"a", "/home/alice/code/h100/x"}},
	}
	got := RewriteObj(in, subs)
	want := map[string]any{
		"cwd": "/home/bob/h100",
		"msg": map[string]any{"text": []any{"a", "/home/bob/h100/x"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RewriteObj: %#v want %#v", got, want)
	}
}

func TestStripTypes(t *testing.T) {
	if !StripTypes["file-history-snapshot"] {
		t.Error("file-history-snapshot should be in StripTypes")
	}
	if !StripTypes["queue-operation"] {
		t.Error("queue-operation should be in StripTypes")
	}
	if StripTypes["user"] {
		t.Error("user should not be in StripTypes")
	}
}

//go:embed testdata/synthetic_session.json
var syntheticFixture []byte

func TestRewriteSessionSyntheticFixture(t *testing.T) {
	tmp := t.TempDir()
	src := tmp + "/src.jsonl"
	// write the fixture as JSONL (one event per line)
	var events []map[string]any
	if err := json.Unmarshal(syntheticFixture, &events); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(src)
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, e := range events {
		enc.Encode(e)
	}
	f.Close()
	dst := tmp + "/out.jsonl"
	stats, err := RewriteSession(src, dst, "/home/alice/code/h100", "/home/alice/msync_test/h100")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Kept != 3 || stats.Stripped != 2 || stats.Orphans != 0 || stats.ResidueSrcCwd != 0 || stats.ResidueSrcDashed != 0 || stats.Malformed != 0 {
		t.Errorf("stats: %+v want Kept=3 Stripped=2 Orphans=0 Residue=0 Malformed=0", stats)
	}
	out, _ := os.ReadFile(dst)
	if strings.Contains(string(out), "/home/alice/code/h100") {
		t.Error("origin path still in output")
	}
	if !strings.Contains(string(out), "/home/alice/msync_test/h100") {
		t.Error("target path not in output")
	}
}

func TestRewriteSessionCountsMalformed(t *testing.T) {
	tmp := t.TempDir()
	src := tmp + "/src.jsonl"
	os.WriteFile(src, []byte(`{"type":"user","uuid":"u1","parentUuid":null,"message":{"content":[{"type":"text","text":"ok"}]}}`+"\nTHIS IS NOT JSON\n"+`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n"), 0644)
	dst := tmp + "/out.jsonl"
	stats, _ := RewriteSession(src, dst, "/home/alice/code/h100", "/home/alice/msync_test/h100")
	if stats.Malformed != 1 || stats.Kept != 2 {
		t.Errorf("malformed=%d kept=%d want 1/2", stats.Malformed, stats.Kept)
	}
}

func TestRewriteSessionResidueNoFalsePositiveWhenDstContainsSrc(t *testing.T) {
	tmp := t.TempDir()
	src := tmp + "/src.jsonl"
	os.WriteFile(src, []byte(`{"type":"user","uuid":"u1","parentUuid":null,"cwd":"/home/alice/code","message":{"content":[{"type":"text","text":"see /home/alice/code/x"}]}}`+"\n"), 0644)
	dst := tmp + "/out.jsonl"
	stats, _ := RewriteSession(src, dst, "/home/alice/code", "/home/alice/codes")
	if stats.ResidueSrcCwd != 0 || stats.ResidueSrcDashed != 0 {
		t.Errorf("residue false-positive: ResidueSrcCwd=%d ResidueSrcDashed=%d want 0/0", stats.ResidueSrcCwd, stats.ResidueSrcDashed)
	}
}

func TestRewriteSessionGoldenCorpusSmoke(t *testing.T) {
	path := os.Getenv("MSYNC_GOLDEN_CORPUS")
	if path == "" {
		t.Skip("MSYNC_GOLDEN_CORPUS not set")
	}
	tmp := t.TempDir()
	dst := tmp + "/out.jsonl"
	// derive srcCwd from the corpus's first cwd event
	data, _ := os.ReadFile(path)
	var firstCwd string
	for _, line := range strings.Split(string(data), "\n") {
		var o map[string]any
		if json.Unmarshal([]byte(line), &o) == nil {
			if cwd, ok := o["cwd"].(string); ok && cwd != "" {
				firstCwd = cwd
				break
			}
		}
	}
	if firstCwd == "" {
		t.Skip("no cwd in corpus")
	}
	stats, err := RewriteSession(path, dst, firstCwd, "/home/dev/msync_test/proj")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Orphans != 0 || stats.ResidueSrcCwd != 0 || stats.ResidueSrcDashed != 0 {
		t.Errorf("corpus stats: Orphans=%d ResidueSrcCwd=%d ResidueSrcDashed=%d want 0/0/0", stats.Orphans, stats.ResidueSrcCwd, stats.ResidueSrcDashed)
	}
}

func TestRewriteSessionResidueDashedNoFalsePositiveWhenDstContainsSrc(t *testing.T) {
	tmp := t.TempDir()
	srcCwd := "/home/alice/code"
	dstCwd := "/home/alice/codes"
	srcD := Dashed(srcCwd)
	dstD := Dashed(dstCwd)
	line := `{"type":"user","uuid":"u1","parentUuid":null,"message":{"content":[{"type":"text","text":"see ` + srcD + `/foo"}]}}`
	src := tmp + "/src.jsonl"
	os.WriteFile(src, []byte(line+"\n"), 0644)
	dst := tmp + "/out.jsonl"
	stats, _ := RewriteSession(src, dst, srcCwd, dstCwd)
	if stats.ResidueSrcDashed != 0 || stats.ResidueSrcCwd != 0 {
		t.Errorf("dashed residue false-positive: ResidueSrcDashed=%d ResidueSrcCwd=%d want 0/0", stats.ResidueSrcDashed, stats.ResidueSrcCwd)
	}
	out, _ := os.ReadFile(dst)
	if !strings.Contains(string(out), dstD) {
		t.Errorf("dstD %q not in output", dstD)
	}
}
