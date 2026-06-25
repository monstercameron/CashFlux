// SPDX-License-Identifier: MIT

// Responses-API codec for the OpenAI Responses endpoint (POST /responses).
// This file has no build tags so it compiles on native Go for unit tests.
package ai

import (
	"encoding/json"
	"errors"
	"fmt"
)

// responsesWebSearchTool is the hosted web-search tool descriptor sent to the
// Responses API. Using the type literal here avoids an extra named type that
// would only be used in one place.
type responsesWebSearchTool struct {
	Type string `json:"type"`
}

// responsesRequest is the JSON body for POST /responses with a web_search tool.
type responsesRequest struct {
	Model string                   `json:"model"`
	Tools []responsesWebSearchTool `json:"tools"`
	Input string                   `json:"input"`
}

// BuildResponsesWebSearchRequest marshals a Responses API request that asks the
// model to use the hosted web_search tool to answer input. The caller owns the
// resulting byte slice.
func BuildResponsesWebSearchRequest(model, input string) ([]byte, error) {
	req := responsesRequest{
		Model: model,
		Tools: []responsesWebSearchTool{{Type: "web_search"}},
		Input: input,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ai: could not build Responses request: %w", err)
	}
	return data, nil
}

// responsesContentItem is one content block inside a Responses API message
// item. Only output_text is handled; other types (annotations, etc.) are
// silently ignored.
type responsesContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// responsesOutputItem is one item in the Responses API output array. Only
// items of type "message" are consulted for text; "web_search_call" items
// and any other types are skipped.
type responsesOutputItem struct {
	Type    string                 `json:"type"`
	Content []responsesContentItem `json:"content,omitempty"`
}

// responsesUsage is the token-accounting block in a Responses API reply. The
// field names follow the Responses API schema (input_tokens / output_tokens
// instead of prompt_tokens / completion_tokens used by Chat Completions).
type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// responsesReply is the top-level Responses API response body.
type responsesReply struct {
	Output []responsesOutputItem `json:"output"`
	Usage  responsesUsage        `json:"usage"`
	Error  *APIError             `json:"error,omitempty"`
}

// ParseResponsesText walks a raw Responses API reply and collects all
// output_text blocks from message items, concatenating them into one string.
// It also surfaces any API error. Usage is always populated (zeroed on parse
// failure). The returned Usage maps Responses field names onto the Chat
// Completions names (PromptTokens / CompletionTokens) so callers can pass it
// directly to EstimateCostUSD.
func ParseResponsesText(data []byte) (text string, usage Usage, err error) {
	var r responsesReply
	if err = json.Unmarshal(data, &r); err != nil {
		return "", Usage{}, fmt.Errorf("ai: could not read Responses reply: %w", err)
	}
	if r.Error != nil {
		return "", Usage{}, fmt.Errorf("ai: %s", r.Error.Message)
	}
	usage = Usage{
		PromptTokens:     r.Usage.InputTokens,
		CompletionTokens: r.Usage.OutputTokens,
		TotalTokens:      r.Usage.InputTokens + r.Usage.OutputTokens,
	}
	for _, item := range r.Output {
		if item.Type != "message" {
			continue
		}
		for _, c := range item.Content {
			if c.Type == "output_text" {
				text += c.Text
			}
		}
	}
	if text == "" {
		return "", usage, errors.New("ai: the Responses reply contained no text output")
	}
	return text, usage, nil
}
