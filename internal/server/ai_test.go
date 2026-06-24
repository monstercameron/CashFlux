// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAIServiceChatUsesEncryptedKeyAndRecordsUsage(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-server-secret" {
			t.Fatalf("authorization = %q", got)
		}
		var body ai.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body.Model != "gpt-4o-mini" || len(body.Messages) != 1 || body.Messages[0].Content != "hello" {
			t.Fatalf("upstream body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi back"}}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
	}))
	defer upstream.Close()

	metrics := NewMetrics()
	svc := NewAIService(store, AIServiceConfig{
		MasterKey: master,
		BaseURL:   upstream.URL,
		Metrics:   metrics,
		Now:       func() time.Time { return time.Date(2026, time.June, 18, 18, 20, 0, 0, time.UTC) },
	})
	got, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:       "gpt-4o-mini",
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		Temperature: 0.2,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got.Content != "hi back" || got.Usage.TotalTokens != 5 {
		t.Fatalf("completion = %+v", got)
	}
	usage, ok, err := store.GetUsage("u1", time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC))
	if err != nil || !ok || usage.Requests != 1 || usage.Tokens != 5 {
		t.Fatalf("usage = %+v/%v/%v", usage, ok, err)
	}
	var metricsOut strings.Builder
	metrics.WritePrometheus(&metricsOut)
	if !strings.Contains(metricsOut.String(), "cashflux_ai_proxy_requests_total 1") {
		t.Fatalf("missing ai request metric in:\n%s", metricsOut.String())
	}
	if !strings.Contains(metricsOut.String(), "cashflux_ai_proxy_tokens_total 5") {
		t.Fatalf("missing ai token metric in:\n%s", metricsOut.String())
	}
}

