// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"errors"
	"fmt"
)

// FunctionDef describes a callable tool the model may invoke: its name, a
// plain-English description the model uses to decide when to call it, and a JSON
// Schema for its arguments (OpenAI "function calling").
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// Tool wraps a function definition in the OpenAI tools envelope.
type Tool struct {
	Type     string      `json:"type"` // always "function"
	Function FunctionDef `json:"function"`
}

// FunctionTool builds a function-type Tool from a name, description, and raw
// JSON-Schema parameters.
func FunctionTool(name, description string, parameters json.RawMessage) Tool {
	return Tool{Type: "function", Function: FunctionDef{Name: name, Description: description, Parameters: parameters}}
}

// FunctionCall is the function name + raw JSON arguments the model wants invoked.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // a JSON string, decode into the tool's params
}

// ToolCall is one tool invocation requested by the model in an assistant turn.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// toolRequest is a chat request that advertises tools to the model.
type toolRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"` // "auto" | "none" | "required"
}

// BuildToolRequest marshals a chat request that offers the given tools to the
// model with tool_choice=auto, so the model may answer directly or ask to call
// one or more tools. With no tools it is equivalent to BuildRequest.
func BuildToolRequest(model string, messages []Message, temperature float64, tools []Tool) ([]byte, error) {
	req := toolRequest{Model: model, Messages: messages, Temperature: temperature, Tools: tools}
	if len(tools) > 0 {
		req.ToolChoice = "auto"
	}
	return json.Marshal(req)
}

// chatChoice mirrors one choice of a chat-completions response, including the
// finish reason and any tool calls the model requested.
type chatChoice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// toolResponse decodes the parts of a chat response needed to drive a tool loop.
type toolResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
	Error   *APIError    `json:"error,omitempty"`
}

// ParseChat decodes a chat-completions response into the assistant's full message
// (content and/or tool calls), the finish reason, and token usage. Use this instead
// of ParseResponse when the request advertised tools: a non-empty Message.ToolCalls
// (finish_reason "tool_calls") means the model wants tools run.
func ParseChat(data []byte) (msg Message, finish string, usage Usage, err error) {
	var r toolResponse
	if e := json.Unmarshal(data, &r); e != nil {
		return Message{}, "", Usage{}, fmt.Errorf("ai: could not read the response: %w", e)
	}
	if r.Error != nil {
		return Message{}, "", Usage{}, fmt.Errorf("ai: %s", r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return Message{}, "", Usage{}, errors.New("ai: the response was empty")
	}
	return r.Choices[0].Message, r.Choices[0].FinishReason, r.Usage, nil
}

// WantsTools reports whether an assistant message is asking to run tools.
func WantsTools(msg Message) bool { return len(msg.ToolCalls) > 0 }

// ToolResultMessage builds the role:"tool" message that returns a tool call's
// result to the model, keyed back to the originating call by ID.
func ToolResultMessage(callID, name, content string) Message {
	return Message{Role: RoleTool, Content: content, ToolCallID: callID, Name: name}
}
