package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildRequestRoundTrip(t *testing.T) {
	msgs := []Message{
		{Role: RoleSystem, Content: "You are helpful."},
		{Role: RoleUser, Content: "Explain my month."},
	}
	data, err := BuildRequest("gpt-x", msgs, 0.2)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	var req ChatRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Model != "gpt-x" || len(req.Messages) != 2 || req.Messages[1].Content != "Explain my month." {
		t.Errorf("round-trip request = %+v", req)
	}
	if req.Temperature != 0.2 {
		t.Errorf("temperature = %g, want 0.2", req.Temperature)
	}
}

func TestParseResponseContent(t *testing.T) {
	body := `{"choices":[{"message":{"role":"assistant","content":"You saved 64% this month."}}],"usage":{"total_tokens":42}}`
	got, err := ParseResponse([]byte(body))
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if got != "You saved 64% this month." {
		t.Errorf("content = %q", got)
	}
	if u := ParseUsage([]byte(body)); u.TotalTokens != 42 {
		t.Errorf("usage total = %d, want 42", u.TotalTokens)
	}
}

func TestParseResponseAPIError(t *testing.T) {
	body := `{"error":{"message":"Invalid API key","type":"auth"}}`
	_, err := ParseResponse([]byte(body))
	if err == nil || !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("expected API error surfaced, got %v", err)
	}
}

func TestParseResponseErrors(t *testing.T) {
	if _, err := ParseResponse([]byte(`{"choices":[]}`)); err == nil {
		t.Error("expected empty-response error")
	}
	if _, err := ParseResponse([]byte(`not json`)); err == nil {
		t.Error("expected decode error")
	}
}
