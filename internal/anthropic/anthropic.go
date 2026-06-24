// SPDX-License-Identifier: MIT

// Package anthropic shapes requests and responses for Anthropic's Messages API —
// the one wire dialect that isn't OpenAI-compatible (C81 phase 3). It only builds
// the request body and parses the response; it does no I/O, so it's pure and
// table-tests natively. The transport layer (internal/ai) does the HTTP/websocket.
//
// Anthropic differs from the OpenAI dialect in the ways modelled here: the system
// prompt is a top-level field (not a message), max_tokens is required, tools carry
// an input_schema (not parameters), and images are base64 content blocks.
package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"
)

const defaultMaxTokens = 1024

// Message is one conversation turn. Text is the content; when ImageBase64 is set the
// content becomes an image block (base64) followed by the text — the vision path.
type Message struct {
	Role        string // "user" or "assistant"
	Text        string
	ImageBase64 string
	ImageMIME   string // e.g. "image/png"
}

// Tool is a callable tool offered to the model (Anthropic shape: name + description
// + input_schema).
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// ToolCall is a tool_use block the model returned.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Usage is the token accounting from a response.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response is the parsed result: the concatenated text, any tool calls, the stop
// reason, and usage.
type Response struct {
	Text       string
	ToolCalls  []ToolCall
	StopReason string
	Usage      Usage
}

// BuildRequest serializes a Messages API request. maxTokens <= 0 uses a default;
// system/tools are omitted when empty; stream sets the streaming flag.
func BuildRequest(model, system string, messages []Message, tools []Tool, maxTokens int, stream bool) ([]byte, error) {
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   buildMessages(messages),
	}
	if strings.TrimSpace(system) != "" {
		body["system"] = system
	}
	if stream {
		body["stream"] = true
	}
	if len(tools) > 0 {
		ts := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			m := map[string]any{"name": t.Name}
			if t.Description != "" {
				m["description"] = t.Description
			}
			if len(t.InputSchema) > 0 {
				m["input_schema"] = t.InputSchema
			}
			ts = append(ts, m)
		}
		body["tools"] = ts
	}
	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}
	return out, nil
}

// buildMessages renders each message's content: a plain string, or — for a vision
// message — an [image, text] block array.
func buildMessages(messages []Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		if m.ImageBase64 != "" {
			blocks := []map[string]any{{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": m.ImageMIME,
					"data":       m.ImageBase64,
				},
			}}
			if m.Text != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Text})
			}
			out = append(out, map[string]any{"role": role, "content": blocks})
			continue
		}
		out = append(out, map[string]any{"role": role, "content": m.Text})
	}
	return out
}

// ParseResponse parses a Messages API response into text + tool calls + usage. An
// error envelope ({"type":"error",...}) is returned as a Go error.
func ParseResponse(data []byte) (Response, error) {
	var raw struct {
		Type    string `json:"type"`
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Response{}, fmt.Errorf("anthropic: parse response: %w", err)
	}
	if raw.Type == "error" || raw.Error != nil {
		msg := "unknown error"
		if raw.Error != nil {
			msg = raw.Error.Type + ": " + raw.Error.Message
		}
		return Response{}, fmt.Errorf("anthropic: %s", msg)
	}

	res := Response{StopReason: raw.StopReason, Usage: Usage{raw.Usage.InputTokens, raw.Usage.OutputTokens}}
	var text strings.Builder
	for _, c := range raw.Content {
		switch c.Type {
		case "text":
			text.WriteString(c.Text)
		case "tool_use":
			res.ToolCalls = append(res.ToolCalls, ToolCall{ID: c.ID, Name: c.Name, Input: c.Input})
		}
	}
	res.Text = text.String()
	return res, nil
}
