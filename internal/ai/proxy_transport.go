//go:build js && wasm

package ai

import (
	"context"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc/status"
)

func SendProxyChat(endpoint, token, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	return invokeProxyCompletion(endpoint, token, backendrpc.MethodAIChat, backendrpc.ChatRequest{
		Model:       model,
		Messages:    rpcMessages(messages),
		Temperature: temperature,
	}, onResult, onError)
}

func SendProxyStructuredVisionChat(endpoint, token, model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func() {
	return invokeProxyCompletion(endpoint, token, backendrpc.MethodAIVision, backendrpc.VisionRequest{
		Model:        strings.TrimSpace(model),
		SystemPrompt: systemPrompt,
		UserText:     userText,
		ImageURL:     imageURL,
		Temperature:  temperature,
		SchemaName:   strings.TrimSpace(schemaName),
		Schema:       schema,
	}, onResult, onError)
}

func rpcMessages(messages []Message) []backendrpc.Message {
	out := make([]backendrpc.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, backendrpc.Message{Role: msg.Role, Content: msg.Content})
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

func invokeProxyCompletion(endpoint, token, method string, req any, onResult func(string, Usage), onError func(string)) func() {
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
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: endpoint, Token: token})
		if err != nil {
			if !cancelled {
				onError("Couldn't reach the backend server.")
			}
			return
		}
		defer conn.Close()
		var out backendrpc.Completion
		err = conn.Invoke(ctx, method, req, &out, backendrpc.JSONCallOptions()...)
		if cancelled {
			return
		}
		if err != nil {
			if st, ok := status.FromError(err); ok && strings.TrimSpace(st.Message()) != "" {
				onError(st.Message())
				return
			}
			onError(err.Error())
			return
		}
		onResult(out.Content, rpcUsage(out.Usage))
	}()
	return cancel
}
