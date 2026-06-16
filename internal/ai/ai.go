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
