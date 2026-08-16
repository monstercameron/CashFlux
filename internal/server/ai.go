// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
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

var defaultAIModels = []string{"gpt-5.4-mini", "gpt-5.5", "o4-mini"}

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
	maxAIChatTools            = 64
	maxAIToolNameLength       = 128
	maxAIToolSchemaBytes      = 32 << 10
	aiCircuitFailureThreshold = 3
	aiCircuitCooldown         = 30 * time.Second
	// defaultAIMaxOutputTokens gives a reasoning model enough room to think AND
	// write an answer. Too low and it spends the lot reasoning and returns nothing;
	// unbounded and one question can eat a day's tokens.
	defaultAIMaxOutputTokens = 8192
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
	maxOutputTokens int
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
	// MaxOutputTokens bounds one completion's output. It defaults to
	// defaultAIMaxOutputTokens rather than to "unlimited": a reasoning model with no
	// ceiling can burn a household's whole daily allowance on a single answer.
	MaxOutputTokens int
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
	// Tools are the functions the caller will run locally on the model's behalf.
	// A household on the server key gets the same tool-using assistant as one on a
	// direct key; without these the proxy could only ever answer from context.
	Tools []ai.Tool `json:"tools,omitempty"`
	// ReasoningEffort is the caller's thinking level for reasoning models.
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
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
	// ToolCalls are the functions the model asked the caller to run. Empty on a
	// plain answer.
	ToolCalls []ai.ToolCall `json:"toolCalls,omitempty"`
	// FinishReason is why the model stopped ("stop", "length", "content_filter",
	// "tool_calls"). It travels with every completion so a short or empty answer can
	// say why it is short rather than looking like a failure with no explanation.
	FinishReason string `json:"finishReason,omitempty"`
	// Reasoning carries the Responses API's reasoning items back to the caller,
	// which MUST echo them alongside the tool call on the next turn or the API
	// rejects that turn.
	Reasoning []json.RawMessage `json:"reasoning,omitempty"`
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
	maxOutputTokens := cfg.MaxOutputTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultAIMaxOutputTokens
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
		maxOutputTokens: maxOutputTokens,
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
	up, err := s.buildChatUpstream(req)
	if err != nil {
		return AICompletion{}, err
	}
	return s.complete(ctx, up)
}

// buildChatUpstream picks the endpoint the request actually needs and prepares the
// call for it. Reasoning models and tool-calling turns go to the Responses API:
// /chat/completions rejects reasoning_effort alongside function tools, and — the
// failure that made this necessary — it reports a reasoning model that spent its
// whole allowance thinking as a *successful* reply with empty content, which is
// how a billed call reached the screen as a blank bubble. The Responses API reports
// that same case as incomplete with a reason. Everything else keeps the simpler
// chat-completions path, now with an output budget so it is bounded too.
func (s *AIService) buildChatUpstream(req AIChatRequest) (aiUpstream, error) {
	if ai.ReasoningModel(req.Model) || len(req.Tools) > 0 {
		body, err := ai.BuildBudgetedResponsesToolRequest(req.Model, req.Messages, req.Temperature, req.ReasoningEffort, req.Tools, s.maxOutputTokens)
		if err != nil {
			return aiUpstream{}, status.Errorf(codes.InvalidArgument, "build chat request: %v", err)
		}
		return aiUpstream{path: "/responses", body: body, parse: parseResponsesUpstream}, nil
	}
	body, err := ai.BuildRequestWithOptions(req.Model, req.Messages, ai.ChatOptions{
		Temperature:         req.Temperature,
		MaxCompletionTokens: s.maxOutputTokens,
		ReasoningEffort:     req.ReasoningEffort,
	})
	if err != nil {
		return aiUpstream{}, status.Errorf(codes.InvalidArgument, "build chat request: %v", err)
	}
	return aiUpstream{path: "/chat/completions", body: body, parse: parseChatUpstream}, nil
}

// aiUpstream is one prepared upstream call: which OpenAI endpoint it targets, the
// body to post, and how to read that endpoint's reply back into a completion. The
// two endpoints answer in different shapes, so the reader travels with the request
// rather than being guessed at the other end.
type aiUpstream struct {
	path  string
	body  []byte
	parse func([]byte) (AICompletion, error)
	// stream is true when the body asked for a server-sent-event response, so the
	// reply is read incrementally rather than whole.
	stream bool
}

// parseChatUpstream reads a /chat/completions reply, refusing an answer that is
// empty for anything other than a tool call — an empty completion is always caused
// by something (a length cap, a filter), and the finish reason names it. The usage
// rides back even on the refusal, because those tokens were still billed.
func parseChatUpstream(data []byte) (AICompletion, error) {
	msg, finish, usage, err := ai.ParseChat(data)
	if err != nil {
		return AICompletion{Usage: usage}, err
	}
	if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
		return AICompletion{Usage: usage, FinishReason: finish}, errors.New(ai.EmptyReplyMessage(finish))
	}
	return AICompletion{Content: msg.Content, Usage: usage, ToolCalls: msg.ToolCalls, FinishReason: finish}, nil
}

