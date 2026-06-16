package ai

import (
	"encoding/json"
	"testing"
)

func TestBuildStructuredVisionRequest(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"transactions":{"type":"array"}},"required":["transactions"],"additionalProperties":false}`)
	raw, err := BuildStructuredVisionRequest("gpt-4o", "sys", "extract", "data:image/png;base64,AAAA", 0.1, "transactions", schema)
	if err != nil {
		t.Fatalf("BuildStructuredVisionRequest: %v", err)
	}
	var got struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		ResponseFormat ResponseFormat `json:"response_format"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The image part must still be present (multimodal user message).
	if len(got.Messages) != 2 || !json.Valid(got.Messages[1].Content) {
		t.Fatalf("messages malformed: %+v", got.Messages)
	}
	rf := got.ResponseFormat
	if rf.Type != "json_schema" || rf.JSONSchema.Name != "transactions" || !rf.JSONSchema.Strict {
		t.Errorf("response_format = %+v, want json_schema/transactions/strict", rf)
	}
	var sch map[string]any
	if err := json.Unmarshal(rf.JSONSchema.Schema, &sch); err != nil || sch["type"] != "object" {
		t.Errorf("schema not preserved: %s (err %v)", rf.JSONSchema.Schema, err)
	}
}

func TestBuildVisionRequest(t *testing.T) {
	const dataURL = "data:image/png;base64,AAAA"
	raw, err := BuildVisionRequest("gpt-4o", "You read receipts.", "Extract the transactions.", dataURL, 0.2)
	if err != nil {
		t.Fatalf("BuildVisionRequest: %v", err)
	}

	var got struct {
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
		Messages    []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Model != "gpt-4o" || got.Temperature != 0.2 {
		t.Errorf("model/temp = %q/%g", got.Model, got.Temperature)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got.Messages))
	}

	// System message content is a plain string.
	var sys string
	if err := json.Unmarshal(got.Messages[0].Content, &sys); err != nil || sys != "You read receipts." {
		t.Errorf("system content = %q (err %v)", sys, err)
	}

	// User message content is an array: a text part and an image_url part.
	var parts []visionContentPart
	if err := json.Unmarshal(got.Messages[1].Content, &parts); err != nil {
		t.Fatalf("user content not an array: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "Extract the transactions." {
		t.Errorf("text part = %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != dataURL {
		t.Errorf("image part = %+v", parts[1])
	}
}

func TestVisionResponseUsesParseResponse(t *testing.T) {
	// The vision reply is a normal chat response, so ParseResponse handles it.
	resp := `{"choices":[{"message":{"role":"assistant","content":"[]"}}]}`
	got, err := ParseResponse([]byte(resp))
	if err != nil || got != "[]" {
		t.Errorf("ParseResponse = %q (err %v)", got, err)
	}
}
