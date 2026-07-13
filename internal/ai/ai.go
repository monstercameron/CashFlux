// SPDX-License-Identifier: MIT

// Package ai holds the OpenAI client used for CashFlux's Phase 2 intelligence
// features. The request/response shapes and their JSON codec live here as pure
// Go (unit-tested via round-trips, no network); the browser fetch transport that
// actually sends a request with the user's key lives in a thin js/wasm layer.
//
// Calls are made client-side with the user's own key (from Settings); no data
// leaves the device except to OpenAI on an explicit action.
package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Role constants for chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is one chat message. ToolCalls is set on an assistant turn that asks to
// run tools; ToolCallID and Name are set on a role:"tool" message returning a tool's
// result (see ToolResultMessage).
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
	// ReasoningRaw carries the Responses API "reasoning" output items verbatim, so an
	// assistant tool-call turn can echo them in the next request's input (the Responses
	// API requires the reasoning items alongside their function_call for reasoning
	// models). It never serializes on the chat-completions path (json:"-") and is unused
	// there; it only rides along in-memory during a Responses tool loop.
	ReasoningRaw []json.RawMessage `json:"-"`
}

// ChatRequest is an OpenAI chat-completions request body.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

// APIError is the error object OpenAI returns on failure.
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Usage is the token accounting OpenAI returns.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is an OpenAI chat-completions response body.
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage Usage     `json:"usage"`
	Error *APIError `json:"error,omitempty"`
}

// BuildRequest marshals a chat request body to JSON.
func BuildRequest(model string, messages []Message, temperature float64) ([]byte, error) {
	return json.Marshal(ChatRequest{Model: model, Messages: messages, Temperature: temperature})
}

// FinancialContext is the minimal, aggregate snapshot sent to the model for
// insights. By construction it holds only pre-formatted totals and an account
// count — never account names/numbers, payees, or any per-transaction detail —
// so an insights prompt can't leak identifying data. Callers format the money at
// the edge and hand over only these aggregates.
type FinancialContext struct {
	NetWorth string
	Income   string
	Spending string
	Accounts int
}

// Line renders the context as one plain-English sentence for a prompt.
func (c FinancialContext) Line() string {
	return fmt.Sprintf("Net worth %s, this month's income %s, spending %s, across %d active accounts.",
		c.NetWorth, c.Income, c.Spending, c.Accounts)
}

// JSONSchema names a JSON Schema for an OpenAI structured-output request. Strict
// constrains the model to match the schema exactly.
type JSONSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict,omitempty"`
}

// ResponseFormat is the OpenAI response_format that requests schema-constrained
// JSON output ("structured outputs").
type ResponseFormat struct {
	Type       string     `json:"type"` // "json_schema"
	JSONSchema JSONSchema `json:"json_schema"`
}

// structuredRequest is a chat request that constrains the reply to a JSON schema.
type structuredRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	Temperature    float64        `json:"temperature,omitempty"`
	ResponseFormat ResponseFormat `json:"response_format"`
}

// BuildStructuredRequest marshals a chat request that asks the model to return
// JSON conforming to the given JSON Schema (OpenAI "structured outputs"), so the
// reply can be decoded straight into a Go struct instead of being coaxed out of
// prose. schemaName is a short identifier OpenAI echoes back; schema is the raw
// JSON Schema. The reply's content is a JSON string — read it with ParseResponse,
// then json.Unmarshal into your target type.
func BuildStructuredRequest(model string, messages []Message, temperature float64, schemaName string, schema json.RawMessage) ([]byte, error) {
	return json.Marshal(structuredRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		ResponseFormat: ResponseFormat{
			Type:       "json_schema",
			JSONSchema: JSONSchema{Name: schemaName, Schema: schema, Strict: true},
		},
	})
}

// ParseResponse decodes a chat response and returns the assistant's first
// message content. It surfaces an API error (with its message) and reports an
// empty/garbled response as an error.
func ParseResponse(data []byte) (string, error) {
	var r ChatResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("ai: could not read the response: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("ai: %s", r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", errors.New("ai: the response was empty")
	}
	return r.Choices[0].Message.Content, nil
}

