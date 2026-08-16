// SPDX-License-Identifier: MIT

package server

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
)

// These tests call the REAL OpenAI API and cost real money, so they are skipped
// unless CASHFLUX_LIVE_AI=1 is set explicitly. Nothing in CI or in a normal
// `go test ./...` run may spend the household's tokens.
//
// They exist because the failures this code was written to fix are ALL failures of
// agreement with a live provider: an endpoint that reports a billed empty
// completion as success, a model that rejects a temperature, a stream whose
// framing has to be read byte by byte. A mocked upstream proves the code does what
// its author expected; only the live provider proves the expectation was right.
//
// Run with:
//
//	CASHFLUX_LIVE_AI=1 go test ./internal/server/ -run TestLiveAI -v

const liveAIEnv = "CASHFLUX_LIVE_AI"

// liveAIKey reads the key from the environment, falling back to the OPENAIKEY line
// in the repo's .env, and skips the test when neither the opt-in nor a key is
// present.
func liveAIKey(t *testing.T) string {
	t.Helper()
	if os.Getenv(liveAIEnv) != "1" {
		t.Skipf("live provider test skipped; set %s=1 to run it (it spends real tokens)", liveAIEnv)
	}
	if k := strings.TrimSpace(os.Getenv("OPENAIKEY")); k != "" {
		return k
	}
	f, err := os.Open("../../.env")
	if err != nil {
		t.Skipf("no OPENAIKEY in the environment and no .env to read: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "OPENAIKEY") {
			return strings.TrimSpace(value)
		}
	}
	t.Skip("no OPENAIKEY found")
	return ""
}

// liveService wires an AIService against the real OpenAI API with the key stored
// the way a real household's would be — encrypted in the store — so the test
// exercises the whole proxy, not just its outbound half.
func liveService(t *testing.T) (*AIService, context.Context) {
	t.Helper()
	key := liveAIKey(t)
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u-live", Provider: "token", Subject: "u-live", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u-live", "openai", key, master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	svc := NewAIService(store, AIServiceConfig{MasterKey: master, UpstreamTimeout: 90 * time.Second})
	return svc, ContextWithAuthUser(context.Background(), AuthUser{ID: "u-live"})
}

// liveModel is the model these tests use. It is a reasoning model on purpose: the
// bug being guarded against only happens with one.
const liveModel = "gpt-5.4-mini"

func TestLiveAIReasoningModelActuallyAnswers(t *testing.T) {
	svc, ctx := liveService(t)
	got, err := svc.Chat(ctx, AIChatRequest{
		Model: liveModel,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "Answer in one short sentence."},
			{Role: ai.RoleUser, Content: "If I spend $312 in August and $280 in July, did spending go up or down?"},
		},
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	// The whole point: a reasoning model, billed, WITH text.
	if strings.TrimSpace(got.Content) == "" {
		t.Fatalf("a billed reasoning completion came back empty — the bug is not fixed: %+v", got)
	}
	if got.Usage.CompletionTokens == 0 {
		t.Fatalf("no completion tokens reported: %+v", got.Usage)
	}
	if !strings.Contains(strings.ToLower(got.Content), "up") {
		t.Errorf("answer does not read as an answer to the question: %q", got.Content)
	}
	t.Logf("answer: %q (%d in / %d out, finish %q)", got.Content, got.Usage.PromptTokens, got.Usage.CompletionTokens, got.FinishReason)
}

