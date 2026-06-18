package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	invalid = valid
	invalid.GRPCReadLimitBytes = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative grpc read limit accepted")
	}
	invalid = valid
	invalid.BlobMaxBytes = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative blob max bytes accepted")
	}
	invalid = valid
	invalid.GRPCKeepaliveInterval = 30
	invalid.GRPCIdleTimeout = 30
	if err := invalid.Validate(); err == nil {
		t.Fatal("grpc keepalive equal to idle timeout accepted")
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

func TestGRPCBridgeEndpointMountedAndOriginChecked(t *testing.T) {
	cfg := Config{
		AuthMode:              "token",
		Token:                 "dev-token",
		AppOrigin:             "http://127.0.0.1:8080",
		GRPCReadLimitBytes:    1 << 20,
		GRPCKeepaliveInterval: 30,
		GRPCIdleTimeout:       90,
	}
	h := NewMux(cfg)

	req := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatal("/grpc was not mounted")
	}

	req = httptest.NewRequest(http.MethodGet, "/grpc", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("forbidden grpc origin status = %d, want 403", rr.Code)
	}
}

func TestGRPCTokenValidatorMatchesHTTPBearerUser(t *testing.T) {
	cfg := Config{AuthMode: "token", Token: "dev-token"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	want, ok := httpBearerUser(req, cfg)
	if !ok {
		t.Fatal("http bearer user missing")
	}
	got, err := grpcTokenValidator(cfg)(context.Background(), "dev-token")
	if err != nil {
		t.Fatalf("grpc token validator rejected token: %v", err)
	}
	if got != want {
		t.Fatalf("grpc user = %+v, want %+v", got, want)
	}
	if _, err := grpcTokenValidator(cfg)(context.Background(), "wrong"); err == nil {
		t.Fatal("grpc token validator accepted wrong token")
	}
}

func TestBlobEndpointsPutGetHead(t *testing.T) {
	store := openTestStore(t)
	data := []byte("receipt bytes")
	hash := blobHash(data)
	cfg := Config{
		AuthMode:     "token",
		Token:        "dev-token",
		DataDir:      t.TempDir(),
		BlobMaxBytes: 1024,
	}
	h := NewMux(cfg, store)

	req := httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Content-Type", "image/png")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put blob status = %d body %q", rr.Code, rr.Body.String())
	}
	var body BlobResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode blob response: %v", err)
	}
	if body.Hash != hash || body.Size != int64(len(data)) || body.Mime != "image/png" {
		t.Fatalf("blob response = %+v", body)
	}

	req = httptest.NewRequest(http.MethodHead, "/v1/blobs/"+hash, nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("head blob status = %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "image/png" || rr.Header().Get("ETag") != `"`+hash+`"` {
		t.Fatalf("head headers = content-type %q etag %q", rr.Header().Get("Content-Type"), rr.Header().Get("ETag"))
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("head body length = %d, want 0", rr.Body.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/blobs/"+hash, nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != string(data) {
		t.Fatalf("get blob status/body = %d/%q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("cache-control = %q", rr.Header().Get("Cache-Control"))
	}
}

func TestBlobEndpointsRejectBadAuthHashAndOversize(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", Token: "dev-token", DataDir: t.TempDir(), BlobMaxBytes: 4}
	h := NewMux(cfg, store)
	hash := blobHash([]byte("abc"))

	req := httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash, bytes.NewReader([]byte("abc")))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash, bytes.NewReader([]byte("wrong")))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash, bytes.NewReader([]byte("abd")))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("hash mismatch status = %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/blobs/not-a-hash", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad hash status = %d", rr.Code)
	}
}

func TestBlobEndpointCORS(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", AppOrigin: "http://127.0.0.1:8080"})
	req := httptest.NewRequest(http.MethodOptions, "/v1/blobs/"+blobHash([]byte("abc")), nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent || !strings.Contains(rr.Header().Get("Access-Control-Allow-Methods"), "PUT") {
		t.Fatalf("allowed blob cors status/methods = %d/%q", rr.Code, rr.Header().Get("Access-Control-Allow-Methods"))
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

func blobHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestAIChatEndpointAppliesConfiguredGuards(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{
		AuthMode:          "token",
		MasterKey:         "0123456789abcdef0123456789abcdef",
		Token:             "dev-token",
		AIAllowedModels:   []string{"gpt-4o-mini"},
		AIRequestMaxBytes: 64,
	}
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

	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", bytes.NewBufferString(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr := httptest.NewRecorder()
	NewMux(cfg, store).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("disallowed model status = %d body %q", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/ai/chat", bytes.NewBufferString(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"`+strings.Repeat("x", 200)+`"}]}`))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	NewMux(cfg, store).ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("oversized request status = %d body %q", rr.Code, rr.Body.String())
	}
}
