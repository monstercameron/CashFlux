package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
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

	svc := NewAIService(store, AIServiceConfig{
		MasterKey: master,
		BaseURL:   upstream.URL,
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
