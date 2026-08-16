// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newProxyTestService wires an AIService against a stub upstream with a stored key,
// returning the service and a pointer to the last request body the upstream saw.
func newProxyTestService(t *testing.T, handler http.HandlerFunc) (*AIService, context.Context) {
	t.Helper()
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	day := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u-proxy", Provider: "token", Subject: "u-proxy", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u-proxy", "openai", "sk-proxy-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	svc := NewAIService(store, AIServiceConfig{
		MasterKey: master,
		BaseURL:   upstream.URL,
		Now:       func() time.Time { return day },
	})
	return svc, ContextWithAuthUser(context.Background(), AuthUser{ID: "u-proxy"})
}

// The report that opened this: 479 completion tokens billed, no answer on screen.
// The proxy must now refuse that reply and say what happened, and it must still
// record the tokens the user was charged for.
func TestAIProxyRefusesBilledEmptyCompletionAndStillRecordsUsage(t *testing.T) {
	svc, ctx := newProxyTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(responsesIncompleteReply("max_output_tokens", 9637, 479)))
	})
	_, err := svc.Chat(ctx, AIChatRequest{
		Model:    "gpt-5.4-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "how much did we spend?"}},
	})
	if err == nil {
		t.Fatal("proxy returned an empty completion as success")
	}
	if !strings.Contains(err.Error(), "thinking") {
		t.Fatalf("error does not explain the cause: %v", err)
	}
	usage, ok, err := svc.store.GetUsage("u-proxy", time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC))
	if err != nil || !ok {
		t.Fatalf("GetUsage: %v/%v", ok, err)
	}
	if usage.Tokens != 10116 {
		t.Fatalf("usage tokens = %d, want the 10,116 that were billed", usage.Tokens)
	}
}

// A household on the shared server key gets the same tool-using assistant as one on
// a direct key: the tools reach OpenAI, and the tool calls come back.
func TestAIProxyCarriesToolsAndReturnsToolCalls(t *testing.T) {
	var sent map[string]any
	svc, ctx := newProxyTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("tool turn went to %s, want /responses", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		sent, _ = decodeResponsesRequest(string(raw))
		_, _ = w.Write([]byte(responsesToolCallReply("call_1", "list_transactions", `{"month":"2026-08"}`, 100, 20)))
	})
	got, err := svc.Chat(ctx, AIChatRequest{
		Model:    "gpt-5.4-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "what did we spend on groceries?"}},
		Tools: []ai.Tool{
			ai.FunctionTool("list_transactions", "list transactions in a month", json.RawMessage(`{"type":"object"}`)),
		},
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	tools, _ := sent["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools reaching OpenAI = %v", sent["tools"])
	}
	if sent["reasoning"] == nil {
		t.Fatalf("reasoning effort was dropped: %v", sent)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Function.Name != "list_transactions" {
		t.Fatalf("tool calls = %+v", got.ToolCalls)
	}
	if got.ToolCalls[0].ID != "call_1" || got.ToolCalls[0].Function.Arguments != `{"month":"2026-08"}` {
		t.Fatalf("tool call detail = %+v", got.ToolCalls[0])
	}
	if got.FinishReason != ai.FinishToolCalls {
		t.Fatalf("finish reason = %q, want %q", got.FinishReason, ai.FinishToolCalls)
	}
}

// A tool conversation survives the round trip: the assistant's tool-call turn and
// the tool result that answers it both reach OpenAI intact.
func TestAIProxyAcceptsAToolResultTurn(t *testing.T) {
	var sent map[string]any
	svc, ctx := newProxyTestService(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		sent, _ = decodeResponsesRequest(string(raw))
		_, _ = w.Write([]byte(responsesTextReply("You spent $312 on groceries.", 200, 30)))
	})
	got, err := svc.Chat(ctx, AIChatRequest{
		Model: "gpt-5.4-mini",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "what did we spend on groceries?"},
			{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
				ID: "call_1", Type: "function",
				Function: ai.FunctionCall{Name: "list_transactions", Arguments: `{"month":"2026-08"}`},
			}}},
			ai.ToolResultMessage("call_1", "list_transactions", `{"total":31200}`),
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got.Content != "You spent $312 on groceries." {
		t.Fatalf("content = %q", got.Content)
	}
	input, _ := sent["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("input items = %d, want 3: %v", len(input), sent["input"])
	}
	call, _ := input[1].(map[string]any)
	if call["type"] != "function_call" || call["call_id"] != "call_1" {
		t.Fatalf("assistant tool call did not survive: %v", input[1])
	}
	result, _ := input[2].(map[string]any)
	if result["type"] != "function_call_output" || result["output"] != `{"total":31200}` {
		t.Fatalf("tool result did not survive: %v", input[2])
	}
}

