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
	"strings"
)

// Role constants for chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
