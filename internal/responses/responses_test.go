package responses

import (
	"encoding/json"
	"strings"
	"testing"
)

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	return m
}

func TestBuildRequest(t *testing.T) {
	b, err := BuildRequest("gpt-5.5", "be helpful",
		[]Message{{Role: "user", Text: "hi"}},
		[]Tool{{Name: "q", Description: "query", Parameters: json.RawMessage(`{"type":"object"}`)}},
		"medium", true, 0)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	m := decode(t, b)
	if m["model"] != "gpt-5.5" {
		t.Errorf("model = %v", m["model"])
	}
	if m["instructions"] != "be helpful" {
		t.Errorf("system prompt should be 'instructions', got %v", m["instructions"])
	}
	if m["max_output_tokens"].(float64) != defaultMaxOutput {
		t.Errorf("max_output_tokens = %v, want default", m["max_output_tokens"])
	}
	if r, ok := m["reasoning"].(map[string]any); !ok || r["effort"] != "medium" {
		t.Errorf("reasoning effort not set: %v", m["reasoning"])
	}
	if m["stream"] != true {
		t.Errorf("stream should be set")
	}
	input := m["input"].([]any)
	if input[0].(map[string]any)["content"] != "hi" {
		t.Errorf("input content = %v", input[0])
	}
	tool := m["tools"].([]any)[0].(map[string]any)
	if tool["type"] != "function" || tool["name"] != "q" {
		t.Errorf("tool shape wrong: %v", tool)
	}
	if _, ok := tool["parameters"]; !ok {
		t.Error("tool should carry parameters")
	}
}

func TestBuildRequestOmitsEmpty(t *testing.T) {
	b, _ := BuildRequest("m", "", []Message{{Text: "x"}}, nil, "", false, 64)
	m := decode(t, b)
	for _, k := range []string{"instructions", "reasoning", "stream", "tools"} {
		if _, ok := m[k]; ok {
			t.Errorf("%q should be omitted when empty/false", k)
		}
	}
	if m["max_output_tokens"].(float64) != 64 {
		t.Errorf("explicit max_output_tokens not honored: %v", m["max_output_tokens"])
	}
}

func TestParseResponseText(t *testing.T) {
	res, err := ParseResponse([]byte(`{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello "},{"type":"output_text","text":"world"}]}],"usage":{"input_tokens":9,"output_tokens":2}}`))
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if res.Text != "Hello world" {
		t.Errorf("text = %q", res.Text)
	}
	if res.Usage.InputTokens != 9 || res.Usage.OutputTokens != 2 {
		t.Errorf("usage = %+v", res.Usage)
	}
	if res.Status != "completed" {
		t.Errorf("status = %q", res.Status)
	}
}

func TestParseResponseToolCall(t *testing.T) {
	res, err := ParseResponse([]byte(`{"status":"completed","output":[{"type":"function_call","call_id":"c1","name":"q","arguments":"{\"x\":1}"}],"usage":{"input_tokens":3,"output_tokens":4}}`))
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "q" || res.ToolCalls[0].CallID != "c1" {
		t.Errorf("tool calls = %+v", res.ToolCalls)
	}
	if string(res.ToolCalls[0].Arguments) != `"{\"x\":1}"` {
		t.Errorf("arguments = %s", res.ToolCalls[0].Arguments)
	}
}

func TestParseResponseError(t *testing.T) {
	_, err := ParseResponse([]byte(`{"error":{"type":"rate_limit_exceeded","message":"slow down"}}`))
	if err == nil || !strings.Contains(err.Error(), "rate_limit_exceeded") {
		t.Errorf("error object should surface, got %v", err)
	}
}
