package memorysync

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// StripTypes are event types with no uuid, not in the parentUuid DAG
// (Step 0 verified: stripping them cannot break the resume chain).
var StripTypes = map[string]bool{
	"file-history-snapshot": true,
	"queue-operation":       true,
}

// Sub is one origin->target path substitution.
type Sub [2]string

// BuildSubs returns ordered origin->target path substitutions (most-specific
// first; bare dashed name last as a catch-all).
func BuildSubs(srcCwd, dstCwd string) []Sub {
	home, _ := os.UserHomeDir()
	srcD := Dashed(srcCwd)
	dstD := Dashed(dstCwd)
	srcReal, err := filepath.EvalSymlinks(srcCwd)
	if err != nil {
		srcReal = srcCwd
	}
	dstReal, err := filepath.EvalSymlinks(dstCwd)
	if err != nil {
		dstReal = dstCwd
	}
	return []Sub{
		{filepath.Join(home, ".claude", "projects", srcD), filepath.Join(home, ".claude", "projects", dstD)},
		{srcReal, dstReal},
		{srcCwd, dstCwd},
		{srcD, dstD},
	}
}

// RewriteStr applies the substitutions to a string.
func RewriteStr(s string, subs []Sub) string {
	for _, sub := range subs {
		if sub[0] != "" && strings.Contains(s, sub[0]) {
			s = strings.ReplaceAll(s, sub[0], sub[1])
		}
	}
	return s
}

// RewriteObj recursively rewrites all strings in o (str/list/map).
func RewriteObj(o any, subs []Sub) any {
	switch v := o.(type) {
	case string:
		return RewriteStr(v, subs)
	case []any:
		for i, x := range v {
			v[i] = RewriteObj(x, subs)
		}
		return v
	case map[string]any:
		for k, val := range v {
			v[k] = RewriteObj(val, subs)
		}
		return v
	}
	return o
}

// RewriteStats holds the Step-0 invariants from a rewrite pass.
type RewriteStats struct {
	Kept             int
	Stripped         int
	Orphans          int
	ResidueSrcCwd    int
	ResidueSrcDashed int
	Malformed        int
}

// RewriteSession reads src JSONL, strips machine-local events, rewrites paths,
// writes dst JSONL. Returns stats incl. orphan + residue checks.
func RewriteSession(srcJSONL, dstJSONL, srcCwd, dstCwd string) (RewriteStats, error) {
	subs := BuildSubs(srcCwd, dstCwd)
	srcD := Dashed(srcCwd)
	dstD := Dashed(dstCwd)
	var stats RewriteStats
	in, err := os.Open(srcJSONL)
	if err != nil {
		return stats, err
	}
	defer in.Close()
	out, err := os.Create(dstJSONL)
	if err != nil {
		return stats, err
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024) // for long lines
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	var objs []map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var o map[string]any
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			stats.Malformed++
			continue
		}
		t, _ := o["type"].(string)
		if StripTypes[t] {
			stats.Stripped++
			continue
		}
		o = RewriteObj(o, subs).(map[string]any)
		enc.Encode(o)
		objs = append(objs, o)
		stats.Kept++
	}
	out.Close()
	if err := scanner.Err(); err != nil {
		return stats, err
	}
	// orphan check
	uuids := map[string]bool{}
	for _, o := range objs {
		if u, ok := o["uuid"].(string); ok {
			uuids[u] = true
		}
	}
	for _, o := range objs {
		if p, ok := o["parentUuid"].(string); ok && p != "" && !uuids[p] {
			stats.Orphans++
		}
	}
	// residue check (dst-span masking for both cwd + dashed forms)
	data, _ := os.ReadFile(dstJSONL)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, srcCwd) {
			masked := line
			if dstCwd != "" && strings.Contains(dstCwd, srcCwd) {
				masked = strings.ReplaceAll(line, dstCwd, strings.Repeat("\x00", len(dstCwd)))
			}
			if strings.Contains(masked, srcCwd) {
				stats.ResidueSrcCwd++
			}
		}
		if strings.Contains(line, srcD) {
			masked := line
			if dstD != "" && strings.Contains(dstD, srcD) {
				masked = strings.ReplaceAll(line, dstD, strings.Repeat("\x00", len(dstD)))
			}
			if strings.Contains(masked, srcD) {
				stats.ResidueSrcDashed++
			}
		}
	}
	return stats, nil
}
