// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildResponsesToolRequest(t *testing.T) {
	params := json.RawMessage(`{"type":"object","properties":{}}`)
	tools := []Tool{FunctionTool("find_dupes", "find duplicates", params)}
	msgs := []Message{
		{Role: RoleSystem, Content: "you are helpful"},
		{Role: RoleUser, Content: "any dupes?"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: FunctionCall{Name: "find_dupes", Arguments: "{}"}}}},
		ToolResultMessage("call_1", "find_dupes", "found 1 group"),
	}
	body, err := BuildResponsesToolRequest("gpt-5.6-sol", msgs, 0, "high", tools)
	if err != nil {
		t.Fatalf("BuildResponsesToolRequest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// reasoning.effort is set with tools present (the whole point — /chat/completions can't).
	rz, _ := got["reasoning"].(map[string]any)
	if rz == nil || rz["effort"] != "high" {
		t.Errorf("reasoning.effort = %v, want high", got["reasoning"])
	}
	if got["tool_choice"] != "auto" {
		t.Errorf("tool_choice = %v, want auto", got["tool_choice"])
	}
	if got["store"] != false {
		t.Errorf("store = %v, want false", got["store"])
	}
	// temperature omitted when 0 (reasoning models reject a custom temp).
	if _, ok := got["temperature"]; ok {
		t.Error("temperature should be omitted when 0")
	}
	// Tools are flat (name at the top level, not nested under function).
	toolsArr, _ := got["tools"].([]any)
	if len(toolsArr) != 1 {
		t.Fatalf("tools len = %d, want 1", len(toolsArr))
	}
	tool0, _ := toolsArr[0].(map[string]any)
	if tool0["type"] != "function" || tool0["name"] != "find_dupes" {
		t.Errorf("tool[0] = %v, want flat function find_dupes", tool0)
	}
	// input: system, user, function_call, function_call_output — in order.
	input, _ := got["input"].([]any)
	if len(input) != 4 {
		t.Fatalf("input len = %d, want 4: %s", len(input), string(body))
	}
	i0, _ := input[0].(map[string]any)
	if i0["role"] != "system" {
		t.Errorf("input[0].role = %v, want system", i0["role"])
	}
	i2, _ := input[2].(map[string]any)
	if i2["type"] != "function_call" || i2["call_id"] != "call_1" || i2["name"] != "find_dupes" {
		t.Errorf("input[2] = %v, want function_call call_1 find_dupes", i2)
	}
	i3, _ := input[3].(map[string]any)
	if i3["type"] != "function_call_output" || i3["call_id"] != "call_1" || i3["output"] != "found 1 group" {
		t.Errorf("input[3] = %v, want function_call_output call_1", i3)
	}
}

func TestBuildResponsesToolRequestNoEffort(t *testing.T) {
	body, err := BuildResponsesToolRequest("gpt-4o", []Message{{Role: RoleUser, Content: "hi"}}, 0.4, "", nil)
	if err != nil {
		t.Fatalf("BuildResponsesToolRequest: %v", err)
	}
	s := string(body)
	if strings.Contains(s, "reasoning") {
		t.Error("reasoning should be omitted when effort is empty")
	}
	if !strings.Contains(s, "\"temperature\":0.4") {
		t.Errorf("temperature 0.4 should be present for a non-reasoning model: %s", s)
	}
}

func TestParseResponsesChat(t *testing.T) {
	// A reply that requests a tool call (with a leading reasoning item to ignore).
	body := []byte(`{"output":[
		{"type":"reasoning","id":"rs_1","summary":[]},
		{"type":"function_call","id":"fc_1","call_id":"call_abc","name":"find_dupes","arguments":"{\"match\":\"x\"}"}
	],"usage":{"input_tokens":100,"output_tokens":20}}`)
	msg, usage, err := ParseResponsesChat(body)
	if err != nil {
		t.Fatalf("ParseResponsesChat: %v", err)
	}
	if !WantsTools(msg) || len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %+v", msg)
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_abc" || tc.Function.Name != "find_dupes" || tc.Function.Arguments != `{"match":"x"}` {
		t.Errorf("tool call = %+v", tc)
	}
	if usage.TotalTokens != 120 {
		t.Errorf("usage total = %d, want 120", usage.TotalTokens)
	}

	// A final text reply.
	final := []byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"No other duplicates."}]}],"usage":{"input_tokens":10,"output_tokens":5}}`)
	msg2, _, err := ParseResponsesChat(final)
	if err != nil {
		t.Fatalf("ParseResponsesChat(final): %v", err)
	}
	if WantsTools(msg2) || msg2.Content != "No other duplicates." {
		t.Errorf("final msg = %+v", msg2)
	}

	// An API error surfaces.
	if _, _, err := ParseResponsesChat([]byte(`{"error":{"message":"bad model"}}`)); err == nil {
		t.Error("expected an error for an error reply")
	}
	// An empty reply is an error.
	if _, _, err := ParseResponsesChat([]byte(`{"output":[],"usage":{}}`)); err == nil {
		t.Error("expected an error for an empty reply")
	}
}

// TestResponsesReasoningEchoRoundTrip verifies the reasoning item captured from a
// reply is echoed back verbatim (before the function_call) in the next request's
// input — the Responses API requires this for reasoning models with tools.
func TestResponsesReasoningEchoRoundTrip(t *testing.T) {
	reply := []byte(`{"output":[
		{"type":"reasoning","id":"rs_9","summary":[],"encrypted_content":"ENC"},
		{"type":"function_call","id":"fc_9","call_id":"call_9","name":"find_dupes","arguments":"{}"}
	],"usage":{"input_tokens":50,"output_tokens":10}}`)
	msg, _, err := ParseResponsesChat(reply)
	if err != nil {
		t.Fatalf("ParseResponsesChat: %v", err)
	}
	if len(msg.ReasoningRaw) != 1 {
		t.Fatalf("ReasoningRaw len = %d, want 1", len(msg.ReasoningRaw))
	}
	// Feed it back: history = [user, assistant(reasoning+tool call), tool result].
	hist := []Message{
		{Role: RoleUser, Content: "any dupes?"},
		msg,
		ToolResultMessage("call_9", "find_dupes", "found none"),
	}
	body, err := BuildResponsesToolRequest("gpt-5.6-sol", hist, 0, "medium", nil)
	if err != nil {
		t.Fatalf("BuildResponsesToolRequest: %v", err)
	}
	var got struct {
		Input []json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Expect: user, reasoning(echoed), function_call, function_call_output.
	if len(got.Input) != 4 {
		t.Fatalf("input len = %d, want 4: %s", len(got.Input), string(body))
	}
	types := make([]string, len(got.Input))
	for i, raw := range got.Input {
		var h struct {
			Type string `json:"type"`
			Role string `json:"role"`
		}
		_ = json.Unmarshal(raw, &h)
		types[i] = h.Type
		if h.Type == "" {
			types[i] = "role:" + h.Role
		}
	}
	want := []string{"role:user", "reasoning", "function_call", "function_call_output"}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("input[%d] type = %q, want %q (all: %v)", i, types[i], want[i], types)
		}
	}
	// The echoed reasoning item is verbatim (retains encrypted_content).
	if !strings.Contains(string(got.Input[1]), "ENC") {
		t.Errorf("reasoning item not echoed verbatim: %s", string(got.Input[1]))
	}
}