func TestAIServiceVisionBuildsStructuredRequest(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if _, ok := body["response_format"]; !ok {
			t.Fatalf("missing response_format in %#v", body)
		}
		raw, _ := json.Marshal(body)
		if !strings.Contains(string(raw), "data:image/png;base64,AAAA") {
			t.Fatalf("missing image url in %s", raw)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"transactions\":[]}"}}],"usage":{"total_tokens":7}}`))
	}))
	defer upstream.Close()

	svc := NewAIService(store, AIServiceConfig{MasterKey: master, BaseURL: upstream.URL})
	got, err := svc.Vision(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIVisionRequest{
		Model:        "gpt-4o",
		SystemPrompt: "read receipts",
		UserText:     "extract",
		ImageURL:     "data:image/png;base64,AAAA",
		SchemaName:   "transactions",
		Schema:       json.RawMessage(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatalf("Vision: %v", err)
	}
	if got.Content != `{"transactions":[]}` || got.Usage.TotalTokens != 7 {
		t.Fatalf("completion = %+v", got)
	}
}

func TestAIServiceMissingKey(t *testing.T) {
	store := openTestStore(t)
	svc := NewAIService(store, AIServiceConfig{MasterKey: []byte("0123456789abcdef0123456789abcdef")})
	_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err == nil || !strings.Contains(err.Error(), "openai key is not configured") {
		t.Fatalf("missing key err = %v", err)
	}
}

func TestAIServiceRejectsDisallowedModel(t *testing.T) {
	store := openTestStore(t)
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:     []byte("0123456789abcdef0123456789abcdef"),
		AllowedModels: []string{"gpt-4o-mini"},
	})
	_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:    "gpt-4o",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err == nil || !strings.Contains(err.Error(), "model is not allowed") {
		t.Fatalf("disallowed model err = %v", err)
	}
}

func TestAIServiceRejectsMalformedChatBeforeKeyLoad(t *testing.T) {
	store := openTestStore(t)
	called := false
	svc := NewAIService(store, AIServiceConfig{
		MasterKey: []byte("0123456789abcdef0123456789abcdef"),
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}),
	})
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})

	for _, tc := range []struct {
		name string
		req  AIChatRequest
	}{
		{name: "missing messages", req: AIChatRequest{Model: "gpt-4o-mini"}},
		{name: "too many messages", req: AIChatRequest{Model: "gpt-4o-mini", Messages: repeatAIMessages(maxAIChatMessages + 1)}},
		{name: "invalid role", req: AIChatRequest{Model: "gpt-4o-mini", Messages: []ai.Message{{Role: "developer", Content: "hello"}}}},
		{name: "empty content", req: AIChatRequest{Model: "gpt-4o-mini", Messages: []ai.Message{{Role: ai.RoleUser, Content: " "}}}},
		{name: "content too large", req: AIChatRequest{Model: "gpt-4o-mini", Messages: []ai.Message{{Role: ai.RoleUser, Content: strings.Repeat("x", maxAIMessageContentBytes+1)}}}},
		{name: "temperature too high", req: AIChatRequest{Model: "gpt-4o-mini", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}}, Temperature: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Chat(ctx, tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("Chat err = %v code %v, want invalid argument", err, status.Code(err))
			}
		})
	}
	if called {
		t.Fatal("malformed chat request reached upstream")
	}
}

func TestAIServiceRejectsOversizedRequestBeforeKeyLoad(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	svc := NewAIService(store, AIServiceConfig{MasterKey: master, RequestMaxBytes: 64})
	_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: strings.Repeat("x", 200)}},
	})
	if err == nil || !strings.Contains(err.Error(), "ai request is too large") {
		t.Fatalf("oversized request err = %v", err)
	}
}

func TestAIServiceRejectsMalformedVisionBeforeKeyLoad(t *testing.T) {
	store := openTestStore(t)
	called := false
	svc := NewAIService(store, AIServiceConfig{
		MasterKey: []byte("0123456789abcdef0123456789abcdef"),
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}),
	})
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	valid := AIVisionRequest{
		Model:        "gpt-4o-mini",
		SystemPrompt: "read receipts",
		UserText:     "extract",
		ImageURL:     "data:image/png;base64,AAAA",
	}

	for _, tc := range []struct {
		name string
		req  AIVisionRequest
	}{
		{name: "missing prompt", req: func() AIVisionRequest { r := valid; r.SystemPrompt = ""; return r }()},
		{name: "image too large", req: func() AIVisionRequest {
			r := valid
			r.ImageURL = strings.Repeat("x", maxAIVisionImageURLBytes+1)
			return r
		}()},
		{name: "bad temperature", req: func() AIVisionRequest { r := valid; r.Temperature = -0.1; return r }()},
		{name: "schema without name", req: func() AIVisionRequest { r := valid; r.Schema = json.RawMessage(`{"type":"object"}`); return r }()},
		{name: "malformed schema", req: func() AIVisionRequest {
			r := valid
			r.SchemaName = "transactions"
			r.Schema = json.RawMessage(`{"type":`)
			return r
		}()},
		{name: "schema too large", req: func() AIVisionRequest {
			r := valid
			r.SchemaName = "transactions"
			r.Schema = json.RawMessage(`{"x":"` + strings.Repeat("x", maxAIVisionSchemaBytes) + `"}`)
			return r
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Vision(ctx, tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("Vision err = %v code %v, want invalid argument", err, status.Code(err))
			}
		})
	}
	if called {
		t.Fatal("malformed vision request reached upstream")
	}
}

func TestAIServiceEnforcesDailyUsageLimits(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	day := time.Date(2026, time.June, 18, 18, 30, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	if _, err := store.AddUsage("u1", day, 2, 99); err != nil {
		t.Fatalf("AddUsage: %v", err)
	}
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:      master,
		RequestsPerDay: 2,
		TokensPerDay:   100,
		Now:            func() time.Time { return day },
	})
	_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err == nil || !strings.Contains(err.Error(), "daily ai request limit reached") {
		t.Fatalf("request limit err = %v", err)
	}

	svc = NewAIService(store, AIServiceConfig{
		MasterKey:      master,
		RequestsPerDay: 3,
		TokensPerDay:   99,
		Now:            func() time.Time { return day },
	})
	_, err = svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err == nil || !strings.Contains(err.Error(), "daily ai token limit reached") {
		t.Fatalf("token limit err = %v", err)
	}
}

func TestAIServiceBlocksUserBeforeKeyLoadOrUpstream(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u-blocked", Provider: "token", Subject: "u-blocked", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u-blocked", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	called := false
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:      master,
		BlockedUserIDs: []string{"u-blocked"},
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}),
	})
	_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u-blocked"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "disabled for this user") {
		t.Fatalf("blocked err = %v code %v", err, status.Code(err))
	}
	if called {
		t.Fatal("blocked user reached upstream")
	}
}

func TestAIServiceAuditsUsageAlertsWhenThresholdsCross(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	day := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u-alert", Provider: "token", Subject: "u-alert", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u-alert", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	if _, err := store.AddUsage("u-alert", day, 1, 9); err != nil {
		t.Fatalf("AddUsage: %v", err)
	}
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:     master,
		AlertRequests: 2,
		AlertTokens:   10,
		Now:           func() time.Time { return day },
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}],"usage":{"total_tokens":2}}`)),
			}, nil
		}),
	})
	if _, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u-alert"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	events, err := store.ListAuditEvents(0, 10)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	var sawRequests, sawTokens bool
	for _, event := range events {
		if event.Action == "ai.usage_alert.requests" && event.TargetID == "2026-06-18" {
			sawRequests = true
		}
		if event.Action == "ai.usage_alert.tokens" && event.TargetID == "2026-06-18" {
			sawTokens = true
		}
	}
	if !sawRequests || !sawTokens {
		t.Fatalf("usage alert events requests=%v tokens=%v events=%+v", sawRequests, sawTokens, events)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func repeatAIMessages(n int) []ai.Message {
	out := make([]ai.Message, n)
	for i := range out {
		out[i] = ai.Message{Role: ai.RoleUser, Content: "hello"}
	}
	return out
}

type cancelAwareClient struct {
	started chan struct{}
	sawDone chan struct{}
}

func (c cancelAwareClient) Do(req *http.Request) (*http.Response, error) {
	close(c.started)
	<-req.Context().Done()
	close(c.sawDone)
	return nil, req.Context().Err()
}

func TestAIServiceCancellationPropagatesToUpstream(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	sawDone := make(chan struct{})
	started := make(chan struct{})
	svc := NewAIService(store, AIServiceConfig{MasterKey: master, Client: cancelAwareClient{started: started, sawDone: sawDone}})
	ctx, cancel := context.WithCancel(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}))
	errc := make(chan error, 1)
	go func() {
		_, err := svc.Chat(ctx, AIChatRequest{
			Model:    "gpt-4o-mini",
			Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		})
		errc <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not start")
	}
	cancel()
	select {
	case <-sawDone:
	case <-time.After(time.Second):
		t.Fatal("upstream request context was not canceled")
	}
	select {
	case err := <-errc:
		if status.Code(err) != codes.Canceled {
			t.Fatalf("cancel error = %v code %v", err, status.Code(err))
		}
	case <-time.After(time.Second):
		t.Fatal("Chat did not return after cancellation")
	}
}

