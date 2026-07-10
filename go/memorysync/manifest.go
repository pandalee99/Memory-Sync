package memorysync

import "encoding/json"

// Manifest is the checkpoint manifest schema.
type Manifest struct {
	ProjectID            string         `json:"project_id"`
	OriginRoot           string         `json:"origin_root"`
	OriginDashed         string         `json:"origin_dashed"`
	SessionUUID          string         `json:"session_uuid"`
	GitHead              string         `json:"git_head"`
	ClaudeVersion        string         `json:"claude_version"`
	SchemaVersion        int            `json:"schema_version"`
	HLCTs                string         `json:"hlc_ts"`
	ParentCheckpoint     *string        `json:"parent_checkpoint"`
	LayerManifest        map[string]any `json:"layer_manifest"`
	RedactionRef         *string        `json:"redaction_ref"`
	TranscriptCompressed bool           `json:"transcript_compressed"`
	SkippedFiles         []string       `json:"skipped_files"`
}

// Dump serializes a Manifest to JSON.
func Dump(m Manifest) string {
	// apply defaults
	if m.ClaudeVersion == "" {
		m.ClaudeVersion = "v2.1.202"
	}
	if m.SchemaVersion == 0 {
		m.SchemaVersion = 1
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

// Load parses a Manifest from JSON, tolerant of extra/unknown fields.
func Load(text string) (Manifest, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return Manifest{}, err
	}
	known := map[string]bool{
		"project_id": true, "origin_root": true, "origin_dashed": true,
		"session_uuid": true, "git_head": true, "claude_version": true,
		"schema_version": true, "hlc_ts": true, "parent_checkpoint": true,
		"layer_manifest": true, "redaction_ref": true,
		"transcript_compressed": true, "skipped_files": true,
	}
	filtered := map[string]any{}
	for k, v := range raw {
		if known[k] {
			filtered[k] = v
		}
	}
	b, _ := json.Marshal(filtered)
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	// apply defaults
	if m.ClaudeVersion == "" {
		m.ClaudeVersion = "v2.1.202"
	}
	if m.SchemaVersion == 0 {
		m.SchemaVersion = 1
	}
	return m, nil
}
