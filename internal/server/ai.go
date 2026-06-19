package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

var defaultAIModels = []string{"gpt-4o-mini", "gpt-4.1-nano", "gpt-4.1-mini", "gpt-4o", "gpt-4.1", "o4-mini"}

type aiHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type AIService struct {
	store           *Store
	client          aiHTTPDoer
	enabled         bool
	baseURL         string
	masterKey       []byte
	allowedModels   map[string]struct{}
	upstreamTimeout time.Duration
	upstreamRetries int
	requestMaxBytes int64
	requestsPerDay  int64
	tokensPerDay    int64
	now             func() time.Time
}

type AIServiceConfig struct {
	MasterKey       []byte
	BaseURL         string
	Client          aiHTTPDoer
	Disabled        bool
	AllowedModels   []string
	UpstreamTimeout time.Duration
	UpstreamRetries int
	RequestMaxBytes int64
	RequestsPerDay  int64
	TokensPerDay    int64
	Now             func() time.Time
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
	allowedModels := make(map[string]struct{}, len(cfg.AllowedModels))
	for _, model := range cfg.AllowedModels {
		model = strings.TrimSpace(model)
		if model != "" {
			allowedModels[model] = struct{}{}
		}
	}
	return &AIService{
		store:           store,
		client:          client,
		enabled:         !cfg.Disabled,
		baseURL:         baseURL,
		masterKey:       cfg.MasterKey,
		allowedModels:   allowedModels,
		upstreamTimeout: cfg.UpstreamTimeout,
		upstreamRetries: cfg.UpstreamRetries,
		requestMaxBytes: cfg.RequestMaxBytes,
		requestsPerDay:  cfg.RequestsPerDay,
		tokensPerDay:    cfg.TokensPerDay,
		now:             now,
	}
}

func (s *AIService) Chat(ctx context.Context, req AIChatRequest) (AICompletion, error) {
	if err := s.ensureEnabled(); err != nil {
		return AICompletion{}, err
	}
	if strings.TrimSpace(req.Model) == "" || len(req.Messages) == 0 {
		return AICompletion{}, status.Error(codes.InvalidArgument, "model and messages are required")
	}
	if err := s.validateModel(req.Model); err != nil {
		return AICompletion{}, err
	}
	body, err := ai.BuildRequest(req.Model, req.Messages, req.Temperature)
	if err != nil {
		return AICompletion{}, status.Errorf(codes.InvalidArgument, "build chat request: %v", err)
	}
	return s.complete(ctx, body)
}

func (s *AIService) Vision(ctx context.Context, req AIVisionRequest) (AICompletion, error) {
	if err := s.ensureEnabled(); err != nil {
		return AICompletion{}, err
	}
	if strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.SystemPrompt) == "" || strings.TrimSpace(req.UserText) == "" || strings.TrimSpace(req.ImageURL) == "" {
		return AICompletion{}, status.Error(codes.InvalidArgument, "model, system prompt, user text, and image url are required")
	}
	if err := s.validateModel(req.Model); err != nil {
		return AICompletion{}, err
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

func (s *AIService) ListModels(context.Context) []string {
	if s == nil || len(s.allowedModels) == 0 {
		return append([]string(nil), defaultAIModels...)
	}
	models := make([]string, 0, len(s.allowedModels))
	for model := range s.allowedModels {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}

func (s *AIService) ensureEnabled() error {
	if s != nil && !s.enabled {
		return status.Error(codes.FailedPrecondition, "ai proxy is disabled")
	}
	return nil
}

func (s *AIService) complete(ctx context.Context, body []byte) (AICompletion, error) {
	if s == nil || s.store == nil {
		return AICompletion{}, status.Error(codes.FailedPrecondition, "ai service store is not configured")
	}
	user, err := syncUser(ctx)
	if err != nil {
		return AICompletion{}, err
	}
	if s.requestMaxBytes > 0 && int64(len(body)) > s.requestMaxBytes {
		return AICompletion{}, status.Error(codes.ResourceExhausted, "ai request is too large")
	}
	if err := s.checkUsageLimit(user.ID, s.now()); err != nil {
		return AICompletion{}, err
	}
	key, ok, err := s.store.GetAIKey(user.ID, "openai", s.masterKey)
	if err != nil {
		return AICompletion{}, fmt.Errorf("server ai: get key: %w", err)
	}
	if !ok {
		return AICompletion{}, status.Error(codes.FailedPrecondition, "openai key is not configured")
	}
	upstreamCtx := ctx
	cancel := func() {}
	if s.upstreamTimeout > 0 {
		upstreamCtx, cancel = context.WithTimeout(ctx, s.upstreamTimeout)
	}
	defer cancel()
	resp, err := s.doUpstream(upstreamCtx, body, key)
	if err != nil {
		if ctx.Err() != nil {
			return AICompletion{}, status.Error(codes.Canceled, "ai request canceled")
		}
		if upstreamCtx.Err() == context.DeadlineExceeded {
			return AICompletion{}, status.Error(codes.DeadlineExceeded, "openai request timed out")
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

func (s *AIService) doUpstream(ctx context.Context, body []byte, key string) (*http.Response, error) {
	attempts := s.upstreamRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "build upstream request: %v", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+key)
		httpReq.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(httpReq)
		if err == nil {
			if !retryableOpenAIStatus(resp.StatusCode) || attempt == attempts-1 {
				return resp, nil
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			_ = resp.Body.Close()
		} else {
			lastErr = err
			if attempt == attempts-1 || ctx.Err() != nil {
				return nil, err
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryBackoff(attempt)):
		}
	}
	return nil, lastErr
}

func retryableOpenAIStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := 100 * time.Millisecond
	for i := 0; i < attempt && delay < time.Second; i++ {
		delay *= 2
	}
	jitterMax := delay / 2
	if jitterMax <= 0 {
		return delay
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(jitterMax)))
	if err != nil {
		return delay
	}
	return delay + time.Duration(n.Int64())
}

func (s *AIService) validateModel(model string) error {
	if len(s.allowedModels) == 0 {
		return nil
	}
	if _, ok := s.allowedModels[strings.TrimSpace(model)]; ok {
		return nil
	}
	return status.Error(codes.InvalidArgument, "model is not allowed")
}

func (s *AIService) checkUsageLimit(userID string, day time.Time) error {
	if s.requestsPerDay <= 0 && s.tokensPerDay <= 0 {
		return nil
	}
	usage, ok, err := s.store.GetUsage(userID, day)
	if err != nil {
		return fmt.Errorf("server ai: get usage: %w", err)
	}
	if !ok {
		return nil
	}
	if s.requestsPerDay > 0 && usage.Requests >= s.requestsPerDay {
		return status.Error(codes.ResourceExhausted, "daily ai request limit reached")
	}
	if s.tokensPerDay > 0 && usage.Tokens >= s.tokensPerDay {
		return status.Error(codes.ResourceExhausted, "daily ai token limit reached")
	}
	return nil
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