type sequenceAIClient struct {
	responses []*http.Response
	errors    []error
	calls     int
}

func (c *sequenceAIClient) Do(req *http.Request) (*http.Response, error) {
	c.calls++
	idx := c.calls - 1
	if idx < len(c.errors) && c.errors[idx] != nil {
		return nil, c.errors[idx]
	}
	if idx < len(c.responses) && c.responses[idx] != nil {
		return c.responses[idx], nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"retry ok"}}],"usage":{"total_tokens":7}}`)),
	}, nil
}

func TestAIServiceRetriesTransientUpstreamStatus(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	client := &sequenceAIClient{responses: []*http.Response{
		{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(`temporary`))},
		{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"retry ok"}}],"usage":{"total_tokens":7}}`))},
	}}
	svc := NewAIService(store, AIServiceConfig{MasterKey: master, Client: client, UpstreamRetries: 1})
	got, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got.Content != "retry ok" || got.Usage.TotalTokens != 7 || client.calls != 2 {
		t.Fatalf("retry response/calls = %+v/%d", got, client.calls)
	}
}

func TestAIServiceOpensCircuitAfterConsecutiveUpstreamFailures(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	now := time.Date(2026, time.June, 19, 20, 0, 0, 0, time.UTC)
	client := &sequenceAIClient{errors: []error{errors.New("dial failed"), errors.New("dial failed"), errors.New("dial failed")}}
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:       master,
		Client:          client,
		UpstreamRetries: 0,
		Now:             func() time.Time { return now },
	})
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	for i := 0; i < aiCircuitFailureThreshold; i++ {
		_, err := svc.Chat(ctx, AIChatRequest{
			Model:    "gpt-4o-mini",
			Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		})
		if status.Code(err) != codes.Unavailable || !strings.Contains(err.Error(), "openai request failed") {
			t.Fatalf("failure %d err = %v code %v", i, err, status.Code(err))
		}
	}
	if client.calls != aiCircuitFailureThreshold {
		t.Fatalf("client calls after failures = %d", client.calls)
	}
	_, err := svc.Chat(ctx, AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if status.Code(err) != codes.Unavailable || !strings.Contains(err.Error(), "circuit is open") {
		t.Fatalf("open circuit err = %v code %v", err, status.Code(err))
	}
	if client.calls != aiCircuitFailureThreshold {
		t.Fatalf("open circuit made upstream call: %d", client.calls)
	}
}

