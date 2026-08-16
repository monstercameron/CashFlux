// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RegisterAIServiceServer(s grpc.ServiceRegistrar, srv *AIService) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "cashflux.v1.AIService",
		HandlerType: (*aiServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "SetKey", Handler: aiSetKeyHandler},
			{MethodName: "ListModels", Handler: aiListModelsHandler},
			{MethodName: "Chat", Handler: aiChatHandler},
			{MethodName: "Vision", Handler: aiVisionHandler},
		},
		Streams: []grpc.StreamDesc{
			{StreamName: "ChatStream", Handler: aiChatStreamHandler, ServerStreams: true},
			{StreamName: "VisionStream", Handler: aiVisionStreamHandler, ServerStreams: true},
		},
		Metadata: "cashflux/v1/ai.proto",
	}, srv)
}

type aiServiceServer interface {
	SetKey(context.Context, backendrpc.SetKeyRequest) (backendrpc.SetKeyResponse, error)
	ListModelsRPC(context.Context, backendrpc.ListModelsRequest) (backendrpc.ListModelsResponse, error)
	ChatRPC(context.Context, backendrpc.ChatRequest) (backendrpc.Completion, error)
	VisionRPC(context.Context, backendrpc.VisionRequest) (backendrpc.Completion, error)
	ChatStreamRPC(backendrpc.ChatRequest, grpc.ServerStream) error
	VisionStreamRPC(backendrpc.VisionRequest, grpc.ServerStream) error
}

func (s *AIService) SetKey(ctx context.Context, req backendrpc.SetKeyRequest) (backendrpc.SetKeyResponse, error) {
	if err := s.ensureEnabled(); err != nil {
		return backendrpc.SetKeyResponse{}, err
	}
	if s == nil || s.store == nil {
		return backendrpc.SetKeyResponse{}, status.Error(codes.FailedPrecondition, "ai service store is not configured")
	}
	if len(s.masterKey) == 0 {
		return backendrpc.SetKeyResponse{}, status.Error(codes.FailedPrecondition, "master key is not configured")
	}
	user, err := syncUser(ctx)
	if err != nil {
		return backendrpc.SetKeyResponse{}, err
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "openai"
	}
	if len(provider) > maxAIProviderLength {
		return backendrpc.SetKeyResponse{}, status.Error(codes.InvalidArgument, "provider is too long")
	}
	key := strings.TrimSpace(req.Key)
	if len(key) > maxAIKeyLength {
		return backendrpc.SetKeyResponse{}, status.Error(codes.InvalidArgument, "openai key is too long")
	}
	// An empty key is the "remove" action — delete any stored key for this provider
	// (§7.11 Cloud AI-key replace/remove) without needing a separate RPC method.
	if key == "" {
		if err := s.store.DeleteAIKey(user.ID, provider); err != nil {
			return backendrpc.SetKeyResponse{}, err
		}
		auditFromContext(ctx, s.store, "ai_key.remove", "provider", provider)
		return backendrpc.SetKeyResponse{Stored: false, Provider: provider}, nil
	}
	if provider != "openai" || !strings.HasPrefix(key, "sk-") {
		return backendrpc.SetKeyResponse{}, status.Error(codes.InvalidArgument, "invalid openai key")
	}
	if err := s.store.UpsertUser(User{ID: user.ID, Provider: "token", Subject: user.ID, CreatedAt: time.Now().UTC()}); err != nil {
		return backendrpc.SetKeyResponse{}, err
	}
	if err := s.store.PutAIKey(user.ID, provider, key, s.masterKey); err != nil {
		return backendrpc.SetKeyResponse{}, err
	}
	auditFromContext(ctx, s.store, "ai_key.set", "provider", provider)
	return backendrpc.SetKeyResponse{Stored: true, Provider: provider}, nil
}

func (s *AIService) ListModelsRPC(ctx context.Context, req backendrpc.ListModelsRequest) (backendrpc.ListModelsResponse, error) {
	if err := s.ensureEnabled(); err != nil {
		return backendrpc.ListModelsResponse{}, err
	}
	if _, err := syncUser(ctx); err != nil {
		return backendrpc.ListModelsResponse{}, err
	}
	return backendrpc.ListModelsResponse{Models: s.ListModels(ctx)}, nil
}

func (s *AIService) ChatRPC(ctx context.Context, req backendrpc.ChatRequest) (backendrpc.Completion, error) {
	out, err := s.Chat(ctx, AIChatRequest{
		Model:           req.Model,
		Messages:        aiMessages(req.Messages),
		Temperature:     req.Temperature,
		Tools:           aiTools(req.Tools),
		ReasoningEffort: req.ReasoningEffort,
	})
	if err != nil {
		return backendrpc.Completion{}, err
	}
	return backendrpc.Completion{
		Content:      out.Content,
		Usage:        rpcUsage(out.Usage),
		ToolCalls:    rpcToolCalls(out.ToolCalls),
		FinishReason: out.FinishReason,
		Reasoning:    out.Reasoning,
	}, nil
}

