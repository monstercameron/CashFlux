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
		Streams:  []grpc.StreamDesc{},
		Metadata: "cashflux/v1/ai.proto",
	}, srv)
}

type aiServiceServer interface {
	SetKey(context.Context, backendrpc.SetKeyRequest) (backendrpc.SetKeyResponse, error)
	ListModelsRPC(context.Context, backendrpc.ListModelsRequest) (backendrpc.ListModelsResponse, error)
	ChatRPC(context.Context, backendrpc.ChatRequest) (backendrpc.Completion, error)
	VisionRPC(context.Context, backendrpc.VisionRequest) (backendrpc.Completion, error)
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
		Model:       req.Model,
		Messages:    aiMessages(req.Messages),
		Temperature: req.Temperature,
	})
	if err != nil {
		return backendrpc.Completion{}, err
	}
	return backendrpc.Completion{Content: out.Content, Usage: rpcUsage(out.Usage)}, nil
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

func aiMessages(messages []backendrpc.Message) []ai.Message {
	out := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, ai.Message{Role: msg.Role, Content: msg.Content})
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
