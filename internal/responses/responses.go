// Package responses shapes requests and parses replies for OpenAI's Responses API —
// the app's preferred endpoint for the reasoning models (gpt-5.5) and streaming
// (C81). It only builds the request body and reads the response; it does no I/O, so
// it is pure and table-tests natively. The transport (internal/ai) opens the
// websocket/SSE connection and honors the streaming flag.
//
// The Responses API differs from /chat/completions: the system prompt is
// `instructions`, messages are `input`, tools are flat `{type:"function", name,
// parameters}`, reasoning effort is `reasoning.effort`, the cap is
// `max_output_tokens`, and the reply is an `output` array of message and
// function_call items.
package responses

import (
	"encoding/json"
	"fmt"
	"strings"
)

const defaultMaxOutput = 1024

// Message is one input turn (role + text content).
type Message struct {
	Role string // "user" | "assistant" | "system"
	Text string
}

// Tool is a function tool offered to the model.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON schema
}

// ToolCall is a function_call item the model returned. Arguments is the raw JSON
// arguments string.
type ToolCall struct {
	CallID    string
	Name      string
	Arguments json.RawMessage
}

// Usage is the token accounting.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response is the parsed reply.
type Response struct {
	Text      string
	ToolCalls []ToolCall
	Status    string
	Usage     Usage
}

// BuildRequest serializes a Responses API request. instructions/tools are omitted
// when empty; effort (low/medium/high) adds a reasoning block when non-empty; stream
// sets streaming; maxOutput <= 0 uses a default.
func BuildRequest(model, instructions string, input []Message, tools []Tool, effort string, stream bool, maxOutput int) ([]byte, error) {
	if maxOutput <= 0 {
		maxOutput = defaultMaxOutput
	}
	in := make([]map[string]any, 0, len(input))
	for _, m := range input {
		role := m.Role
		if role == "" {
			role = "user"
		}
		in = append(in, map[string]any{"role": role, "content": m.Text})
	}
	body := map[string]any{
		"model":             model,
		"input":             in,
		"max_output_tokens": maxOutput,
	}
	if strings.TrimSpace(instructions) != "" {
		body["instructions"] = instructions
	}
	if strings.TrimSpace(effort) != "" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	if stream {
		body["stream"] = true
	}
	if len(tools) > 0 {
		ts := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			m := map[string]any{"type": "function", "name": t.Name}
			if t.Description != "" {
				m["description"] = t.Description
			}
			if len(t.Parameters) > 0 {
				m["parameters"] = t.Parameters
			}
			ts = append(ts, m)
		}
		body["tools"] = ts
	}
	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("responses: marshal request: %w", err)
	}
	return out, nil
}

// ParseResponse reads a Responses API reply: it concatenates output_text across
// message items, collects function_call items as tool calls, and reads usage. A
// top-level error object is returned as a Go error.
func ParseResponse(data []byte) (Response, error) {
	var raw struct {
		Status string `json:"status"`
		Output []struct {
			Type      string          `json:"type"`
			Name      string          `json:"name"`
			CallID    string          `json:"call_id"`
			Arguments json.RawMessage `json:"arguments"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Response{}, fmt.Errorf("responses: parse: %w", err)
	}
	if raw.Error != nil {
		return Response{}, fmt.Errorf("responses: %s: %s", raw.Error.Type, raw.Error.Message)
	}

	res := Response{Status: raw.Status, Usage: Usage{raw.Usage.InputTokens, raw.Usage.OutputTokens}}
	var text strings.Builder
	for _, item := range raw.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					text.WriteString(c.Text)
				}
			}
		case "function_call":
			res.ToolCalls = append(res.ToolCalls, ToolCall{CallID: item.CallID, Name: item.Name, Arguments: item.Arguments})
		}
	}
	res.Text = text.String()
	return res, nil
}
