// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

var defaultAIModels = []string{"gpt-4o-mini", "gpt-4.1-nano", "gpt-4.1-mini", "gpt-4o", "gpt-4.1", "o4-mini"}

const (
	maxAIModelLength          = 128
	maxAIChatMessages         = 64
	maxAIMessageContentBytes  = 64 << 10
	maxAIChatContentBytes     = 256 << 10
	maxAIProviderLength       = 32
	maxAIKeyLength            = 1024
	maxAIVisionPromptBytes    = 64 << 10
	maxAIVisionImageURLBytes  = 4 << 20
	maxAIVisionSchemaNameSize = 128
	maxAIVisionSchemaBytes    = 64 << 10
	aiCircuitFailureThreshold = 3
	aiCircuitCooldown         = 30 * time.Second
)

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
	alertRequests   int64
	alertTokens     int64
	blockedUsers    map[string]struct{}
	metrics         *Metrics
	now             func() time.Time
	circuitMu       sync.Mutex
	circuitFailures int
	circuitOpenTil  time.Time
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
	AlertRequests   int64
	AlertTokens     int64
	BlockedUserIDs  []string
	Metrics         *Metrics
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
	blockedUsers := make(map[string]struct{}, len(cfg.BlockedUserIDs))
	for _, userID := range cfg.BlockedUserIDs {
		userID = strings.TrimSpace(userID)
		if userID != "" {
			blockedUsers[userID] = struct{}{}
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
		alertRequests:   cfg.AlertRequests,
		alertTokens:     cfg.AlertTokens,
		blockedUsers:    blockedUsers,
		metrics:         cfg.Metrics,
		now:             now,
	}
}

