// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

// A reasoning model asked for output with no room to produce any returns a reply
// that is billed and successful on the wire but carries no text. These tests pin
// the three behaviours that keep such a reply from reaching a user as a blank
// bubble: the request gives the model room, the parser refuses the empty result,
// and the refusal says why.

func TestBuildRequestWithOptionsGivesReasoningModelsRoomAndNoTemperature(t *testing.T) {
	body, err := BuildRequestWithOptions("gpt-5.4-mini", []Message{{Role: RoleUser, Content: "hi"}}, ChatOptions{
		Temperature:         0.7,
		MaxCompletionTokens: 4096,
		ReasoningEffort:     "low",
	})
	if err != nil {
		t.Fatalf("BuildRequestWithOptions: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["max_completion_tokens"] != float64(4096) {
		t.Fatalf("max_completion_tokens = %v, want 4096", got["max_completion_tokens"])
	}
	if got["reasoning_effort"] != "low" {
		t.Fatalf("reasoning_effort = %v, want low", got["reasoning_effort"])
	}
	if _, ok := got["temperature"]; ok {
		t.Fatalf("reasoning model was sent a temperature: %s", body)
	}
}

func TestBuildRequestWithOptionsKeepsTemperatureForOrdinaryModels(t *testing.T) {
	body, err := BuildRequestWithOptions("gpt-4.1", []Message{{Role: RoleUser, Content: "hi"}}, ChatOptions{
		Temperature:         0.7,
		MaxCompletionTokens: 1024,
		ReasoningEffort:     "high",
	})
	if err != nil {
		t.Fatalf("BuildRequestWithOptions: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["temperature"] != 0.7 {
		t.Fatalf("temperature = %v, want 0.7", got["temperature"])
	}
	if _, ok := got["reasoning_effort"]; ok {
		t.Fatalf("non-reasoning model was sent a reasoning effort: %s", body)
	}
	if got["max_completion_tokens"] != float64(1024) {
		t.Fatalf("max_completion_tokens = %v, want 1024", got["max_completion_tokens"])
	}
}

func TestReasoningModel(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  bool
	}{
		{"gpt-5.5", true},
		{"gpt-5.4-mini", true},
		{"o4-mini", true},
		{"o3", true},
		{"GPT-5.5", true},
		{"gpt-4.1", false},
		{"gpt-4o-mini", false},
		{"", false},
	} {
		if got := ReasoningModel(tc.model); got != tc.want {
			t.Fatalf("ReasoningModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestParseResponseRefusesBilledEmptyCompletion(t *testing.T) {
	// The exact failing shape: a 200 with usage recorded and no content.
	body := []byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"length"}],` +
		`"usage":{"prompt_tokens":9637,"completion_tokens":479,"total_tokens":10116}}`)
	got, err := ParseResponse(body)
	if err == nil {
		t.Fatalf("ParseResponse accepted an empty completion, returned %q", got)
	}
	if !strings.Contains(err.Error(), "thinking") {
		t.Fatalf("error does not explain the length cap: %v", err)
	}
}

func TestParseResponseAcceptsRealContent(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"role":"assistant","content":"you spent $40"},"finish_reason":"stop"}]}`)
	got, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if got != "you spent $40" {
		t.Fatalf("content = %q", got)
	}
}

func TestEmptyReplyMessageNamesTheCause(t *testing.T) {
	for _, tc := range []struct {
		finish string
		want   string
	}{
		{FinishLength, "thinking"},
		{FinishContentFilter, "safety filter"},
		{"", "without writing an answer"},
		{"weird", "without writing an answer"},
	} {
		got := EmptyReplyMessage(tc.finish)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("EmptyReplyMessage(%q) = %q, want it to mention %q", tc.finish, got, tc.want)
		}
		if !strings.HasSuffix(strings.TrimSpace(got), ".") {
			t.Fatalf("EmptyReplyMessage(%q) is not a finished sentence: %q", tc.finish, got)
		}
	}
}

func TestParseResponsesChatExplainsAnIncompleteReply(t *testing.T) {
	body := []byte(`{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},` +
		`"output":[{"type":"reasoning","summary":[]}],"usage":{"input_tokens":9637,"output_tokens":479}}`)
	_, usage, err := ParseResponsesChat(body)
	if err == nil {
		t.Fatal("ParseResponsesChat accepted a reply with no answer")
	}
	if !strings.Contains(err.Error(), "thinking") {
		t.Fatalf("error does not explain the budget: %v", err)
	}
	// The tokens were still spent, so they are still reported.
	if usage.CompletionTokens != 479 || usage.PromptTokens != 9637 {
		t.Fatalf("usage = %+v, want the billed tokens", usage)
	}
}

func TestBuildBudgetedResponsesToolRequestBoundsOutputAndDropsTemperature(t *testing.T) {
	body, err := BuildBudgetedResponsesToolRequest("gpt-5.4-mini",
		[]Message{{Role: RoleUser, Content: "hi"}}, 0.5, "medium",
		[]Tool{FunctionTool("list_transactions", "list them", json.RawMessage(`{"type":"object"}`))}, 2048)
	if err != nil {
		t.Fatalf("BuildBudgetedResponsesToolRequest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["max_output_tokens"] != float64(2048) {
		t.Fatalf("max_output_tokens = %v, want 2048", got["max_output_tokens"])
	}
	if _, ok := got["temperature"]; ok {
		t.Fatalf("reasoning model was sent a temperature: %s", body)
	}
	if got["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %v, want auto", got["tool_choice"])
	}
	tools, _ := got["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", got["tools"])
	}
	if first, _ := tools[0].(map[string]any); first["name"] != "list_transactions" {
		t.Fatalf("tool = %v", tools[0])
	}
}
