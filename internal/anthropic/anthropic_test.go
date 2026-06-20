package anthropic

import (
	"encoding/json"
	"strings"
	"testing"
)

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("request is not valid JSON: %v", err)
	}
	return m
}

func TestBuildRequestBasics(t *testing.T) {
	b, err := BuildRequest("claude-3-5-sonnet-latest", "be helpful",
		[]Message{{Role: "user", Text: "hi"}}, nil, 0, true)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	m := decode(t, b)
	if m["model"] != "claude-3-5-sonnet-latest" {
		t.Errorf("model = %v", m["model"])
	}
	if m["max_tokens"].(float64) != defaultMaxTokens {
		t.Errorf("max_tokens = %v, want default %d", m["max_tokens"], defaultMaxTokens)
	}
	if m["system"] != "be helpful" {
		t.Errorf("system should be a top-level field, got %v", m["system"])
	}
	if m["stream"] != true {
		t.Errorf("stream should be set")
	}
	msgs := m["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["content"] != "hi" {
		t.Errorf("messages = %v, want one user 'hi'", msgs)
	}
	// No system when blank, no tools when none, no stream when false.
	b2, _ := BuildRequest("m", "", []Message{{Text: "x"}}, nil, 50, false)
	m2 := decode(t, b2)
	if _, ok := m2["system"]; ok {
		t.Error("blank system should be omitted")
	}
	if _, ok := m2["stream"]; ok {
		t.Error("stream=false should be omitted")
	}
	if m2["max_tokens"].(float64) != 50 {
		t.Errorf("explicit max_tokens not honored: %v", m2["max_tokens"])
	}
}

func TestBuildRequestTools(t *testing.T) {
	b, _ := BuildRequest("m", "", []Message{{Text: "x"}},
		[]Tool{{Name: "query", Description: "d", InputSchema: json.RawMessage(`{"type":"object"}`)}}, 0, false)
	m := decode(t, b)
	tools := m["tools"].([]any)
	tool := tools[0].(map[string]any)
	if tool["name"] != "query" {
		t.Errorf("tool name = %v", tool["name"])
	}
	if _, ok := tool["input_schema"]; !ok {
		t.Error("tool should carry input_schema (anthropic), not parameters")
	}
}

func TestBuildRequestVision(t *testing.T) {
	b, _ := BuildRequest("m", "", []Message{{Role: "user", Text: "what's this?", ImageBase64: "QUJD", ImageMIME: "image/png"}}, nil, 0, false)
	m := decode(t, b)
	content := m["messages"].([]any)[0].(map[string]any)["content"].([]any)
	img := content[0].(map[string]any)
	if img["type"] != "image" {
		t.Fatalf("first block should be image: %v", img)
	}
	src := img["source"].(map[string]any)
	if src["type"] != "base64" || src["media_type"] != "image/png" || src["data"] != "QUJD" {
		t.Errorf("image source wrong: %v", src)
	}
	if content[1].(map[string]any)["text"] != "what's this?" {
		t.Errorf("text block should follow the image: %v", content[1])
	}
}

func TestParseResponseText(t *testing.T) {
	res, err := ParseResponse([]byte(`{"type":"message","content":[{"type":"text","text":"Hello "},{"type":"text","text":"world"}],"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":3}}`))
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if res.Text != "Hello world" {
		t.Errorf("text = %q, want concatenated 'Hello world'", res.Text)
	}
	if res.Usage.InputTokens != 12 || res.Usage.OutputTokens != 3 {
		t.Errorf("usage = %+v", res.Usage)
	}
	if res.StopReason != "end_turn" {
		t.Errorf("stop reason = %q", res.StopReason)
	}
}

func TestParseResponseToolUse(t *testing.T) {
	res, err := ParseResponse([]byte(`{"type":"message","content":[{"type":"text","text":"let me check"},{"type":"tool_use","id":"t1","name":"query","input":{"x":1}}],"stop_reason":"tool_use","usage":{"input_tokens":5,"output_tokens":7}}`))
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "query" || res.ToolCalls[0].ID != "t1" {
		t.Errorf("tool calls = %+v", res.ToolCalls)
	}
	if string(res.ToolCalls[0].Input) != `{"x":1}` {
		t.Errorf("tool input = %s", res.ToolCalls[0].Input)
	}
}

func TestParseResponseError(t *testing.T) {
	_, err := ParseResponse([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"slow down"}}`))
	if err == nil || !strings.Contains(err.Error(), "overloaded_error") {
		t.Errorf("error envelope should return an error mentioning the type, got %v", err)
	}
}
