// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIsRetryable(t *testing.T) {
	retryable := []int{0, 429, 500, 502, 503}
	for _, s := range retryable {
		if !IsRetryable(s) {
			t.Errorf("status %d should be retryable", s)
		}
	}
	for _, s := range []int{400, 401, 403, 404} {
		if IsRetryable(s) {
			t.Errorf("status %d should not be retryable", s)
		}
	}
}

func TestRetryDelayMS(t *testing.T) {
	want := []int{500, 1000, 2000}
	for i, w := range want {
		ms, retry := RetryDelayMS(i)
		if !retry || ms != w {
			t.Errorf("RetryDelayMS(%d) = (%d, %v), want (%d, true)", i, ms, retry, w)
		}
	}
	if _, retry := RetryDelayMS(MaxRetries); retry {
		t.Errorf("RetryDelayMS(%d) should stop retrying", MaxRetries)
	}
	if _, retry := RetryDelayMS(-1); retry {
		t.Error("RetryDelayMS(-1) should not retry")
	}
}

func TestEstimateCostUSD(t *testing.T) {
	// gpt-5.4-mini: $0.25/1M in, $2.00/1M out. 1,000,000 in + 1,000,000 out = 0.25 + 2.00 = 2.25.
	if got, ok := EstimateCostUSD("gpt-5.4-mini", Usage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000}); !ok || got != 2.25 {
		t.Errorf("gpt-5.4-mini cost = %v (ok=%v), want 2.25", got, ok)
	}
	// Dated variant resolves to the longest prefix (gpt-5.4-mini, not gpt-5.5).
	if got, ok := EstimateCostUSD("gpt-5.4-mini-2026-01-01", Usage{PromptTokens: 1_000_000}); !ok || got != 0.25 {
		t.Errorf("dated variant cost = %v (ok=%v), want 0.25 via gpt-5.4-mini prefix", got, ok)
	}
	// Unknown model → not ok.
	if _, ok := EstimateCostUSD("some-future-model", Usage{PromptTokens: 100}); ok {
		t.Error("unknown model should return ok=false")
	}
}

func TestFormatCostUSD(t *testing.T) {
	cases := map[float64]string{
		0:         "$0.00",   // no tokens → a true zero
		0.0000005: "<$0.001", // a nonzero spend below a tenth of a cent never reads as $0.00
		0.0003:    "<$0.001", // still below the $0.001 floor
		0.004:     "$0.004",  // sub-cent shows three decimals
		0.009:     "$0.009",  // sub-cent, upper edge
		0.01:      "$0.01",   // a cent or more → two decimals
		1.5:       "$1.50",
		12.345:    "$12.35",
	}
	for cost, want := range cases {
		if got := FormatCostUSD(cost); got != want {
			t.Errorf("FormatCostUSD(%v) = %q, want %q", cost, got, want)
		}
	}
}

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

func TestFinancialContextLine(t *testing.T) {
	c := FinancialContext{NetWorth: "$12,000.00", Income: "$4,200.00", Spending: "$2,600.00", Accounts: 5}
	got := c.Line()
	want := "Net worth $12,000.00, this month's income $4,200.00, spending $2,600.00, across 5 active accounts."
	if got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
	// Guardrail: the rendered line carries only the four aggregate fields we set —
	// no extra detail can leak through a struct with no other text fields.
	for _, leak := range []string{"payee", "account #", "transaction"} {
		if strings.Contains(strings.ToLower(got), leak) {
			t.Errorf("context line unexpectedly contains %q: %q", leak, got)
		}
	}
}

func TestBuildStructuredRequest(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"total":{"type":"number"}},"required":["total"]}`)
	msgs := []Message{{Role: RoleUser, Content: "How much did I spend?"}}
	data, err := BuildStructuredRequest("gpt-5.4-mini", msgs, 0, "spend", schema)
	if err != nil {
		t.Fatalf("BuildStructuredRequest: %v", err)
	}
	var got struct {
		Model          string         `json:"model"`
		Messages       []Message      `json:"messages"`
		ResponseFormat ResponseFormat `json:"response_format"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Model != "gpt-5.4-mini" || len(got.Messages) != 1 {
		t.Errorf("request body wrong: %+v", got)
	}
	rf := got.ResponseFormat
	if rf.Type != "json_schema" || rf.JSONSchema.Name != "spend" || !rf.JSONSchema.Strict {
		t.Errorf("response_format = %+v, want json_schema/spend/strict", rf)
	}
	// The schema must round-trip intact (decodable as the object we passed).
	var sch map[string]any
	if err := json.Unmarshal(rf.JSONSchema.Schema, &sch); err != nil || sch["type"] != "object" {
		t.Errorf("schema not preserved: %s (err %v)", rf.JSONSchema.Schema, err)
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
