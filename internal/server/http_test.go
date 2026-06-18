package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{Addr: ":0", DataDir: t.TempDir(), AuthMode: "token"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	invalid := valid
	invalid.AuthMode = "magic"
	if err := invalid.Validate(); err == nil {
		t.Fatal("unsupported auth mode accepted")
	}
	invalid = valid
	invalid.MasterKey = "short"
	if err := invalid.Validate(); err == nil {
		t.Fatal("short master key accepted")
	}
}

func TestHealthReadyAndVersionEndpoints(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", Billing: false})
	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want 204", path, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("version status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	var body VersionResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	if body.APIVersion != APIVersion || body.MinClientAPIVersion != MinClientAPIVersion {
		t.Fatalf("version body = %+v", body)
	}
	if body.AuthMode != "token" || body.BillingEnabled {
		t.Fatalf("mode flags = %+v", body)
	}
}

func TestAIKeyEndpointStoresEncryptedKey(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", MasterKey: "0123456789abcdef0123456789abcdef", Token: "dev-token"}
	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/key", bytes.NewBufferString(`{"provider":"openai","key":"sk-secret"}`))
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("store key status = %d body %q", rr.Code, rr.Body.String())
	}
	var body AIKeyResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode key response: %v", err)
	}
	if !body.Stored || body.Provider != "openai" {
		t.Fatalf("key response = %+v", body)
	}
	user, ok := httpBearerUser(req, cfg)
	if !ok {
		t.Fatal("bearer user missing")
	}
	got, ok, err := store.GetAIKey(user.ID, "openai", []byte(cfg.MasterKey))
	if err != nil || !ok || got != "sk-secret" {
		t.Fatalf("stored key = %q/%v/%v", got, ok, err)
	}
}

func TestAIKeyEndpointRejectsMissingAuthAndMaster(t *testing.T) {
	store := openTestStore(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/key", bytes.NewBufferString(`{"provider":"openai","key":"sk-secret"}`))
	rr := httptest.NewRecorder()
	NewMux(Config{AuthMode: "token", MasterKey: "0123456789abcdef0123456789abcdef", Token: "dev-token"}, store).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/ai/key", bytes.NewBufferString(`{"provider":"openai","key":"sk-secret"}`))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	NewMux(Config{AuthMode: "token", Token: "dev-token"}, store).ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing master status = %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/ai/key", bytes.NewBufferString(`{"provider":"openai","key":"sk-secret"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	rr = httptest.NewRecorder()
	NewMux(Config{AuthMode: "token", MasterKey: "0123456789abcdef0123456789abcdef", Token: "dev-token"}, store).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d", rr.Code)
	}
}

func TestAIKeyEndpointCORS(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", AppOrigin: "http://127.0.0.1:8080"})
	req := httptest.NewRequest(http.MethodOptions, "/v1/ai/key", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent || rr.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:8080" {
		t.Fatalf("allowed cors status/header = %d/%q", rr.Code, rr.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest(http.MethodOptions, "/v1/ai/key", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("forbidden cors status = %d", rr.Code)
	}
}

func TestAIChatEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-secret" {
			t.Fatalf("authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"server says hi"}}],"usage":{"total_tokens":11}}`))
	}))
	defer upstream.Close()

	store := openTestStore(t)
	cfg := Config{AuthMode: "token", MasterKey: "0123456789abcdef0123456789abcdef", Token: "dev-token", OpenAIBaseURL: upstream.URL}
	reqForUser := httptest.NewRequest(http.MethodPost, "/v1/ai/key", bytes.NewBufferString(`{}`))
	reqForUser.Header.Set("Authorization", "Bearer dev-token")
	user, ok := httpBearerUser(reqForUser, cfg)
	if !ok {
		t.Fatal("bearer user missing")
	}
	if err := store.UpsertUser(User{ID: user.ID, Provider: "token", Subject: user.ID}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutAIKey(user.ID, "openai", "sk-secret", []byte(cfg.MasterKey)); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}

	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", bytes.NewBufferString(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("chat status = %d body %q", rr.Code, rr.Body.String())
	}
	var body AICompletion
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode chat response: %v", err)
	}
	if body.Content != "server says hi" || body.Usage.TotalTokens != 11 {
		t.Fatalf("chat response = %+v", body)
	}
}