// ParseUsage decodes just the token usage from a response (zero Usage on error).
func ParseUsage(data []byte) Usage {
	var r ChatResponse
	_ = json.Unmarshal(data, &r)
	return r.Usage
}

// ModelPricing is a model's USD price per 1,000,000 input and output tokens.
type ModelPricing struct {
	Input  float64
	Output float64
}

// modelPricing maps known models to approximate per-1M-token USD pricing. It's a
// best-effort table for surfacing rough cost (prices change; treat as an
// estimate). Dated/variant model names fall back to the longest matching prefix.
var modelPricing = map[string]ModelPricing{
	"gpt-5.5":      {Input: 2.00, Output: 8.00},
	"gpt-5.4-mini": {Input: 0.25, Output: 2.00},
	"o4-mini":      {Input: 1.10, Output: 4.40},
}

// pricingFor returns the pricing for a model, matching exactly first, then by the
// longest known prefix (so "gpt-5.4-mini-2026-xx-xx" resolves to gpt-5.4-mini, not
// gpt-5.5). ok is false when no entry matches.
func pricingFor(model string) (ModelPricing, bool) {
	if p, ok := modelPricing[model]; ok {
		return p, true
	}
	best := ""
	for k := range modelPricing {
		if strings.HasPrefix(model, k) && len(k) > len(best) {
			best = k
		}
	}
	if best != "" {
		return modelPricing[best], true
	}
	return ModelPricing{}, false
}

// EstimateCostUSD returns the approximate USD cost of a completion from its token
// usage and model, with ok=false when the model's pricing isn't known.
func EstimateCostUSD(model string, u Usage) (float64, bool) {
	p, ok := pricingFor(model)
	if !ok {
		return 0, false
	}
	return float64(u.PromptTokens)/1e6*p.Input + float64(u.CompletionTokens)/1e6*p.Output, true
}

// FormatCostUSD renders an estimated cost compactly: zero as "$0.00", sub-cent
// amounts with four decimals (so a fraction of a cent is still visible), and
// larger amounts with the usual two.
func FormatCostUSD(cost float64) string {
	switch {
	case cost <= 0:
		return "$0.00"
	case cost < 0.01:
		return fmt.Sprintf("$%.4f", cost)
	default:
		return fmt.Sprintf("$%.2f", cost)
	}
}

// MaxRetries is how many times a transient OpenAI failure is retried before
// giving up (so up to MaxRetries+1 total attempts).
const MaxRetries = 3

// IsRetryable reports whether a failure is worth retrying: rate limiting (429),
// server errors (5xx), or a network-level failure (status 0). Client errors like
// 400/401/404 are not retried — they won't succeed on a repeat.
func IsRetryable(status int) bool {
	return status == 0 || status == 429 || status >= 500
}

// RetryDelayMS returns the backoff delay in milliseconds before retry attempt n
// (0-indexed) and whether another attempt should be made at all. Delays grow
// exponentially from 500ms (500, 1000, 2000) up to MaxRetries.
func RetryDelayMS(attempt int) (ms int, retry bool) {
	if attempt < 0 || attempt >= MaxRetries {
		return 0, false
	}
	return 500 << attempt, true
}

// ErrorMessage turns a failed OpenAI HTTP response (its status code and body)
// into a concise, plain-English, actionable message for the user. It recognizes
// the common failure modes — rejected key, rate limit vs. spent quota, missing
// model, server trouble — and otherwise falls back to OpenAI's own error message
// or a generic line that names the status code.
func ErrorMessage(status int, body []byte) string {
	detail := apiErrorMessage(body)
	low := strings.ToLower(detail)
	switch {
	case status == 401:
		return "OpenAI didn't accept your API key. Check it in Settings."
	case status == 403:
		return "OpenAI refused this request — your key may not have access to this model. Check your plan or pick another model in Settings."
	case status == 429:
		if strings.Contains(low, "quota") || strings.Contains(low, "billing") || strings.Contains(low, "insufficient") {
			return "Your OpenAI account is out of quota. Check your billing, then try again."
		}
		return "OpenAI is rate-limiting requests. Wait a few seconds and try again."
	case status == 404:
		return "OpenAI couldn't find that model. Pick a different model in Settings."
	case status >= 500:
		return "OpenAI is having server trouble. Try again in a moment."
	case status == 400 && detail != "":
		return "OpenAI rejected the request: " + detail
	case detail != "":
		return detail
	default:
		return fmt.Sprintf("OpenAI returned an unexpected error (HTTP %d).", status)
	}
}

