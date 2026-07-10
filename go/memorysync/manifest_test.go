package memorysync

import "testing"

func TestManifestRoundtripMinimal(t *testing.T) {
	m := Manifest{ProjectID: "p", OriginRoot: "/a", OriginDashed: "-a", SessionUUID: "11111111-2222-3333-4444-555555555555", GitHead: "abc123"}
	text := Dump(m)
	m2, err := Load(text)
	if err != nil {
		t.Fatal(err)
	}
	if m2.ProjectID != "p" || m2.SessionUUID != "11111111-2222-3333-4444-555555555555" || m2.ClaudeVersion != "v2.1.202" || m2.SchemaVersion != 1 {
		t.Errorf("roundtrip: %+v want defaults ClaudeVersion=v2.1.202 SchemaVersion=1", m2)
	}
}

func TestManifestLoadIgnoresUnknownFields(t *testing.T) {
	text := `{"project_id":"p","origin_root":"/a","origin_dashed":"-a","session_uuid":"11111111-2222-3333-4444-555555555555","git_head":"h","unknown_future_field":"ignored","another":99}`
	m2, err := Load(text)
	if err != nil {
		t.Fatal(err)
	}
	if m2.ProjectID != "p" || m2.GitHead != "h" {
		t.Errorf("load with extra fields: %+v", m2)
	}
}

func TestManifestTranscriptCompressedRoundtrip(t *testing.T) {
	m := Manifest{
		ProjectID:            "p",
		OriginRoot:           "/a",
		OriginDashed:         "-a",
		SessionUUID:          "11111111-2222-3333-4444-555555555555",
		GitHead:              "h",
		TranscriptCompressed: true,
		SkippedFiles:         []string{"tool-results/big.png"},
	}
	text := Dump(m)
	m2, err := Load(text)
	if err != nil {
		t.Fatal(err)
	}
	if !m2.TranscriptCompressed || len(m2.SkippedFiles) != 1 || m2.SkippedFiles[0] != "tool-results/big.png" {
		t.Errorf("roundtrip: compressed=%v skipped=%v", m2.TranscriptCompressed, m2.SkippedFiles)
	}
}