func TestAIServiceCircuitResetsAfterCooldownAndSuccess(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	now := time.Date(2026, time.June, 19, 20, 5, 0, 0, time.UTC)
	client := &sequenceAIClient{responses: []*http.Response{
		{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(`temporary`))},
		{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(`temporary`))},
		{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(`temporary`))},
		{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"reset ok"}}],"usage":{"total_tokens":3}}`))},
	}}
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:       master,
		Client:          client,
		UpstreamRetries: 0,
		Now:             func() time.Time { return now },
	})
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	for i := 0; i < aiCircuitFailureThreshold; i++ {
		if _, err := svc.Chat(ctx, AIChatRequest{Model: "gpt-4o-mini", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}}}); status.Code(err) != codes.Unavailable {
			t.Fatalf("failure %d did not return unavailable: %v", i, err)
		}
	}
	now = now.Add(aiCircuitCooldown + time.Second)
	got, err := svc.Chat(ctx, AIChatRequest{Model: "gpt-4o-mini", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatalf("Chat after cooldown: %v", err)
	}
	if got.Content != "reset ok" || client.calls != aiCircuitFailureThreshold+1 {
		t.Fatalf("after cooldown response/calls = %+v/%d", got, client.calls)
	}
	_, err = svc.Chat(ctx, AIChatRequest{Model: "gpt-4o-mini", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatalf("Chat after reset: %v", err)
	}
	if client.calls != aiCircuitFailureThreshold+2 {
		t.Fatalf("circuit did not stay reset, calls = %d", client.calls)
	}
}

type blockingAIClient struct {
	started chan struct{}
}

func (c blockingAIClient) Do(req *http.Request) (*http.Response, error) {
	close(c.started)
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestAIServiceUpstreamTimeoutMapsDeadlineExceeded(t *testing.T) {
	store := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := store.UpsertUser(User{ID: "u1", Provider: "token", Subject: "u1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey("u1", "openai", "sk-server-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	started := make(chan struct{})
	svc := NewAIService(store, AIServiceConfig{
		MasterKey:       master,
		Client:          blockingAIClient{started: started},
		UpstreamTimeout: 10 * time.Millisecond,
	})
	_, err := svc.Chat(ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}), AIChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err == nil || status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("timeout err = %v code %v", err, status.Code(err))
	}
	select {
	case <-started:
	default:
		t.Fatal("upstream client was not called")
	}
}

func TestRetryBackoffIsBoundedJitteredExponential(t *testing.T) {
	for attempt, bounds := range []struct {
		min time.Duration
		max time.Duration
	}{
		{100 * time.Millisecond, 150 * time.Millisecond},
		{200 * time.Millisecond, 300 * time.Millisecond},
		{400 * time.Millisecond, 600 * time.Millisecond},
	} {
		got := retryBackoff(attempt)
		if got < bounds.min || got >= bounds.max {
			t.Fatalf("attempt %d backoff = %s, want [%s,%s)", attempt, got, bounds.min, bounds.max)
		}
	}
}
