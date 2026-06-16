package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestErrorMessage(t *testing.T) {
	quotaBody := []byte(`{"error":{"message":"You exceeded your current quota","type":"insufficient_quota","code":"insufficient_quota"}}`)
	rateBody := []byte(`{"error":{"message":"Rate limit reached for requests"}}`)
	badReq := []byte(`{"error":{"message":"Unknown parameter: foo"}}`)

	tests := []struct {
		name       string
		status     int
		body       []byte
		wantHas    string // substring the message must contain
		wantNotHas string // substring it must NOT contain (empty to skip)
	}{
		{"unauthorized", 401, nil, "API key", ""},
		{"forbidden", 403, nil, "access", ""},
		{"quota over 429", 429, quotaBody, "out of quota", ""},
		{"rate limit 429", 429, rateBody, "rate-limiting", "quota"},
		{"missing model", 404, nil, "model", ""},
		{"server error", 503, nil, "server trouble", ""},
		{"bad request shows detail", 400, badReq, "Unknown parameter: foo", ""},
		{"unknown status falls back", 418, nil, "HTTP 418", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorMessage(tt.status, tt.body)
			if !strings.Contains(got, tt.wantHas) {
				t.Errorf("ErrorMessage(%d) = %q, want it to contain %q", tt.status, got, tt.wantHas)
			}
			if tt.wantNotHas != "" && strings.Contains(got, tt.wantNotHas) {
				t.Errorf("ErrorMessage(%d) = %q, should not contain %q", tt.status, got, tt.wantNotHas)
			}
		})
	}
}

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