func TestLiveAIToolsReachTheModelAndComeBack(t *testing.T) {
	svc, ctx := liveService(t)
	got, err := svc.Chat(ctx, AIChatRequest{
		Model: liveModel,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "You have tools. Use them rather than guessing."},
			{Role: ai.RoleUser, Content: "How much did I spend on groceries in August 2026?"},
		},
		Tools: []ai.Tool{ai.FunctionTool("spending_by_category",
			"Total the user's spending per category for a month.",
			json.RawMessage(`{"type":"object","properties":{"month":{"type":"string","description":"YYYY-MM"}},"required":["month"]}`))},
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(got.ToolCalls) == 0 {
		t.Fatalf("the model was given a tool and did not call it — tools are not reaching it: %+v", got)
	}
	if got.ToolCalls[0].Function.Name != "spending_by_category" {
		t.Fatalf("tool call = %+v", got.ToolCalls[0])
	}
	if !json.Valid([]byte(got.ToolCalls[0].Function.Arguments)) {
		t.Fatalf("tool arguments are not JSON: %q", got.ToolCalls[0].Function.Arguments)
	}
	t.Logf("tool call: %s(%s)", got.ToolCalls[0].Function.Name, got.ToolCalls[0].Function.Arguments)
}

func TestLiveAIToolResultProducesAGroundedAnswer(t *testing.T) {
	svc, ctx := liveService(t)
	// The second half of a real tool conversation: the assistant asked, the app
	// answered, and the model must now use that answer rather than inventing one.
	got, err := svc.Chat(ctx, AIChatRequest{
		Model: liveModel,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "How much did I spend on groceries in August 2026?"},
			{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
				ID: "call_live_1", Type: "function",
				Function: ai.FunctionCall{Name: "spending_by_category", Arguments: `{"month":"2026-08"}`},
			}}},
			ai.ToolResultMessage("call_live_1", "spending_by_category", "Groceries: $312.40\nDining: $88.10"),
		},
		Tools: []ai.Tool{ai.FunctionTool("spending_by_category",
			"Total the user's spending per category for a month.",
			json.RawMessage(`{"type":"object","properties":{"month":{"type":"string"}},"required":["month"]}`))},
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(got.Content, "312") {
		t.Fatalf("the answer ignored the tool result it was given: %q", got.Content)
	}
	t.Logf("grounded answer: %q", got.Content)
}

func TestLiveAIStreamsFragmentsThenCompletes(t *testing.T) {
	svc, ctx := liveService(t)
	var fragments []string
	got, err := svc.ChatStream(ctx, AIChatRequest{
		Model: liveModel,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "Answer in two or three sentences."},
			{Role: ai.RoleUser, Content: "Explain in plain English why a budget can be over even when income went up."},
		},
		ReasoningEffort: "low",
	}, func(delta string) { fragments = append(fragments, delta) })
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if len(fragments) < 2 {
		t.Fatalf("the answer arrived in %d fragment(s) — it did not stream", len(fragments))
	}
	joined := strings.Join(fragments, "")
	if strings.TrimSpace(joined) == "" {
		t.Fatal("fragments carried no text")
	}
	// The streamed text and the authoritative completion must agree, or the
	// reader watched one answer appear and was handed a different one.
	if strings.TrimSpace(joined) != strings.TrimSpace(got.Content) {
		t.Fatalf("streamed text and final answer differ:\nstreamed: %q\nfinal:    %q", joined, got.Content)
	}
	if got.Usage.CompletionTokens == 0 {
		t.Fatalf("the completed event carried no usage: %+v", got.Usage)
	}
	t.Logf("streamed %d fragments, %d chars, %d out tokens", len(fragments), len(joined), got.Usage.CompletionTokens)
}

func TestLiveAIRejectedKeyIsExplainedInPlainEnglish(t *testing.T) {
	if os.Getenv(liveAIEnv) != "1" {
		t.Skipf("live provider test skipped; set %s=1 to run it", liveAIEnv)
	}
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u-bad", Provider: "token", Subject: "u-bad", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u-bad", "openai", "sk-definitely-not-a-real-key", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	svc := NewAIService(store, AIServiceConfig{MasterKey: master, UpstreamTimeout: 30 * time.Second})
	_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u-bad"}), AIChatRequest{
		Model:    liveModel,
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("a rejected key produced no error")
	}
	// A person reading this has to know what to DO, not what HTTP thinks.
	if !strings.Contains(err.Error(), "Settings") {
		t.Fatalf("the rejection does not tell the user where to fix it: %v", err)
	}
	t.Logf("rejection reads: %v", err)
}
