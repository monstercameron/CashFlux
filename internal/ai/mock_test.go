package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMockProviderChatReturnsCanedReply verifies that Chat fires onResult (not
// onError) with a non-empty string and plausible token usage.
func TestMockProviderChatReturnsCannedReply(t *testing.T) {
	m := MockProvider{}
	msgs := []Message{{Role: RoleUser, Content: "Explain my spending this month."}}
	var got string
	var usage Usage
	m.Chat("", "", "gpt-4o", msgs, 0.7, func(s string, u Usage) { got = s; usage = u }, func(e string) { t.Fatalf("unexpected error: %s", e) })
	if got == "" {
		t.Error("Chat: expected a non-empty reply")
	}
	if usage.TotalTokens == 0 {
		t.Error("Chat: expected non-zero token usage")
	}
}

// TestMockProviderChatOverride verifies that Overrides take precedence over the
// built-in canned responses.
func TestMockProviderChatOverride(t *testing.T) {
	m := MockProvider{Overrides: map[string]string{"hello": "custom reply"}}
	msgs := []Message{{Role: RoleUser, Content: "Hello there"}}
	var got string
	m.Chat("", "", "", msgs, 0, func(s string, _ Usage) { got = s }, func(e string) { t.Fatalf("unexpected error: %s", e) })
	if got != "custom reply" {
		t.Errorf("Chat override: got %q, want %q", got, "custom reply")
	}
}

// TestMockProviderChatNoUserTurn verifies graceful handling when the message
// list contains no user-role entry.
func TestMockProviderChatNoUserTurn(t *testing.T) {
	m := MockProvider{}
	msgs := []Message{{Role: RoleSystem, Content: "You are a helpful assistant."}}
	var got string
	m.Chat("", "", "", msgs, 0, func(s string, _ Usage) { got = s }, func(e string) { t.Fatalf("unexpected error: %s", e) })
	if got == "" {
		t.Error("Chat no-user-turn: expected fallback reply, got empty string")
	}
}

// TestMockProviderVisionChatReturnsReply verifies the VisionChat path returns
// a non-empty reply and does not call onError.
func TestMockProviderVisionChatReturnsReply(t *testing.T) {
	m := MockProvider{}
	var got string
	m.VisionChat("", "", "", "You are a receipt scanner.", "scan this receipt", "data:image/png;base64,ABC", 0,
		func(s string, _ Usage) { got = s }, func(e string) { t.Fatalf("unexpected error: %s", e) })
	if !strings.Contains(strings.ToLower(got), "receipt") && got == "" {
		t.Errorf("VisionChat: got empty reply")
	}
}

// TestMockProviderStructuredVisionChatJSON verifies that StructuredVisionChat
// returns a valid JSON string matching the named schema family, and that the
// JSON is well-formed.
func TestMockProviderStructuredVisionChatJSON(t *testing.T) {
	m := MockProvider{}
	schema := []byte(`{"type":"object","properties":{"description":{"type":"string"},"amount":{"type":"number"}}}`)
	var got string
	m.StructuredVisionChat("", "", "", "", "", "", 0, "transaction_import", schema,
		func(s string, _ Usage) { got = s }, func(e string) { t.Fatalf("unexpected error: %s", e) })
	var v map[string]any
	if err := json.Unmarshal([]byte(got), &v); err != nil {
		t.Errorf("StructuredVisionChat: reply is not valid JSON: %v (got %q)", err, got)
	}
	if _, ok := v["description"]; !ok {
		t.Errorf("StructuredVisionChat: missing 'description' field in %q", got)
	}
}

// TestMockProviderStructuredVisionChatBadSchema verifies that a malformed JSON
// schema triggers onError, not onResult.
func TestMockProviderStructuredVisionChatBadSchema(t *testing.T) {
	m := MockProvider{}
	var errMsg string
	m.StructuredVisionChat("", "", "", "", "", "", 0, "anything", []byte(`{bad json`),
		func(s string, _ Usage) { t.Fatalf("expected error but got result: %q", s) },
		func(e string) { errMsg = e })
	if !strings.Contains(errMsg, "invalid schema") {
		t.Errorf("expected 'invalid schema' in error, got %q", errMsg)
	}
}

// TestMockProviderStructuredVisionChatOverride verifies the Overrides map
// works for structured calls keyed on schemaName.
func TestMockProviderStructuredVisionChatOverride(t *testing.T) {
	customJSON := `{"title":"custom task","priority":"high","due":""}`
	m := MockProvider{Overrides: map[string]string{"insights_task": customJSON}}
	var got string
	m.StructuredVisionChat("", "", "", "", "", "", 0, "insights_task", nil,
		func(s string, _ Usage) { got = s }, func(e string) { t.Fatalf("unexpected error: %s", e) })
	if got != customJSON {
		t.Errorf("StructuredVisionChat override: got %q, want %q", got, customJSON)
	}
}

// TestMockProviderCancelIsNoop verifies that the returned cancel function can
// be called safely (it is a no-op, not nil).
func TestMockProviderCancelIsNoop(t *testing.T) {
	m := MockProvider{}
	msgs := []Message{{Role: RoleUser, Content: "test"}}
	cancel := m.Chat("", "", "", msgs, 0, func(string, Usage) {}, func(string) {})
	// Must not panic.
	cancel()
	cancel() // double-cancel also safe
}

// TestMockProviderSatisfiesProviderInterface is a compile-time check that
// MockProvider implements the Provider interface.
func TestMockProviderSatisfiesProviderInterface(t *testing.T) {
	var _ Provider = MockProvider{}
}

// TestMockUsageIsNonZero confirms mockUsage is filled (cost-display code
// relies on TotalTokens > 0 to show a figure).
func TestMockUsageIsNonZero(t *testing.T) {
	if mockUsage.TotalTokens == 0 {
		t.Error("mockUsage.TotalTokens must be non-zero")
	}
	if mockUsage.PromptTokens == 0 {
		t.Error("mockUsage.PromptTokens must be non-zero")
	}
	if mockUsage.CompletionTokens == 0 {
		t.Error("mockUsage.CompletionTokens must be non-zero")
	}
}
