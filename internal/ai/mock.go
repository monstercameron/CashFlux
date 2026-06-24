// SPDX-License-Identifier: MIT

// Package ai — mock provider (L8).
//
// MockProvider is a deterministic, zero-network AI backend that returns canned
// well-formed responses. It implements the Provider interface (same call
// signatures as the production transport) so callers can swap backends via a
// single construction-time choice.
//
// Activation: instantiate a MockProvider and pass it wherever a Provider is
// accepted. For in-browser dev, gate on the js global cashfluxAIMock=1 (set by
// a query-param bootstrap snippet). For tests, construct MockProvider{} directly.
package ai

import (
	"encoding/json"
	"strings"
)

// cancel is a no-op abort function returned from mock calls (there is nothing
// to abort). It satisfies the func() cancel contract of every Provider method.
func mockNoop() {}

// Provider is the interface both the real OpenAI transport and the mock satisfy.
// Call sites that exercise the AI flow accept a Provider so tests can inject a
// MockProvider without any network setup.
type Provider interface {
	// Chat sends a plain-text chat request and returns the assistant's response
	// plus token usage, or an error string. The returned func() aborts an
	// in-flight call; after abort, neither callback fires.
	Chat(apiKey, baseURL, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) func()

	// VisionChat sends a multimodal request (text + one image URL) and returns
	// the assistant's plain-text response plus usage.
	VisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, onResult func(string, Usage), onError func(string)) func()

	// StructuredVisionChat sends a multimodal request whose reply is
	// schema-constrained JSON. The content string in onResult is a JSON object
	// matching the given schema — decode with json.Unmarshal.
	StructuredVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func()
}

// MockProvider is a deterministic, offline AI backend for development and
// testing. Responses are keyed on the last user-turn content or the schema name
// so different prompt types return predictable, well-formed text. The mock never
// calls the network and fires onResult synchronously, so tests need no goroutine
// plumbing or timeouts.
type MockProvider struct {
	// Overrides lets tests inject specific responses keyed on a substring of the
	// user message or the schemaName. An empty map falls through to the built-in
	// canned responses.
	Overrides map[string]string
}

// mockUsage is a fixed token-cost estimate returned by all mock responses. The
// numbers are plausible for a short prompt so cost-display code can be exercised.
var mockUsage = Usage{PromptTokens: 120, CompletionTokens: 80, TotalTokens: 200}

// canned returns the canned mock response for a given key (user message text or
// schema name). It checks Overrides first, then falls back to built-in replies.
func (m MockProvider) canned(key string) string {
	low := strings.ToLower(key)
	for k, v := range m.Overrides {
		if strings.Contains(low, strings.ToLower(k)) {
			return v
		}
	}
	// Built-in canned responses for the known Insights and vision flows.
	switch {
	case strings.Contains(low, "insight"), strings.Contains(low, "explain"), strings.Contains(low, "summary"):
		return "Your spending this month is on track. Food & dining accounts for the largest share at roughly 30%, which is typical. Your savings rate looks healthy — keep it up."
	case strings.Contains(low, "task"), strings.Contains(low, "todo"), strings.Contains(low, "action"):
		return "Review your subscription services this week to spot any unused ones."
	case strings.Contains(low, "vision"), strings.Contains(low, "receipt"), strings.Contains(low, "image"):
		return "Receipt scanned. Detected a transaction for $42.50 at Whole Foods on 2026-06-20."
	case strings.Contains(low, "allocate"), strings.Contains(low, "budget"):
		return "Recommended allocation: Housing 30%, Food 15%, Transport 10%, Savings 20%, Other 25%."
	default:
		return "I'm a mock AI response. Everything looks good!"
	}
}

// cannedJSON returns a minimal JSON object that satisfies the named schema.
// Used by StructuredVisionChat when the caller expects JSON-shaped output.
func cannedJSON(schemaName string) string {
	switch strings.ToLower(schemaName) {
	case "transaction_import", "import", "receipt":
		return `{"description":"Whole Foods","amount":4250,"currency":"USD","date":"2026-06-20","category":"Groceries"}`
	case "insights_task":
		return `{"title":"Review unused subscriptions","priority":"medium","due":""}`
	default:
		// Return a generic single-field JSON so any json.Unmarshal call succeeds.
		return `{"result":"mock"}`
	}
}

// userTurn returns the last user-role message content for key selection; falls
// back to the first message content when no user turn exists.
func userTurn(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			return messages[i].Content
		}
	}
	if len(messages) > 0 {
		return messages[0].Content
	}
	return ""
}

// Chat satisfies Provider. The mock fires onResult synchronously with a canned
// reply and returns a no-op cancel.
func (m MockProvider) Chat(_, _, _ string, messages []Message, _ float64, onResult func(string, Usage), _ func(string)) func() {
	onResult(m.canned(userTurn(messages)), mockUsage)
	return mockNoop
}

// VisionChat satisfies Provider. The mock ignores the image URL and returns a
// canned receipt/vision reply based on the user-text hint.
func (m MockProvider) VisionChat(_, _, _, _, userText, _ string, _ float64, onResult func(string, Usage), _ func(string)) func() {
	onResult(m.canned("vision "+userText), mockUsage)
	return mockNoop
}

// StructuredVisionChat satisfies Provider. It validates that the provided schema
// is well-formed JSON (so callers catch schema bugs early), then returns a canned
// JSON string keyed on schemaName. The reply is compatible with json.Unmarshal.
func (m MockProvider) StructuredVisionChat(_, _, _, _, _, _ string, _ float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func() {
	if len(schema) > 0 {
		var check json.RawMessage
		if err := json.Unmarshal(schema, &check); err != nil {
			onError("mock: invalid schema JSON: " + err.Error())
			return mockNoop
		}
	}
	reply := cannedJSON(schemaName)
	if override, ok := m.Overrides[schemaName]; ok {
		reply = override
	}
	onResult(reply, mockUsage)
	return mockNoop
}
