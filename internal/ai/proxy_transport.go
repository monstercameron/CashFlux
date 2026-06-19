//go:build js && wasm

package ai

import (
	"context"
	"io"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func SendProxyChat(endpoint, token, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	return invokeProxyCompletionStream(endpoint, token, backendrpc.MethodAIChatStream, backendrpc.ChatRequest{
		Model:       model,
		Messages:    rpcMessages(messages),
		Temperature: temperature,
	}, onResult, onError)
}

func SendProxyStructuredVisionChat(endpoint, token, model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func() {
	return invokeProxyCompletionStream(endpoint, token, backendrpc.MethodAIVisionStream, backendrpc.VisionRequest{
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

func invokeProxyCompletionStream(endpoint, token, method string, req any, onResult func(string, Usage), onError func(string)) func() {
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
		stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, method, backendrpc.JSONCallOptions()...)
		if err == nil {
			err = stream.SendMsg(req)
		}
		if err == nil {
			err = stream.CloseSend()
		}
		if cancelled {
			return
		}
		if err != nil {
			reportProxyError(err, onError)
			return
		}
		var content strings.Builder
		var usage backendrpc.Usage
		for {
			var chunk backendrpc.CompletionChunk
			err := stream.RecvMsg(&chunk)
			if cancelled {
				return
			}
			if err == io.EOF {
				onResult(content.String(), rpcUsage(usage))
				return
			}
			if err != nil {
				reportProxyError(err, onError)
				return
			}
			content.WriteString(chunk.Content)
			if chunk.Usage.TotalTokens != 0 || chunk.Usage.PromptTokens != 0 || chunk.Usage.CompletionTokens != 0 {
				usage = chunk.Usage
			}
			if chunk.Done {
				onResult(content.String(), rpcUsage(usage))
				return
			}
		}
	}()
	return cancel
}

func reportProxyError(err error, onError func(string)) {
	if st, ok := status.FromError(err); ok && strings.TrimSpace(st.Message()) != "" {
		onError(st.Message())
		return
	}
	onError(err.Error())
}
