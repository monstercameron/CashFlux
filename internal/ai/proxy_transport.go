// SPDX-License-Identifier: MIT

//go:build js && wasm

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
	"github.com/monstercameron/CashFlux/internal/rpcworker"
)

// SendProxyChat asks the backend proxy for a plain answer — no tools. Callers that
// want the model to be able to call tools use SendProxyChatTools.
func SendProxyChat(endpoint, token, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	return SendProxyChatTools(endpoint, token, model, messages, temperature, "", nil,
		func(msg Message, u Usage) { onResult(msg.Content, u) }, onError)
}

// SendProxyChatTools runs one turn through the backend proxy with tools advertised,
// handing back the model's whole message — its text, or the tool calls it wants run.
// This is the parity fix for the server path: a household on the shared server key
// gets the same tool-using assistant as one on a direct key, instead of a quietly
// weaker one that could only answer from whatever context was injected.
func SendProxyChatTools(endpoint, token, model string, messages []Message, temperature float64, reasoningEffort string, tools []Tool, onResult func(Message, Usage), onError func(string)) func() {
	return SendProxyChatToolsStreaming(endpoint, token, model, messages, temperature, reasoningEffort, tools, nil, onResult, onError)
}

// SendProxyChatToolsStreaming is SendProxyChatTools with the answer surfaced as it
// arrives: onDelta receives each fragment the server forwards. onDelta may be nil
// for a caller that only wants the finished answer.
func SendProxyChatToolsStreaming(endpoint, token, model string, messages []Message, temperature float64, reasoningEffort string, tools []Tool, onDelta func(string), onResult func(Message, Usage), onError func(string)) func() {
	return invokeProxyCompletionStreamDelta(endpoint, token, backendrpc.MethodAIChatStream, backendrpc.ChatRequest{
		Model:           model,
		Messages:        rpcMessages(messages),
		Temperature:     temperature,
		Tools:           rpcTools(tools),
		ReasoningEffort: reasoningEffort,
	}, onDelta, onResult, onError)
}

func SendProxyStructuredVisionChat(endpoint, token, model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func() {
	toText := func(msg Message, u Usage) { onResult(msg.Content, u) }
	return invokeProxyCompletionStream(endpoint, token, backendrpc.MethodAIVisionStream, backendrpc.VisionRequest{
		Model:        strings.TrimSpace(model),
		SystemPrompt: systemPrompt,
		UserText:     userText,
		ImageURL:     imageURL,
		Temperature:  temperature,
		SchemaName:   strings.TrimSpace(schemaName),
		Schema:       schema,
	}, toText, onError)
}

func rpcMessages(messages []Message) []backendrpc.Message {
	out := make([]backendrpc.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, backendrpc.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  rpcToolCalls(msg.ToolCalls),
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
			// The reasoning items that produced a tool call have to travel with it:
			// the Responses API rejects a turn that echoes the call without them.
			Reasoning: msg.ReasoningRaw,
		})
	}
	return out
}

func rpcTools(tools []Tool) []backendrpc.Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]backendrpc.Tool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, backendrpc.Tool{
			Type: tool.Type,
			Function: backendrpc.FunctionDef{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}
	return out
}

func rpcToolCalls(calls []ToolCall) []backendrpc.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]backendrpc.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, backendrpc.ToolCall{
			ID:       call.ID,
			Type:     call.Type,
			Function: backendrpc.FunctionCall{Name: call.Function.Name, Arguments: call.Function.Arguments},
		})
	}
	return out
}

func toolCallsFromRPC(calls []backendrpc.ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, ToolCall{
			ID:       call.ID,
			Type:     call.Type,
			Function: FunctionCall{Name: call.Function.Name, Arguments: call.Function.Arguments},
		})
	}
	return out
}

func rpcUsage(usage backendrpc.Usage) Usage {
	return Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func invokeProxyCompletionStream(endpoint, token, method string, req any, onResult func(Message, Usage), onError func(string)) func() {
	return invokeProxyCompletionStreamDelta(endpoint, token, method, req, nil, onResult, onError)
}

func invokeProxyCompletionStreamDelta(endpoint, token, method string, req any, onDelta func(string), onResult func(Message, Usage), onError func(string)) func() {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	token = strings.TrimSpace(token)
	if endpoint == "" || token == "" {
		onError("Backend URL and token are required.")
		return noopCancel
	}
	cancelled := false
	ctx, cancelContext := context.WithCancel(context.Background())
	cancel := func() {
		if cancelled {
			return
		}
		cancelled = true
		cancelContext()
	}
	go func() {
		client := rpcworker.Default()
		if client == nil {
			var err error
			client, err = rpcworker.StartDefault()
			if err != nil {
				if !cancelled {
					onError("Couldn't start the backend service worker.")
				}
				return
			}
		}
		stream, err := client.OpenServerStream(ctx, endpoint, token, method, req)
		if err != nil {
			if !cancelled {
				onError("Couldn't reach the backend server.")
			}
			return
		}
		defer stream.Cancel()
		var content strings.Builder
		var usage backendrpc.Usage
		var toolCalls []backendrpc.ToolCall
		var reasoning []json.RawMessage
		var finish string
		// A turn that only asks for tools carries no text, so "finished with nothing"
		// is a real answer here rather than a failure — the caller decides what to do
		// with an empty message that has tool calls attached.
		finished := func() {
			onResult(Message{
				Role:         RoleAssistant,
				Content:      content.String(),
				ToolCalls:    toolCallsFromRPC(toolCalls),
				ReasoningRaw: reasoning,
			}, rpcUsage(usage))
		}
		for {
			var chunk backendrpc.CompletionChunk
			err := stream.Recv(&chunk)
			if cancelled {
				return
			}
			if err == io.EOF {
				finished()
				return
			}
			if err != nil {
				reportProxyError(err, onError)
				return
			}
			content.WriteString(chunk.Content)
			// Each intermediate chunk is a fragment of the answer being written.
			// The terminal chunk carries the accounting and repeats no text, so
			// forwarding its (empty) content would be a no-op either way.
			if onDelta != nil && chunk.Content != "" && !chunk.Done {
				onDelta(chunk.Content)
			}
			if len(chunk.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.ToolCalls...)
			}
			if len(chunk.Reasoning) > 0 {
				reasoning = append(reasoning, chunk.Reasoning...)
			}
			if chunk.FinishReason != "" {
				finish = chunk.FinishReason
			}
			if chunk.Usage.TotalTokens != 0 || chunk.Usage.PromptTokens != 0 || chunk.Usage.CompletionTokens != 0 {
				usage = chunk.Usage
			}
			if chunk.Done {
				if content.Len() == 0 && len(toolCalls) == 0 {
					// The server refuses empty completions, so reaching here means a
					// stream ended without ever carrying an answer. Say why rather than
					// handing the screen a blank bubble.
					onError(EmptyReplyMessage(finish))
					return
				}
				finished()
				return
			}
		}
	}()
	return cancel
}

func reportProxyError(err error, onError func(string)) {
	var rpcErr *rpcprotocol.Error
	if errors.As(err, &rpcErr) && strings.TrimSpace(rpcErr.Message) != "" {
		onError(rpcErr.Message)
		return
	}
	onError(err.Error())
}