func (s *AIService) Chat(ctx context.Context, req AIChatRequest) (AICompletion, error) {
	if err := s.ensureEnabled(); err != nil {
		return AICompletion{}, err
	}
	req.Model = strings.TrimSpace(req.Model)
	if err := s.validateModel(req.Model); err != nil {
		return AICompletion{}, err
	}
	if err := validateAIChatRequest(req); err != nil {
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
	req.Model = strings.TrimSpace(req.Model)
	if err := s.validateModel(req.Model); err != nil {
		return AICompletion{}, err
	}
	if err := validateAIVisionRequest(req); err != nil {
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
	if s.aiBlocked(user.ID) {
		return AICompletion{}, status.Error(codes.PermissionDenied, "ai proxy is disabled for this user")
	}
	if s.requestMaxBytes > 0 && int64(len(body)) > s.requestMaxBytes {
		return AICompletion{}, status.Error(codes.ResourceExhausted, "ai request is too large")
	}
	day := s.now()
	if err := s.checkUsageLimit(user.ID, day); err != nil {
		return AICompletion{}, err
	}
	if err := s.checkAIUpstreamCircuit(day); err != nil {
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
			s.recordAIUpstreamFailure(day)
			return AICompletion{}, status.Error(codes.DeadlineExceeded, "openai request timed out")
		}
		s.recordAIUpstreamFailure(day)
		return AICompletion{}, status.Errorf(codes.Unavailable, "openai request failed: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return AICompletion{}, status.Errorf(codes.Unavailable, "read openai response: %v", err)
	}
	if resp.StatusCode >= 400 {
		if resp.StatusCode >= 500 {
			s.recordAIUpstreamFailure(day)
		} else {
			s.recordAIUpstreamSuccess()
		}
		return AICompletion{}, status.Error(openAICode(resp.StatusCode), ai.ErrorMessage(resp.StatusCode, data))
	}
	s.recordAIUpstreamSuccess()
	content, err := ai.ParseResponse(data)
	if err != nil {
		return AICompletion{}, status.Errorf(codes.Internal, "parse openai response: %v", err)
	}
	usage := ai.ParseUsage(data)
	previous, _, err := s.store.GetUsage(user.ID, day)
	if err != nil {
		return AICompletion{}, fmt.Errorf("server ai: get usage before alert: %w", err)
	}
	next, err := s.store.AddUsage(user.ID, day, 1, int64(usage.TotalTokens))
	if err != nil {
		return AICompletion{}, fmt.Errorf("server ai: add usage: %w", err)
	}
	s.auditAIUsageAlerts(ctx, day, previous, next)
	s.metrics.ObserveAIProxy(int64(usage.TotalTokens))
	return AICompletion{Content: content, Usage: usage}, nil
}

func (s *AIService) aiBlocked(userID string) bool {
	if s == nil || len(s.blockedUsers) == 0 {
		return false
	}
	_, ok := s.blockedUsers[strings.TrimSpace(userID)]
	return ok
}

func (s *AIService) checkAIUpstreamCircuit(now time.Time) error {
	if s == nil {
		return nil
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	if !s.circuitOpenTil.IsZero() && now.Before(s.circuitOpenTil) {
		return status.Error(codes.Unavailable, "openai upstream circuit is open")
	}
	if !s.circuitOpenTil.IsZero() && !now.Before(s.circuitOpenTil) {
		s.circuitOpenTil = time.Time{}
		s.circuitFailures = 0
	}
	return nil
}

func (s *AIService) recordAIUpstreamFailure(now time.Time) {
	if s == nil {
		return
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	s.circuitFailures++
	if s.circuitFailures >= aiCircuitFailureThreshold {
		s.circuitOpenTil = now.Add(aiCircuitCooldown)
	}
}

func (s *AIService) recordAIUpstreamSuccess() {
	if s == nil {
		return
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	s.circuitFailures = 0
	s.circuitOpenTil = time.Time{}
}

func (s *AIService) auditAIUsageAlerts(ctx context.Context, day time.Time, previous, next Usage) {
	if s == nil || s.store == nil {
		return
	}
	if crossedUsageAlert(previous.Requests, next.Requests, s.alertRequests) {
		auditFromContext(ctx, s.store, "ai.usage_alert.requests", "usage", usageDay(day))
	}
	if crossedUsageAlert(previous.Tokens, next.Tokens, s.alertTokens) {
		auditFromContext(ctx, s.store, "ai.usage_alert.tokens", "usage", usageDay(day))
	}
}

func crossedUsageAlert(previous, next, threshold int64) bool {
	return threshold > 0 && previous < threshold && next >= threshold
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
	model = strings.TrimSpace(model)
	if model == "" {
		return status.Error(codes.InvalidArgument, "model is required")
	}
	if len(model) > maxAIModelLength {
		return status.Error(codes.InvalidArgument, "model is too long")
	}
	if len(s.allowedModels) == 0 {
		return nil
	}
	if _, ok := s.allowedModels[model]; ok {
		return nil
	}
	return status.Error(codes.InvalidArgument, "model is not allowed")
}

func validateAIChatRequest(req AIChatRequest) error {
	if len(req.Messages) == 0 {
		return status.Error(codes.InvalidArgument, "messages are required")
	}
	if len(req.Messages) > maxAIChatMessages {
		return status.Error(codes.InvalidArgument, "too many messages")
	}
	if err := validateAITemperature(req.Temperature); err != nil {
		return err
	}
	var total int
	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role != ai.RoleSystem && role != ai.RoleUser && role != ai.RoleAssistant {
			return status.Error(codes.InvalidArgument, "message role is invalid")
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			return status.Error(codes.InvalidArgument, "message content is required")
		}
		if len(msg.Content) > maxAIMessageContentBytes {
			return status.Error(codes.InvalidArgument, "message content is too large")
		}
		total += len(msg.Content)
		if total > maxAIChatContentBytes {
			return status.Error(codes.InvalidArgument, "chat content is too large")
		}
	}
	return nil
}

func validateAIVisionRequest(req AIVisionRequest) error {
	if strings.TrimSpace(req.SystemPrompt) == "" || strings.TrimSpace(req.UserText) == "" || strings.TrimSpace(req.ImageURL) == "" {
		return status.Error(codes.InvalidArgument, "system prompt, user text, and image url are required")
	}
	if len(req.SystemPrompt) > maxAIVisionPromptBytes || len(req.UserText) > maxAIVisionPromptBytes {
		return status.Error(codes.InvalidArgument, "vision prompt is too large")
	}
	if len(req.ImageURL) > maxAIVisionImageURLBytes {
		return status.Error(codes.InvalidArgument, "image url is too large")
	}
	if err := validateAITemperature(req.Temperature); err != nil {
		return err
	}
	if len(req.Schema) > 0 || strings.TrimSpace(req.SchemaName) != "" {
		if strings.TrimSpace(req.SchemaName) == "" || len(req.Schema) == 0 {
			return status.Error(codes.InvalidArgument, "schema name and schema are required together")
		}
		if len(req.SchemaName) > maxAIVisionSchemaNameSize {
			return status.Error(codes.InvalidArgument, "schema name is too long")
		}
		if len(req.Schema) > maxAIVisionSchemaBytes {
			return status.Error(codes.InvalidArgument, "schema is too large")
		}
		if !json.Valid(req.Schema) {
			return status.Error(codes.InvalidArgument, "schema must be valid JSON")
		}
	}
	return nil
}

func validateAITemperature(temperature float64) error {
	if math.IsNaN(temperature) || math.IsInf(temperature, 0) || temperature < 0 || temperature > 2 {
		return status.Error(codes.InvalidArgument, "temperature is invalid")
	}
	return nil
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