func (s *AIService) VisionRPC(ctx context.Context, req backendrpc.VisionRequest) (backendrpc.Completion, error) {
	out, err := s.Vision(ctx, AIVisionRequest{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		UserText:     req.UserText,
		ImageURL:     req.ImageURL,
		Temperature:  req.Temperature,
		SchemaName:   req.SchemaName,
		Schema:       req.Schema,
	})
	if err != nil {
		return backendrpc.Completion{}, err
	}
	return backendrpc.Completion{Content: out.Content, Usage: rpcUsage(out.Usage)}, nil
}

func (s *AIService) ChatStreamRPC(req backendrpc.ChatRequest, stream grpc.ServerStream) error {
	start := time.Now()
	statusCode := codes.OK.String()
	defer func() {
		if s.metrics != nil {
			s.metrics.ObserveStreamDuration(backendrpc.MethodAIChatStream, statusCode, time.Since(start))
		}
	}()
	// Each fragment goes out as its own chunk, so the browser paints the answer as
	// it is written instead of waiting for the whole thing. A send failure means the
	// client has gone; sendErr stops the forwarding rather than letting the upstream
	// read run on to fill a connection nobody is holding.
	var sendErr error
	completion, err := s.ChatStream(stream.Context(), AIChatRequest{
		Model:           req.Model,
		Messages:        aiMessages(req.Messages),
		Temperature:     req.Temperature,
		Tools:           aiTools(req.Tools),
		ReasoningEffort: req.ReasoningEffort,
	}, func(delta string) {
		if sendErr != nil || delta == "" {
			return
		}
		sendErr = stream.SendMsg(&backendrpc.CompletionChunk{Content: delta})
	})
	if err != nil {
		statusCode = status.Code(err).String()
		return err
	}
	if sendErr != nil {
		statusCode = status.Code(sendErr).String()
		return sendErr
	}
	// The final chunk carries the accounting and the tool calls, and repeats no
	// text: the deltas already delivered it, and sending it twice would double the
	// answer on screen.
	if err := stream.SendMsg(&backendrpc.CompletionChunk{
		Usage:        rpcUsage(completion.Usage),
		ToolCalls:    rpcToolCalls(completion.ToolCalls),
		FinishReason: completion.FinishReason,
		Reasoning:    completion.Reasoning,
		Done:         true,
	}); err != nil {
		statusCode = status.Code(err).String()
		return err
	}
	return nil
}

func (s *AIService) VisionStreamRPC(req backendrpc.VisionRequest, stream grpc.ServerStream) error {
	start := time.Now()
	statusCode := codes.OK.String()
	defer func() {
		if s.metrics != nil {
			s.metrics.ObserveStreamDuration(backendrpc.MethodAIVisionStream, statusCode, time.Since(start))
		}
	}()
	completion, err := s.VisionRPC(stream.Context(), req)
	if err != nil {
		statusCode = status.Code(err).String()
		return err
	}
	if err := stream.SendMsg(&backendrpc.CompletionChunk{Content: completion.Content, Usage: completion.Usage, Done: true}); err != nil {
		statusCode = status.Code(err).String()
		return err
	}
	return nil
}

func aiMessages(messages []backendrpc.Message) []ai.Message {
	out := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, ai.Message{
			Role:         msg.Role,
			Content:      msg.Content,
			ToolCalls:    aiToolCalls(msg.ToolCalls),
			ToolCallID:   msg.ToolCallID,
			Name:         msg.Name,
			ReasoningRaw: msg.Reasoning,
		})
	}
	return out
}

func aiToolCalls(calls []backendrpc.ToolCall) []ai.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ai.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, ai.ToolCall{
			ID:       call.ID,
			Type:     call.Type,
			Function: ai.FunctionCall{Name: call.Function.Name, Arguments: call.Function.Arguments},
		})
	}
	return out
}

func rpcToolCalls(calls []ai.ToolCall) []backendrpc.ToolCall {
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

func aiTools(tools []backendrpc.Tool) []ai.Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ai.Tool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, ai.Tool{
			Type: tool.Type,
			Function: ai.FunctionDef{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}
	return out
}

func rpcUsage(usage ai.Usage) backendrpc.Usage {
	return backendrpc.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func aiSetKeyHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.SetKeyRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(aiServiceServer).SetKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAISetKey}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(aiServiceServer).SetKey(ctx, req.(backendrpc.SetKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func aiListModelsHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.ListModelsRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(aiServiceServer).ListModelsRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAIListModels}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(aiServiceServer).ListModelsRPC(ctx, req.(backendrpc.ListModelsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func aiChatHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.ChatRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(aiServiceServer).ChatRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAIChat}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(aiServiceServer).ChatRPC(ctx, req.(backendrpc.ChatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func aiVisionHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.VisionRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(aiServiceServer).VisionRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAIVision}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(aiServiceServer).VisionRPC(ctx, req.(backendrpc.VisionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func aiChatStreamHandler(srv any, stream grpc.ServerStream) error {
	var in backendrpc.ChatRequest
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	return srv.(aiServiceServer).ChatStreamRPC(in, stream)
}

func aiVisionStreamHandler(srv any, stream grpc.ServerStream) error {
	var in backendrpc.VisionRequest
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	return srv.(aiServiceServer).VisionStreamRPC(in, stream)
}
