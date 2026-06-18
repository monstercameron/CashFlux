package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

type aiHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type AIService struct {
	store     *Store
	client    aiHTTPDoer
	baseURL   string
	masterKey []byte
	now       func() time.Time
}

type AIServiceConfig struct {
	MasterKey []byte
	BaseURL   string
	Client    aiHTTPDoer
	Now       func() time.Time
}

type AIChatRequest struct {
	Model       string       `json:"model"`
	Messages    []ai.Message `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
}

type AIVisionRequest struct {
	Model        string          `json:"model"`
	SystemPrompt string          `json:"systemPrompt"`
	UserText     string          `json:"userText"`
	ImageURL     string          `json:"imageUrl"`
	Temperature  float64         `json:"temperature,omitempty"`
	SchemaName   string          `json:"schemaName,omitempty"`
	Schema       json.RawMessage `json:"schema,omitempty"`
}

type AICompletion struct {
	Content string   `json:"content"`
	Usage   ai.Usage `json:"usage"`
}

func NewAIService(store *Store, cfg AIServiceConfig) *AIService {
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AIService{store: store, client: client, baseURL: baseURL, masterKey: cfg.MasterKey, now: now}
}

func (s *AIService) Chat(ctx context.Context, req AIChatRequest) (AICompletion, error) {
	if strings.TrimSpace(req.Model) == "" || len(req.Messages) == 0 {
		return AICompletion{}, status.Error(codes.InvalidArgument, "model and messages are required")
	}
	body, err := ai.BuildRequest(req.Model, req.Messages, req.Temperature)
	if err != nil {
		return AICompletion{}, status.Errorf(codes.InvalidArgument, "build chat request: %v", err)
	}
	return s.complete(ctx, body)
}

func (s *AIService) Vision(ctx context.Context, req AIVisionRequest) (AICompletion, error) {
	if strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.SystemPrompt) == "" || strings.TrimSpace(req.UserText) == "" || strings.TrimSpace(req.ImageURL) == "" {
		return AICompletion{}, status.Error(codes.InvalidArgument, "model, system prompt, user text, and image url are required")
	}
	var (
		body []byte
		err  error
	)
	if len(req.Schema) > 0 || strings.TrimSpace(req.SchemaName) != "" {
		if strings.TrimSpace(req.SchemaName) == "" || len(req.Schema) == 0 {
			return AICompletion{}, status.Error(codes.InvalidArgument, "schema name and schema are required together")
		}
		body, err = ai.BuildStructuredVisionRequest(req.Model, req.SystemPrompt, req.UserText, req.ImageURL, req.Temperature, req.SchemaName, req.Schema)
	} else {
		body, err = ai.BuildVisionRequest(req.Model, req.SystemPrompt, req.UserText, req.ImageURL, req.Temperature)
	}
	if err != nil {
		return AICompletion{}, status.Errorf(codes.InvalidArgument, "build vision request: %v", err)
	}
	return s.complete(ctx, body)
}

func (s *AIService) complete(ctx context.Context, body []byte) (AICompletion, error) {
	if s == nil || s.store == nil {
		return AICompletion{}, status.Error(codes.FailedPrecondition, "ai service store is not configured")
	}
	user, err := syncUser(ctx)
	if err != nil {
		return AICompletion{}, err
	}
	key, ok, err := s.store.GetAIKey(user.ID, "openai", s.masterKey)
	if err != nil {
		return AICompletion{}, fmt.Errorf("server ai: get key: %w", err)
	}
	if !ok {
		return AICompletion{}, status.Error(codes.FailedPrecondition, "openai key is not configured")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return AICompletion{}, status.Errorf(codes.Internal, "build upstream request: %v", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return AICompletion{}, status.Error(codes.Canceled, "ai request canceled")
		}
		return AICompletion{}, status.Errorf(codes.Unavailable, "openai request failed: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return AICompletion{}, status.Errorf(codes.Unavailable, "read openai response: %v", err)
	}
	if resp.StatusCode >= 400 {
		return AICompletion{}, status.Error(openAICode(resp.StatusCode), ai.ErrorMessage(resp.StatusCode, data))
	}
	content, err := ai.ParseResponse(data)
	if err != nil {
		return AICompletion{}, status.Errorf(codes.Internal, "parse openai response: %v", err)
	}
	usage := ai.ParseUsage(data)
	if _, err := s.store.AddUsage(user.ID, s.now(), 1, int64(usage.TotalTokens)); err != nil {
		return AICompletion{}, fmt.Errorf("server ai: add usage: %w", err)
	}
	return AICompletion{Content: content, Usage: usage}, nil
}

func openAICode(httpStatus int) codes.Code {
	switch httpStatus {
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusBadRequest:
		return codes.InvalidArgument
	default:
		if httpStatus >= 500 {
			return codes.Unavailable
		}
		return codes.Unknown
	}
}