// parseResponsesUpstream reads a /responses reply. ParseResponsesChat already
// refuses an empty reply, explains why it was empty, and reports the tokens the
// attempt cost.
func parseResponsesUpstream(data []byte) (AICompletion, error) {
	msg, usage, err := ai.ParseResponsesChat(data)
	if err != nil {
		return AICompletion{Usage: usage}, err
	}
	finish := ai.FinishStop
	if len(msg.ToolCalls) > 0 {
		finish = ai.FinishToolCalls
	}
	return AICompletion{
		Content: msg.Content, Usage: usage, ToolCalls: msg.ToolCalls,
		FinishReason: finish, Reasoning: msg.ReasoningRaw,
	}, nil
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
	return s.complete(ctx, aiUpstream{path: "/chat/completions", body: body, parse: parseChatUpstream})
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

// ChatStream runs a chat request with streaming turned on, handing each fragment
// of the answer to onDelta as it arrives and returning the whole completion at the
// end. A household on the shared key sees the answer being written exactly as one
// on a direct key does — the parity that C551 started.
func (s *AIService) ChatStream(ctx context.Context, req AIChatRequest, onDelta func(string)) (AICompletion, error) {
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
	up, err := s.buildChatUpstream(req)
	if err != nil {
		return AICompletion{}, err
	}
	// Only the Responses path streams. The chat-completions path is used for
	// ordinary models and for vision, where the reply is one object; streaming it
	// would add a second parser for no gain.
	if up.path != "/responses" {
		return s.complete(ctx, up)
	}
	body, err := ai.BuildStreamingResponsesToolRequest(req.Model, req.Messages, req.Temperature, req.ReasoningEffort, req.Tools, s.maxOutputTokens)
	if err != nil {
		return AICompletion{}, status.Errorf(codes.InvalidArgument, "build chat request: %v", err)
	}
	up.body, up.stream = body, true
	return s.completeStreaming(ctx, up, onDelta)
}

func (s *AIService) complete(ctx context.Context, up aiUpstream) (AICompletion, error) {
	return s.completeStreaming(ctx, up, nil)
}

// completeStreaming runs one upstream call. onDelta is nil for a whole-answer
// request; when it is set and the request asked for a stream, each text fragment
// is forwarded as it arrives and the authoritative completion still comes from the
// stream's completed event.
func (s *AIService) completeStreaming(ctx context.Context, up aiUpstream, onDelta func(string)) (AICompletion, error) {
	body := up.body
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
	// Capture usage BEFORE reserving so the alert-threshold crossing below compares
	// pre-request against post-request counts.
	previous, _, err := s.store.GetUsage(user.ID, day)
	if err != nil {
		return AICompletion{}, fmt.Errorf("server ai: get usage before reserve: %w", err)
	}
	// Atomically reserve one request against the daily cap. This replaces a
	// read-then-later-increment that let concurrent requests slip past the limit.
	allowed, limit, err := s.store.ReserveAIRequest(user.ID, day, s.requestsPerDay, s.tokensPerDay)
	if err != nil {
		return AICompletion{}, fmt.Errorf("server ai: reserve request: %w", err)
	}
	if !allowed {
		if limit == "tokens" {
			return AICompletion{}, status.Error(codes.ResourceExhausted, "daily ai token limit reached")
		}
		return AICompletion{}, status.Error(codes.ResourceExhausted, "daily ai request limit reached")
	}
	// The slot is reserved; release it if the request never completes, so only
	// requests that actually run keep counting against the cap.
	release := func() { _ = s.store.ReleaseAIRequest(user.ID, day) }
	upstreamCtx := ctx
	cancel := func() {}
	if s.upstreamTimeout > 0 {
		upstreamCtx, cancel = context.WithTimeout(ctx, s.upstreamTimeout)
	}
	defer cancel()
	resp, err := s.doUpstream(upstreamCtx, up.path, body, key)
	if err != nil {
		release()
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
	if up.stream && resp.StatusCode < 400 {
		s.recordAIUpstreamSuccess()
		completion, streamErr := readAIStream(resp.Body, onDelta)
		next, err := s.store.AddUsage(user.ID, day, 0, int64(completion.Usage.TotalTokens))
		if err != nil {
			return AICompletion{}, fmt.Errorf("server ai: add usage: %w", err)
		}
		s.auditAIUsageAlerts(ctx, day, previous, next)
		s.metrics.ObserveAIProxy(int64(completion.Usage.TotalTokens))
		if streamErr != nil {
			// A stream that dropped part-way never produced an answer, so it does
			// not keep its slot against the daily cap — the same rule the
			// whole-answer path applies to its own failures. Without this, a run of
			// flaky-network drops burns a household's day of requests while
			// answering nothing. The TOKENS above are still recorded: those were
			// billed whether or not an answer arrived.
			release()
			return AICompletion{}, status.Error(codes.Internal, streamErr.Error())
		}
		return completion, nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		release()
		return AICompletion{}, status.Errorf(codes.Unavailable, "read openai response: %v", err)
	}
	if resp.StatusCode >= 400 {
		release()
		if resp.StatusCode >= 500 {
			s.recordAIUpstreamFailure(day)
		} else {
			s.recordAIUpstreamSuccess()
		}
		return AICompletion{}, status.Error(openAICode(resp.StatusCode), ai.ErrorMessage(resp.StatusCode, data))
	}
	s.recordAIUpstreamSuccess()
	// A reply that parsed to nothing was still billed upstream, so its tokens are
	// recorded before the error is returned — the user pays for it either way, and a
	// usage line that quietly omits failed calls understates the real spend.
	completion, parseErr := up.parse(data)
	usage := completion.Usage
	next, err := s.store.AddUsage(user.ID, day, 0, int64(usage.TotalTokens))
	if err != nil {
		return AICompletion{}, fmt.Errorf("server ai: add usage: %w", err)
	}
	s.auditAIUsageAlerts(ctx, day, previous, next)
	s.metrics.ObserveAIProxy(int64(usage.TotalTokens))
	if parseErr != nil {
		return AICompletion{}, status.Error(codes.Internal, parseErr.Error())
	}
	return completion, nil
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

func (s *AIService) doUpstream(ctx context.Context, path string, body []byte, key string) (*http.Response, error) {
	attempts := s.upstreamRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(body))
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
	if len(req.Tools) > maxAIChatTools {
		return status.Error(codes.InvalidArgument, "too many tools")
	}
	for _, tool := range req.Tools {
		if strings.TrimSpace(tool.Function.Name) == "" {
			return status.Error(codes.InvalidArgument, "tool name is required")
		}
		if len(tool.Function.Name) > maxAIToolNameLength {
			return status.Error(codes.InvalidArgument, "tool name is too long")
		}
		if len(tool.Function.Parameters) > maxAIToolSchemaBytes {
			return status.Error(codes.InvalidArgument, "tool schema is too large")
		}
		if len(tool.Function.Parameters) > 0 && !json.Valid(tool.Function.Parameters) {
			return status.Error(codes.InvalidArgument, "tool schema must be valid JSON")
		}
	}
	var total int
	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role != ai.RoleSystem && role != ai.RoleUser && role != ai.RoleAssistant && role != ai.RoleTool {
			return status.Error(codes.InvalidArgument, "message role is invalid")
		}
		if role == ai.RoleTool && strings.TrimSpace(msg.ToolCallID) == "" {
			return status.Error(codes.InvalidArgument, "tool result is missing its call id")
		}
		// An assistant turn that only asks to run tools carries no text, and a tool
		// result may legitimately be an empty string — so content is required only
		// where nothing else in the message says what it is.
		if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 && role != ai.RoleTool {
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

// aiStreamReadLimit bounds how much of a streamed reply is read, mirroring the
// whole-answer path's limit. A stream is still untrusted input from a third party.
const aiStreamReadLimit = 8 << 20

// readAIStream consumes a server-sent-event body, forwarding each text fragment to
// onDelta and returning the completion built from the stream's completed event.
//
// The completion comes from the completed event rather than from the concatenated
// fragments on purpose: the fragments are the answer's text and nothing else, while
// the completed event carries the tool calls, the token counts and the finish
// reason. Reassembling those from deltas would be a second, subtly different parser
// for the same reply.
func readAIStream(body io.Reader, onDelta func(string)) (AICompletion, error) {
	var (
		dec       ai.StreamDecoder
		final     []byte
		streamErr string
	)
	buf := make([]byte, 8<<10)
	limited := io.LimitReader(body, aiStreamReadLimit)
	for {
		n, err := limited.Read(buf)
		if n > 0 {
			for _, ev := range dec.Feed(buf[:n]) {
				switch {
				case ev.Err != "":
					streamErr = ev.Err
				case ev.IsFinal():
					final = append([]byte(nil), ev.Response...)
				case ev.IsDelta():
					if onDelta != nil {
						onDelta(ev.TextDelta)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return AICompletion{}, fmt.Errorf("server ai: read stream: %w", err)
		}
	}
	if streamErr != "" {
		return AICompletion{}, errors.New(streamErr)
	}
	if len(final) == 0 {
		// The visible text may look finished while the tool calls and token counts
		// never arrived. Acting on that half-state is worse than saying so.
		return AICompletion{}, errors.New("The answer stopped part-way through. Try asking again.")
	}
	return parseResponsesUpstream(final)
}
