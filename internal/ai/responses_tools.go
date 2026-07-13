// SPDX-License-Identifier: MIT

// Responses-API codec for tool-calling chat (POST /responses). The Responses API
// is the only endpoint that accepts reasoning.effort ALONGSIDE function tools for
// the reasoning models (gpt-5.x / o-series) — /chat/completions rejects that combo —
// so the assistant routes its tool loop through here. This file has no build tags so
// it compiles on native Go for unit tests. It converts to/from the same ai.Message /
// ai.Tool shapes the chat-completions path uses, so the caller's tool loop is unchanged.
package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// responsesInputItem is one item in a Responses request's `input` array: a plain
// role message ({role, content}), a prior assistant tool call ({type:function_call,
// call_id, name, arguments}), or a tool result ({type:function_call_output, call_id,
// output}). omitempty keeps each JSON object to just the fields its kind needs.
type responsesInputItem struct {
	Type      string `json:"type,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

// responsesFunctionTool is a Responses-API function tool. Unlike chat-completions
// (which nests under a "function" key), Responses puts name/description/parameters
// flat on the tool object.
type responsesFunctionTool struct {
	Type        string          `json:"type"` // "function"
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// responsesReasoning carries the thinking level for reasoning models.
type responsesReasoning struct {
	Effort string `json:"effort"`
}

// responsesToolRequest is the POST /responses body for a tool-calling turn. Input is
// []json.RawMessage so reasoning items can be echoed back verbatim alongside the
// struct-built role / function_call / function_call_output items.
type responsesToolRequest struct {
	Model       string                  `json:"model"`
	Input       []json.RawMessage       `json:"input"`
	Tools       []responsesFunctionTool `json:"tools,omitempty"`
	ToolChoice  string                  `json:"tool_choice,omitempty"`
	Temperature float64                 `json:"temperature,omitempty"`
	Reasoning   *responsesReasoning     `json:"reasoning,omitempty"`
	Store       bool                    `json:"store"`
}

// messagesToResponsesInput converts the chat-completions message history into the
// Responses `input` array: role messages pass through, an assistant turn with tool
// calls echoes its reasoning items (required by the Responses API for reasoning
// models) then becomes function_call items (plus its text, if any), and a role:tool
// result becomes a function_call_output keyed by call id.
func messagesToResponsesInput(messages []Message) []json.RawMessage {
	items := make([]json.RawMessage, 0, len(messages))
	add := func(v responsesInputItem) {
		if b, err := json.Marshal(v); err == nil {
			items = append(items, b)
		}
	}
	for _, m := range messages {
		switch {
		case m.Role == RoleTool:
			add(responsesInputItem{Type: "function_call_output", CallID: m.ToolCallID, Output: m.Content})
		case len(m.ToolCalls) > 0:
			// The reasoning items that produced these tool calls must precede the
			// function_call items in the input, or the Responses API rejects the turn.
			items = append(items, m.ReasoningRaw...)
			if strings.TrimSpace(m.Content) != "" {
				add(responsesInputItem{Role: RoleAssistant, Content: m.Content})
			}
			for _, tc := range m.ToolCalls {
				add(responsesInputItem{Type: "function_call", CallID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments})
			}
		default:
			add(responsesInputItem{Role: m.Role, Content: m.Content})
		}
	}
	return items
}

// toolsToResponses flattens chat-completions tools into Responses function tools.
func toolsToResponses(tools []Tool) []responsesFunctionTool {
	out := make([]responsesFunctionTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, responsesFunctionTool{Type: "function", Name: t.Function.Name, Description: t.Function.Description, Parameters: t.Function.Parameters})
	}
	return out
}

// BuildResponsesToolRequest marshals a POST /responses body for a tool-calling turn.
// reasoningEffort ("low"/"medium"/"high", or "" to omit) sets the thinking level for
// reasoning models — the Responses API accepts it together with function tools.
// temperature is included only when non-zero (reasoning models reject a custom temp).
// store=false so the exchange isn't retained on OpenAI's side (privacy).
func BuildResponsesToolRequest(model string, messages []Message, temperature float64, reasoningEffort string, tools []Tool) ([]byte, error) {
	req := responsesToolRequest{
		Model:       model,
		Input:       messagesToResponsesInput(messages),
		Tools:       toolsToResponses(tools),
		Temperature: temperature,
		Store:       false,
	}
	if len(tools) > 0 {
		req.ToolChoice = "auto"
	}
	if strings.TrimSpace(reasoningEffort) != "" {
		req.Reasoning = &responsesReasoning{Effort: reasoningEffort}
	}
	return json.Marshal(req)
}

// responsesToolOutputItem is one typed item of a Responses tool reply's output
// array: a "message" (assistant text) or a "function_call" (a requested tool call).
type responsesToolOutputItem struct {
	Type      string                 `json:"type"`
	Role      string                 `json:"role,omitempty"`
	Content   []responsesContentItem `json:"content,omitempty"`
	ID        string                 `json:"id,omitempty"`
	CallID    string                 `json:"call_id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Arguments string                 `json:"arguments,omitempty"`
}

// responsesToolReply is the top-level Responses tool response body. Output is kept
// raw so "reasoning" items can be carried back verbatim (see Message.ReasoningRaw).
type responsesToolReply struct {
	Output []json.RawMessage `json:"output"`
	Usage  responsesUsage    `json:"usage"`
	Error  *APIError         `json:"error,omitempty"`
}

// ParseResponsesChat decodes a Responses tool reply into the same ai.Message shape
// the chat-completions tool loop consumes: function_call items become ToolCalls
// (keyed by call_id), message output_text becomes Content, and any reasoning items
// are stashed verbatim in ReasoningRaw so the next turn can echo them. Usage maps the
// Responses token names onto the shared Usage fields.
func ParseResponsesChat(data []byte) (Message, Usage, error) {
	var r responsesToolReply
	if err := json.Unmarshal(data, &r); err != nil {
		return Message{}, Usage{}, fmt.Errorf("ai: could not read the Responses reply: %w", err)
	}
	if r.Error != nil {
		return Message{}, Usage{}, fmt.Errorf("ai: %s", r.Error.Message)
	}
	usage := Usage{
		PromptTokens:     r.Usage.InputTokens,
		CompletionTokens: r.Usage.OutputTokens,
		TotalTokens:      r.Usage.InputTokens + r.Usage.OutputTokens,
	}
	msg := Message{Role: RoleAssistant}
	for _, raw := range r.Output {
		var head struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &head) != nil {
			continue
		}
		switch head.Type {
		case "reasoning":
			msg.ReasoningRaw = append(msg.ReasoningRaw, raw)
		case "function_call":
			var item responsesToolOutputItem
			_ = json.Unmarshal(raw, &item)
			id := item.CallID
			if id == "" {
				id = item.ID
			}
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{ID: id, Type: "function", Function: FunctionCall{Name: item.Name, Arguments: item.Arguments}})
		case "message":
			var item responsesToolOutputItem
			_ = json.Unmarshal(raw, &item)
			for _, c := range item.Content {
				if c.Type == "output_text" {
					msg.Content += c.Text
				}
			}
		}
	}
	if msg.Content == "" && len(msg.ToolCalls) == 0 {
		return Message{}, usage, errors.New("ai: the Responses reply was empty")
	}
	return msg, usage, nil
}