// apiErrorMessage extracts OpenAI's error.message from a response body, or "" when
// the body isn't a recognizable error object.
func apiErrorMessage(body []byte) string {
	var r ChatResponse
	if err := json.Unmarshal(body, &r); err == nil && r.Error != nil {
		return strings.TrimSpace(r.Error.Message)
	}
	return ""
}

// modelListResponse is the shape of OpenAI's GET /v1/models body: {"data":[{"id":...}]}.
type modelListResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// nonChatModelHints are id fragments that mark a model as NOT a chat/completions
// model (embeddings, audio, image, moderation, etc.), so ChatModelIDs can drop them.
var nonChatModelHints = []string{
	"embedding", "whisper", "tts", "audio", "realtime", "transcribe",
	"image", "dall-e", "moderation", "-search", "instruct", "codex",
	"davinci", "babbage", "computer-use",
}

// isChatModelID reports whether an OpenAI model id names a chat/completions-capable
// model: a gpt-* / chatgpt-* / o-series id that isn't one of the non-chat families.
func isChatModelID(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return false
	}
	chatFamily := strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "chatgpt") ||
		strings.HasPrefix(id, "o1") || strings.HasPrefix(id, "o3") ||
		strings.HasPrefix(id, "o4") || strings.HasPrefix(id, "o5")
	if !chatFamily {
		return false
	}
	for _, h := range nonChatModelHints {
		if strings.Contains(id, h) {
			return false
		}
	}
	return true
}

// ParseModelIDs extracts every model id from an OpenAI /v1/models response body,
// de-duplicated. Returns an error if the body isn't the expected list shape.
func ParseModelIDs(data []byte) ([]string, error) {
	var r modelListResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse model list: %w", err)
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(r.Data))
	for _, m := range r.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

// modelsRejectingEffort remembers non-gpt-5 models learned at runtime to reject
// reasoning_effort alongside function tools on /chat/completions. The whole gpt-5.x
// family is handled by prefix in EffortRejectedWithTools (no learning needed). Reset
// each page load (a fresh wasm instance).
var modelsRejectingEffort = map[string]bool{}

// EffortRejectedWithTools reports whether a model rejects reasoning_effort when
// function tools are advertised on /chat/completions. The entire gpt-5.x family does
// — it requires /v1/responses for that combination (confirmed for gpt-5.4-mini and
// gpt-5.6-luna) — so we skip it by prefix; other models are learned at runtime via a
// one-time failed request. Callers use this to skip the effort AND to hide the
// thinking-level control where it has no effect. o-series (o1/o3/o4) DO accept it.
func EffortRejectedWithTools(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(m, "gpt-5") {
		return true
	}
	return modelsRejectingEffort[m]
}

// noteEffortRejected records that a model rejects reasoning_effort with tools.
func noteEffortRejected(model string) { modelsRejectingEffort[strings.TrimSpace(model)] = true }

// mentionsEffortUnsupported reports whether an API error is the "reasoning_effort not
// supported with function tools" rejection (so we can retry without the effort).
func mentionsEffortUnsupported(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "reasoning_effort")
}

// ChatModelIDs parses an OpenAI /v1/models response and returns the ids usable for
// chat (the gpt-* and o-series families), excluding non-chat endpoints (embeddings,
// audio, image, moderation, …). Sorted descending so newer families surface first.
func ChatModelIDs(data []byte) ([]string, error) {
	all, err := ParseModelIDs(data)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(all))
	for _, id := range all {
		if isChatModelID(id) {
			out = append(out, id)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}
