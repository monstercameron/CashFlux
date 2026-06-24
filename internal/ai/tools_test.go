// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildToolRequest(t *testing.T) {
	params := json.RawMessage(`{"type":"object","properties":{"category":{"type":"string"}}}`)
	tools := []Tool{FunctionTool("spend_by_category", "Spend for a category", params)}
	body, err := BuildToolRequest("gpt-4o-mini", []Message{{Role: RoleUser, Content: "hi"}}, 0.4, tools)
	if err != nil {
		t.Fatalf("BuildToolRequest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["tool_choice"] != "auto" {
		t.Errorf("tool_choice = %v, want auto", got["tool_choice"])
	}
	if _, ok := got["tools"]; !ok {
		t.Error("tools missing from request")
	}
	if !strings.Contains(string(body), "spend_by_category") {
		t.Error("function name not serialized")
	}
}

func TestBuildToolRequestNoTools(t *testing.T) {
	body, err := BuildToolRequest("gpt-4o-mini", []Message{{Role: RoleUser, Content: "hi"}}, 0, nil)
	if err != nil {
		t.Fatalf("BuildToolRequest: %v", err)
	}
	if strings.Contains(string(body), "tool_choice") {
		t.Error("tool_choice should be omitted when no tools are offered")
	}
}

func TestParseChatToolCalls(t *testing.T) {
	resp := `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[
		{"id":"call_1","type":"function","function":{"name":"spending_by_category","arguments":"{\"category\":\"Groceries\"}"}}
	]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
	msg, finish, usage, err := ParseChat([]byte(resp))
	if err != nil {
		t.Fatalf("ParseChat: %v", err)
	}
	if finish != "tool_calls" || !WantsTools(msg) {
		t.Fatalf("finish=%q wantsTools=%v", finish, WantsTools(msg))
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "spending_by_category" {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}
	if msg.ToolCalls[0].Function.Arguments != `{"category":"Groceries"}` {
		t.Errorf("arguments = %q", msg.ToolCalls[0].Function.Arguments)
	}
	if usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", usage)
	}
}

func TestParseChatPlainContent(t *testing.T) {
	resp := `{"choices":[{"message":{"role":"assistant","content":"You spent $40."},"finish_reason":"stop"}],"usage":{"total_tokens":7}}`
	msg, finish, _, err := ParseChat([]byte(resp))
	if err != nil {
		t.Fatalf("ParseChat: %v", err)
	}
	if WantsTools(msg) || finish != "stop" || msg.Content != "You spent $40." {
		t.Errorf("msg = %+v finish=%q", msg, finish)
	}
}

func TestParseChatErrors(t *testing.T) {
	if _, _, _, err := ParseChat([]byte(`{"error":{"message":"bad key"}}`)); err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Errorf("API error not surfaced: %v", err)
	}
	if _, _, _, err := ParseChat([]byte(`{"choices":[]}`)); err == nil {
		t.Error("empty choices should error")
	}
	if _, _, _, err := ParseChat([]byte(`not json`)); err == nil {
		t.Error("garbage should error")
	}
}

func TestToolResultMessageRoundTrip(t *testing.T) {
	m := ToolResultMessage("call_1", "spending_by_category", `{"total":"$420"}`)
	if m.Role != RoleTool || m.ToolCallID != "call_1" || m.Name != "spending_by_category" {
		t.Fatalf("tool result message = %+v", m)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Message
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Role != m.Role || got.Content != m.Content || got.ToolCallID != m.ToolCallID || got.Name != m.Name {
		t.Errorf("round trip = %+v, want %+v", got, m)
	}
}
