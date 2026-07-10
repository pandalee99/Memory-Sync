package memorysync

import (
	"strings"
	"testing"
)

func TestScanFindsAnthropic(t *testing.T) {
	text := "my key is bsk-ant-FAKEFAKEFAKEFAKEFAKE2024 plus sk-ant-FAKEFAKEFAKEFAKEFAKE2024"
	matches := ScanText(text)
	found := false
	for _, m := range matches {
		if m.Kind == "anthropic" {
			found = true
		}
	}
	if !found {
		t.Error("anthropic key not found")
	}
}

func TestRedactReplacesAndKeepsMap(t *testing.T) {
	text := "token=FAKESECRET12345 password=FAKEPASS2abc"
	red, mapping := RedactText(text)
	if strings.Contains(red, "FAKESECRET12345") || strings.Contains(red, "FAKEPASS2abc") {
		t.Error("secret not redacted")
	}
	if !strings.Contains(red, "<redacted:") {
		t.Error("placeholder not inserted")
	}
	vals := map[string]bool{}
	for _, v := range mapping {
		vals[v] = true
	}
	if !vals["FAKESECRET12345"] || !vals["FAKEPASS2abc"] {
		t.Error("originals not in mapping")
	}
}

func TestRedactNoFalsePositive(t *testing.T) {
	red, mapping := RedactText("just a normal sentence with no secrets")
	if red != "just a normal sentence with no secrets" || len(mapping) != 0 {
		t.Error("false positive on plain text")
	}
}

func TestRedactObjNoCollision(t *testing.T) {
	o := map[string]any{"a": "token=FAKESECRET1", "b": "token=FAKESECRET2"}
	red, mapping := RedactObj(o)
	ra := red.(map[string]any)["a"].(string)
	rb := red.(map[string]any)["b"].(string)
	if ra == rb {
		t.Error("placeholders collided")
	}
	if len(mapping) != 2 {
		t.Errorf("mapping size: %d want 2", len(mapping))
	}
	vals := map[string]bool{}
	for _, v := range mapping {
		vals[v] = true
	}
	if !vals["FAKESECRET1"] || !vals["FAKESECRET2"] {
		t.Error("originals not both in mapping")
	}
}

func TestScanFindsAwsAndGithub(t *testing.T) {
	text := "aws AKIAIOSFODNN7EXAMPLE and github ghp_FAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKE"
	matches := ScanText(text)
	kinds := map[string]bool{}
	for _, m := range matches {
		kinds[m.Kind] = true
	}
	if !kinds["aws"] || !kinds["github_pat"] {
		t.Errorf("aws/github_pat not found: %+v", kinds)
	}
}

func TestScanFindsPrivateKeyBlock(t *testing.T) {
	text := "-----BEGIN RSA PRIVATE KEY-----\nMIIBfake...\n-----END RSA PRIVATE KEY-----"
	matches := ScanText(text)
	found := false
	for _, m := range matches {
		if m.Kind == "private_key" {
			found = true
		}
	}
	if !found {
		t.Error("private_key not found")
	}
}

func TestRedactObjRecurses(t *testing.T) {
	o := map[string]any{
		"cwd": "/a",
		"msg": map[string]any{"text": []any{"token=FAKESECRET12345", "plain"}},
		"n":   float64(3),
	}
	red, mapping := RedactObj(o)
	ra := red.(map[string]any)
	if ra["n"].(float64) != 3 {
		t.Errorf("n not preserved: %v", ra["n"])
	}
	found := false
	for _, v := range mapping {
		if strings.Contains(v, "FAKESECRET12345") {
			found = true
		}
	}
	if !found {
		t.Error("FAKESECRET12345 not in mapping")
	}
	msg := ra["msg"].(map[string]any)
	text := msg["text"].([]any)
	if strings.Contains(text[0].(string), "FAKESECRET12345") {
		t.Error("secret not redacted in nested list")
	}
}