// An ordinary (non-reasoning) model keeps the simpler endpoint, and still gets an
// output ceiling so one question cannot run away with a household's daily budget.
func TestAIProxyKeepsChatCompletionsForOrdinaryModels(t *testing.T) {
	var path string
	var sent map[string]any
	svc, ctx := newProxyTestService(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &sent)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"total_tokens":4}}`))
	})
	svc.allowedModels = nil
	got, err := svc.Chat(ctx, AIChatRequest{
		Model:       "gpt-4.1",
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		Temperature: 0.4,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if path != "/chat/completions" {
		t.Fatalf("ordinary model went to %s", path)
	}
	if sent["max_completion_tokens"] != float64(defaultAIMaxOutputTokens) {
		t.Fatalf("max_completion_tokens = %v", sent["max_completion_tokens"])
	}
	if sent["temperature"] != 0.4 {
		t.Fatalf("temperature = %v, want it kept for an ordinary model", sent["temperature"])
	}
	if got.FinishReason != ai.FinishStop || got.Content != "hello" {
		t.Fatalf("completion = %+v", got)
	}
}

// The same refusal applies on the chat-completions path, so neither endpoint can
// hand a blank answer to the screen.
func TestAIProxyRefusesEmptyChatCompletionsReply(t *testing.T) {
	svc, ctx := newProxyTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"content_filter"}],"usage":{"total_tokens":12}}`))
	})
	svc.allowedModels = nil
	_, err := svc.Chat(ctx, AIChatRequest{
		Model:    "gpt-4.1",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("empty chat-completions reply was returned as success")
	}
	if !strings.Contains(err.Error(), "safety filter") {
		t.Fatalf("error does not name the filter: %v", err)
	}
}

// Tool definitions are user-supplied input over the wire, so they are bounded like
// every other field rather than trusted.
func TestAIProxyValidatesToolDefinitions(t *testing.T) {
	called := false
	svc, ctx := newProxyTestService(t, func(http.ResponseWriter, *http.Request) { called = true })
	for _, tc := range []struct {
		name string
		req  AIChatRequest
	}{
		{"unnamed tool", AIChatRequest{
			Model:    "gpt-5.4-mini",
			Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
			Tools:    []ai.Tool{{Type: "function"}},
		}},
		{"schema is not JSON", AIChatRequest{
			Model:    "gpt-5.4-mini",
			Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
			Tools:    []ai.Tool{ai.FunctionTool("t", "d", json.RawMessage(`{nope`))},
		}},
		{"tool result without a call id", AIChatRequest{
			Model: "gpt-5.4-mini",
			Messages: []ai.Message{
				{Role: ai.RoleUser, Content: "hi"},
				{Role: ai.RoleTool, Content: "result"},
			},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Chat(ctx, tc.req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("err = %v, want invalid argument", err)
			}
		})
	}
	if called {
		t.Fatal("an invalid request reached OpenAI")
	}
}

// An assistant turn that only asks for tools carries no text. Rejecting it as
// "content is required" would have made tool conversations impossible.
func TestAIProxyAcceptsContentlessAssistantToolCallTurn(t *testing.T) {
	svc, ctx := newProxyTestService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(responsesTextReply("done", 10, 2)))
	})
	_, err := svc.Chat(ctx, AIChatRequest{
		Model: "gpt-5.4-mini",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "hi"},
			{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
				ID: "c1", Type: "function", Function: ai.FunctionCall{Name: "t", Arguments: "{}"},
			}}},
			ai.ToolResultMessage("c1", "t", ""),
		},
	})
	if err != nil {
		t.Fatalf("Chat rejected a valid tool conversation: %v", err)
	}
}

// The failure the proxy path would otherwise hit on step two of every tool
// conversation. A reasoning model returns reasoning items alongside its tool
// call, and the Responses API rejects the NEXT turn unless those items are echoed
// back with the call. They therefore have to survive the round trip through the
// proxy — out to the caller, and back in with the follow-up.
func TestReasoningItemsSurviveTheRoundTripSoStepTwoIsAccepted(t *testing.T) {
	var sent map[string]any
	svc, ctx := newProxyTestService(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		sent, _ = decodeResponsesRequest(string(raw))
		_, _ = w.Write([]byte(`{"status":"completed","output":[` +
			`{"type":"reasoning","id":"rs_1","summary":[]},` +
			`{"type":"function_call","call_id":"call_1","name":"list_transactions","arguments":"{}"}` +
			`],"usage":{"input_tokens":10,"output_tokens":5}}`))
	})
	tools := []ai.Tool{ai.FunctionTool("list_transactions", "list them", json.RawMessage(`{"type":"object"}`))}

	// Step one: the model asks for a tool, and hands back reasoning items with it.
	first, err := svc.Chat(ctx, AIChatRequest{
		Model:    "gpt-5.4-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "what did we spend?"}},
		Tools:    tools,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(first.Reasoning) != 1 {
		t.Fatalf("reasoning items returned = %d, want 1 — without them step two is rejected", len(first.Reasoning))
	}

	// Step two: the caller returns the assistant turn WITH those items, plus the
	// tool result. They must reach OpenAI ahead of the function_call.
	if _, err := svc.Chat(ctx, AIChatRequest{
		Model: "gpt-5.4-mini",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "what did we spend?"},
			{Role: ai.RoleAssistant, ToolCalls: first.ToolCalls, ReasoningRaw: first.Reasoning},
			ai.ToolResultMessage("call_1", "list_transactions", "total $312"),
		},
		Tools: tools,
	}); err != nil {
		t.Fatalf("second Chat: %v", err)
	}
	input, _ := sent["input"].([]any)
	if len(input) != 4 {
		t.Fatalf("input items = %d, want user + reasoning + function_call + result: %v", len(input), sent["input"])
	}
	if item, _ := input[1].(map[string]any); item["type"] != "reasoning" {
		t.Fatalf("input[1] = %v, want the reasoning item ahead of the call", input[1])
	}
	if item, _ := input[2].(map[string]any); item["type"] != "function_call" {
		t.Fatalf("input[2] = %v, want the function call", input[2])
	}
}
