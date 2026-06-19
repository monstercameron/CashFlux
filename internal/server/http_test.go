package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
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
	invalid.AppOrigin = "http://cashflux.example"
	if err := invalid.Validate(); err == nil {
		t.Fatal("non-tls app origin accepted")
	}
	invalid = valid
	invalid.AppOrigin = "*"
	if err := invalid.Validate(); err == nil {
		t.Fatal("wildcard app origin accepted")
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
	invalid.GRPCMaxStreamsPerUser = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative grpc stream limit accepted")
	}
	invalid = valid
	invalid.BlobMaxBytes = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative blob max bytes accepted")
	}
	invalid = valid
	invalid.BlobIOTimeout = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative blob io timeout accepted")
	}
	invalid = valid
	invalid.StorageMaxBytes = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative storage max bytes accepted")
	}
	invalid = valid
	invalid.AIUpstreamRetries = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative ai upstream retries accepted")
	}
	invalid = valid
	invalid.AIUpstreamTimeout = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative ai upstream timeout accepted")
	}
	invalid = valid
	invalid.HTTPReadTimeout = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative http read timeout accepted")
	}
	invalid = valid
	invalid.HTTPMaxInFlight = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative http max in-flight accepted")
	}
	invalid = valid
	invalid.HTTPRateLimitPerMinute = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative http rate limit accepted")
	}
	invalid = valid
	invalid.HTTPUserRateLimitPerMinute = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative http user rate limit accepted")
	}
	invalid = valid
	invalid.AuthRateLimitPerMinute = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative auth rate limit accepted")
	}
	invalid = valid
	invalid.AuditRetentionDays = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative audit retention accepted")
	}
	invalid = valid
	invalid.SnapshotHistoryRetentionDays = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative snapshot retention accepted")
	}
	invalid = valid
	invalid.BackupRetentionDays = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative backup retention accepted")
	}
	invalid = valid
	invalid.TokenSHA256 = "not-a-digest"
	if err := invalid.Validate(); err == nil {
		t.Fatal("bad token sha256 accepted")
	}
	invalid = valid
	invalid.GRPCKeepaliveInterval = 30
	invalid.GRPCIdleTimeout = 30
	if err := invalid.Validate(); err == nil {
		t.Fatal("grpc keepalive equal to idle timeout accepted")
	}
	invalid = valid
	invalid.AuthMode = "oauth"
	if err := invalid.Validate(); err == nil {
		t.Fatal("oauth mode without providers accepted")
	}
	invalid = valid
	invalid.OAuthProviders = map[string]OAuthProviderConfig{"github": {ClientID: "id"}}
	if err := invalid.Validate(); err == nil {
		t.Fatal("partial oauth provider accepted")
	}
	invalid = valid
	invalid.OAuthProviders = map[string]OAuthProviderConfig{"github": {
		ClientID: "id", ClientSecret: "secret", RedirectURL: "http://127.0.0.1/oauth/callback",
	}}
	if err := invalid.Validate(); err == nil {
		t.Fatal("bad oauth redirect accepted")
	}
	invalid = valid
	invalid.OAuthProviders = map[string]OAuthProviderConfig{"github": {
		ClientID: "id", ClientSecret: "secret", RedirectURL: "http://cashflux.example/v1/auth/github/callback",
	}}
	if err := invalid.Validate(); err == nil {
		t.Fatal("non-tls oauth redirect accepted")
	}
	valid.AuthMode = "oauth"
	valid.OAuthProviders = map[string]OAuthProviderConfig{"google": {ClientID: "id", ClientSecret: "secret", RedirectURL: "http://127.0.0.1/v1/auth/google/callback"}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("oauth config rejected: %v", err)
	}
	valid.AppOrigin = "https://cashflux.example"
	if err := valid.Validate(); err != nil {
		t.Fatalf("https app origin rejected: %v", err)
	}
}

func TestFromEnvLoadsHTTPLimits(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_HTTP_READ_TIMEOUT", "5s")
	t.Setenv("CASHFLUX_SERVER_HTTP_WRITE_TIMEOUT", "7s")
	t.Setenv("CASHFLUX_SERVER_HTTP_MAX_IN_FLIGHT", "17")
	t.Setenv("CASHFLUX_SERVER_HTTP_RATE_LIMIT_PER_MINUTE", "19")
	t.Setenv("CASHFLUX_SERVER_HTTP_USER_RATE_LIMIT_PER_MINUTE", "23")
	t.Setenv("CASHFLUX_SERVER_AUTH_RATE_LIMIT_PER_MINUTE", "29")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.HTTPReadTimeout != 5*time.Second || cfg.HTTPWriteTimeout != 7*time.Second ||
		cfg.HTTPMaxInFlight != 17 || cfg.HTTPRateLimitPerMinute != 19 || cfg.HTTPUserRateLimitPerMinute != 23 ||
		cfg.AuthRateLimitPerMinute != 29 {
		t.Fatalf("http limits = read %s write %s in-flight %d rate %d user rate %d auth rate %d",
			cfg.HTTPReadTimeout, cfg.HTTPWriteTimeout, cfg.HTTPMaxInFlight, cfg.HTTPRateLimitPerMinute,
			cfg.HTTPUserRateLimitPerMinute, cfg.AuthRateLimitPerMinute)
	}
}

func TestFromEnvLoadsAIProxyFlag(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_AI_PROXY_ENABLED", "false")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if !cfg.AIProxyDisabled {
		t.Fatal("AIProxyDisabled = false, want true")
	}
}

func TestFromEnvLoadsAIBlockedUsers(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_AI_BLOCKED_USER_IDS", "u1, github:blocked , , token:abc")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if len(cfg.AIBlockedUserIDs) != 3 || cfg.AIBlockedUserIDs[0] != "u1" ||
		cfg.AIBlockedUserIDs[1] != "github:blocked" || cfg.AIBlockedUserIDs[2] != "token:abc" {
		t.Fatalf("AIBlockedUserIDs = %+v", cfg.AIBlockedUserIDs)
	}
}

func TestFromEnvLoadsGRPCStreamLimit(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_GRPC_MAX_STREAMS_PER_USER", "3")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.GRPCMaxStreamsPerUser != 3 {
		t.Fatalf("GRPCMaxStreamsPerUser = %d, want 3", cfg.GRPCMaxStreamsPerUser)
	}
}

func TestFromEnvLoadsStorageQuota(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_STORAGE_MAX_BYTES", "12345")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.StorageMaxBytes != 12345 {
		t.Fatalf("StorageMaxBytes = %d, want 12345", cfg.StorageMaxBytes)
	}
}

func TestFromEnvLoadsRetentionWindows(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_AUDIT_RETENTION_DAYS", "90")
	t.Setenv("CASHFLUX_SERVER_SNAPSHOT_HISTORY_RETENTION_DAYS", "45")
	t.Setenv("CASHFLUX_SERVER_BACKUP_RETENTION_DAYS", "14")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.AuditRetentionDays != 90 || cfg.SnapshotHistoryRetentionDays != 45 || cfg.BackupRetentionDays != 14 {
		t.Fatalf("retention windows = audit %d snapshot %d backup %d", cfg.AuditRetentionDays, cfg.SnapshotHistoryRetentionDays, cfg.BackupRetentionDays)
	}
}

func TestFromEnvLoadsOAuthProviders(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_AUTH_MODE", "oauth")
	t.Setenv("CASHFLUX_SERVER_OAUTH_GOOGLE_CLIENT_ID", "google-id")
	t.Setenv("CASHFLUX_SERVER_OAUTH_GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("CASHFLUX_SERVER_OAUTH_GOOGLE_REDIRECT_URL", "http://127.0.0.1:8081/v1/auth/google/callback")
	t.Setenv("CASHFLUX_SERVER_OAUTH_GITHUB_CLIENT_ID", "github-id")
	t.Setenv("CASHFLUX_SERVER_OAUTH_GITHUB_CLIENT_SECRET", "github-secret")
	t.Setenv("CASHFLUX_SERVER_OAUTH_GITHUB_REDIRECT_URL", "http://127.0.0.1:8081/v1/auth/github/callback")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if got := cfg.OAuthProviderNames(); len(got) != 2 || got[0] != "github" || got[1] != "google" {
		t.Fatalf("OAuthProviderNames = %+v", got)
	}
	if cfg.OAuthProviders["google"].ClientSecret != "google-secret" {
		t.Fatalf("google provider = %+v", cfg.OAuthProviders["google"])
	}
}

func TestFromEnvGeneratesTokenModeToken(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_AUTH_MODE", "token")
	t.Setenv("CASHFLUX_SERVER_TOKEN", "")
	t.Setenv("CASHFLUX_SERVER_TOKEN_SHA256", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if !cfg.GeneratedToken || len(cfg.Token) < 32 || cfg.TokenForDisplay() != cfg.Token {
		t.Fatalf("generated token config = %+v", cfg)
	}
}

func TestHealthReadyAndVersionEndpoints(t *testing.T) {
	store := openTestStore(t)
	h := NewMux(Config{
		AuthMode: "oauth",
		Billing:  false,
		OAuthProviders: map[string]OAuthProviderConfig{
			"google": {ClientID: "id", ClientSecret: "secret", RedirectURL: "http://127.0.0.1/v1/auth/google/callback"},
		},
		AppOrigin: "http://127.0.0.1:8080",
	}, store)
	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want 204", path, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
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
	if body.AuthMode != "oauth" || body.BillingEnabled {
		t.Fatalf("mode flags = %+v", body)
	}
	if len(body.AuthProviders) != 1 || body.AuthProviders[0] != "google" {
		t.Fatalf("auth providers = %+v", body.AuthProviders)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:8080" {
		t.Fatalf("version CORS origin = %q", got)
	}
}

func TestRootEndpointAdvertisesBackend(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("root status = %d body %q", rr.Code, rr.Body.String())
	}
	var body RootResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode root response: %v", err)
	}
	if body.Service != "cashflux-server" || body.Status != "ok" {
		t.Fatalf("root response = %+v", body)
	}
	for _, endpoint := range []string{"/grpc", "/v1/version", "/legal/privacy", "/legal/terms"} {
		if !rootEndpointContains(body.Endpoints, endpoint) {
			t.Fatalf("root endpoints = %+v, missing %s", body.Endpoints, endpoint)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing route status = %d, want 404", rr.Code)
	}
}

func TestLegalEndpointsArePublicJSON(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", Token: "dev-token"}, openTestStore(t))
	for _, tc := range []struct {
		path string
		slug string
	}{
		{path: "/legal/privacy", slug: "privacy"},
		{path: "/legal/terms", slug: "terms"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d body %q", rr.Code, rr.Body.String())
			}
			if got := rr.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("content-type = %q, want application/json", got)
			}
			var body LegalResponse
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode legal response: %v", err)
			}
			if body.Slug != tc.slug || body.Title == "" || body.Version == "" || body.EffectiveAt == "" || len(body.Summary) == 0 {
				t.Fatalf("legal body = %+v", body)
			}
		})
	}
}

func TestLegalEndpointsRequireGET(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"}, openTestStore(t))
	req := httptest.NewRequest(http.MethodPost, "/legal/privacy", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post legal status = %d, want 405", rr.Code)
	}
}

func TestStatusEndpointReportsComponents(t *testing.T) {
	store := openTestStore(t)
	h := NewMux(Config{AuthMode: "token"}, store)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d body %q", rr.Code, rr.Body.String())
	}
	var body StatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if body.Service != "cashflux-server" || body.Status != "ok" || body.Components["database"] != "ok" {
		t.Fatalf("status body = %+v", body)
	}
	if body.UpdatedAt.IsZero() {
		t.Fatalf("status updatedAt is zero: %+v", body)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("degraded status code = %d body %q", rr.Code, rr.Body.String())
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode degraded status: %v", err)
	}
	if body.Status != "degraded" || body.Components["database"] != "unavailable" || body.Components["process"] != "ok" {
		t.Fatalf("degraded status body = %+v", body)
	}
}

func rootEndpointContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestSecurityHeaders(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	want := map[string]string{
		"Strict-Transport-Security":    "max-age=31536000; includeSubDomains",
		"X-Content-Type-Options":       "nosniff",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Embedder-Policy": "require-corp",
		"Content-Security-Policy":      "frame-ancestors 'none'",
	}
	for name, value := range want {
		if got := rr.Header().Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestHTTPDataRoutesRequireAuthentication(t *testing.T) {
	store := openTestStore(t)
	h := NewMux(Config{AuthMode: "token", Token: "dev-token", DataDir: t.TempDir()}, store)
	hash := blobHash([]byte("abc"))
	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/metrics"},
		{method: http.MethodGet, path: "/v1/audit"},
		{method: http.MethodPut, path: "/v1/blobs/" + hash + "?workspaceId=w1", body: "abc"},
		{method: http.MethodGet, path: "/v1/blobs/" + hash + "?workspaceId=w1"},
		{method: http.MethodHead, path: "/v1/blobs/" + hash + "?workspaceId=w1"},
		{method: http.MethodGet, path: "/v1/account/export"},
		{method: http.MethodDelete, path: "/v1/account"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d body %q, want 401", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAccountExportAndDeleteEndpoints(t *testing.T) {
	store := openTestStore(t)
	user := authUserFromToken("dev-token")
	other := authUserFromToken("other-token")
	now := time.Date(2026, time.June, 19, 9, 30, 0, 0, time.UTC)
	seedSyncUser(t, store, user.ID, now)
	seedSyncUser(t, store, other.ID, now)
	if err := store.PutWorkspace(Workspace{ID: "w-export", UserID: user.ID, Name: "Export", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	if err := store.PutWorkspace(Workspace{ID: "w-other", UserID: other.ID, Name: "Other", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace other: %v", err)
	}
	if err := store.PutSnapshot(Snapshot{WorkspaceID: "w-export", Dataset: []byte(`{"ok":true}`), Version: 2, UpdatedAt: now}, 1024, 3); err != nil {
		t.Fatalf("PutSnapshot: %v", err)
	}
	dataDir := t.TempDir()
	blob, err := store.PutBlob(blobRoot(Config{DataDir: dataDir}), []byte("receipt"), "text/plain", "receipt.txt", 1024)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if err := store.LinkWorkspaceBlob("w-export", blob.Hash); err != nil {
		t.Fatalf("LinkWorkspaceBlob: %v", err)
	}
	if err := store.PutAIKey(user.ID, "openai", "sk-secret-export", []byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	if _, err := store.AddUsage(user.ID, now, 4, 120); err != nil {
		t.Fatalf("AddUsage: %v", err)
	}
	if _, err := store.AppendAuditEvent(AuditEvent{Timestamp: now, ActorID: user.ID, Action: "workspace.put", TargetType: "workspace", TargetID: "w-export"}); err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}
	h := NewMux(Config{
		AuthMode:  "token",
		Token:     "dev-token",
		DataDir:   dataDir,
		MasterKey: "0123456789abcdef0123456789abcdef",
		AppOrigin: "http://127.0.0.1:8080",
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/export", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("export status = %d body %q", rr.Code, rr.Body.String())
	}
	var export AccountExport
	if err := json.Unmarshal(rr.Body.Bytes(), &export); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if export.User.ID != user.ID || len(export.Workspaces) != 1 || export.Workspaces[0].ID != "w-export" {
		t.Fatalf("export user/workspaces = %+v", export)
	}
	if len(export.Snapshots) != 1 || len(export.Blobs) != 1 || len(export.Usage) != 1 || len(export.AIKeyProviders) != 1 || len(export.AuditEvents) != 1 {
		t.Fatalf("export related rows = snapshots %d blobs %d usage %d providers %d audit %d",
			len(export.Snapshots), len(export.Blobs), len(export.Usage), len(export.AIKeyProviders), len(export.AuditEvents))
	}
	if strings.Contains(rr.Body.String(), "sk-secret-export") || strings.Contains(rr.Body.String(), "w-other") {
		t.Fatalf("export leaked secret or other user data: %s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/account", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body %q", rr.Code, rr.Body.String())
	}
	if _, ok, err := store.GetUserByID(user.ID); err != nil || ok {
		t.Fatalf("deleted user lookup = ok %v err %v", ok, err)
	}
	if _, ok, err := store.GetUserByID(other.ID); err != nil || !ok {
		t.Fatalf("other user lookup = ok %v err %v", ok, err)
	}
	if _, ok, err := store.GetBlob(blob.Hash); err != nil || ok {
		t.Fatalf("deleted user's blob metadata = ok %v err %v", ok, err)
	}
}

func TestAccountAndAdminErrorsReturnMachineReadableReasons(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", Token: "dev-token", AppOrigin: "http://127.0.0.1:8080"}, openTestStore(t))
	for _, tc := range []struct {
		name   string
		method string
		path   string
		status int
		reason ErrorReason
		auth   bool
	}{
		{name: "account export unauthenticated", method: http.MethodGet, path: "/v1/account/export", status: http.StatusUnauthorized, reason: ErrorReasonUnauthenticated},
		{name: "account delete unauthenticated", method: http.MethodDelete, path: "/v1/account", status: http.StatusUnauthorized, reason: ErrorReasonUnauthenticated},
		{name: "admin bad day", method: http.MethodGet, path: "/v1/admin/usage?day=2026-99-99", status: http.StatusBadRequest, reason: ErrorReasonInvalidArgument, auth: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Origin", "http://127.0.0.1:8080")
			if tc.auth {
				req.Header.Set("Authorization", "Bearer dev-token")
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("status = %d body %q, want %d", rr.Code, rr.Body.String(), tc.status)
			}
			if rr.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("content-type = %q", rr.Header().Get("Content-Type"))
			}
			var body ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if body.Error.Reason != tc.reason || body.Error.Message == "" {
				t.Fatalf("error body = %+v, want reason %s", body, tc.reason)
			}
		})
	}
}

func TestMetricsEndpointRequiresAuth(t *testing.T) {
	metrics := NewMetrics()
	h := NewMux(Config{AuthMode: "token", Token: "dev-token", Metrics: metrics}, openTestStore(t))
	versionReq := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	h.ServeHTTP(httptest.NewRecorder(), versionReq)
	metrics.ObserveGRPC("/cashflux.v1.SyncService/ListWorkspaces", "OK", 2*time.Millisecond)
	metrics.ObserveStreamDuration("/cashflux.v1.SyncService/WatchWorkspaces", "OK", 3*time.Millisecond)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status = %d, want 401", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonUnauthenticated)

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/plain; version=0.0.4" {
		t.Fatalf("metrics content-type = %q", got)
	}
	if !strings.Contains(rr.Body.String(), "cashflux_server_up 1") {
		t.Fatalf("metrics body = %q", rr.Body.String())
	}
	for _, want := range []string{
		`cashflux_http_requests_total{route="/v1/version",status="200"} 1`,
		`cashflux_http_request_duration_seconds_bucket{route="/v1/version",status="200",le="+Inf"} 1`,
		`cashflux_grpc_requests_total{method="/cashflux.v1.SyncService/ListWorkspaces",status="OK"} 1`,
		`cashflux_grpc_stream_duration_seconds_sum{method="/cashflux.v1.SyncService/WatchWorkspaces",status="OK"} 0.003000`,
	} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("metrics body missing %q in %q", want, rr.Body.String())
		}
	}
}

func TestAuditEndpointStreamsNDJSON(t *testing.T) {
	store := openTestStore(t)
	event, err := store.AppendAuditEvent(AuditEvent{
		Timestamp:  time.Date(2026, time.June, 19, 2, 15, 0, 0, time.UTC),
		ActorID:    "token:abc",
		Action:     "workspace.put",
		TargetType: "workspace",
		TargetID:   "w1",
		RequestID:  "req-audit",
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}
	h := NewMux(Config{AuthMode: "token", Token: "dev-token", AppOrigin: "http://127.0.0.1:8080"}, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("audit status = %d body %q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("audit content-type = %q", rr.Header().Get("Content-Type"))
	}
	var got AuditEvent
	if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&got); err != nil {
		t.Fatalf("decode audit ndjson: %v", err)
	}
	if got.ID != event.ID || got.Action != "workspace.put" || got.Hash == "" {
		t.Fatalf("audit event = %+v, want %+v", got, event)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized audit status = %d, want 401", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonUnauthenticated)
}

func TestAdminUsageEndpointIsReadOnlyAndScopedToAuthenticatedUser(t *testing.T) {
	store := openTestStore(t)
	day := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	user := authUserFromToken("dev-token")
	other := authUserFromToken("other-token")
	if err := store.UpsertUser(User{ID: user.ID, Provider: "token", Subject: user.ID, CreatedAt: day}); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if err := store.UpsertUser(User{ID: other.ID, Provider: "token", Subject: other.ID, CreatedAt: day}); err != nil {
		t.Fatalf("upsert other user: %v", err)
	}
	if _, err := store.AddUsage(user.ID, day, 3, 90); err != nil {
		t.Fatalf("add user usage: %v", err)
	}
	if _, err := store.AddUsage(other.ID, day, 99, 9999); err != nil {
		t.Fatalf("add other usage: %v", err)
	}
	h := NewMux(Config{AuthMode: "token", Token: "dev-token", AppOrigin: "http://127.0.0.1:8080"}, store)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/usage?day=2026-06-19&userId="+other.ID, nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Authorization", "Bearer dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("usage status = %d body %q", rr.Code, rr.Body.String())
	}
	var body AdminUsageResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode usage: %v", err)
	}
	if body.UserID != user.ID || body.Day != "2026-06-19" || body.Requests != 3 || body.Tokens != 90 {
		t.Fatalf("usage body = %+v, want only authenticated user's usage", body)
	}
	if body.UserID == other.ID || body.Requests == 99 || body.Tokens == 9999 {
		t.Fatalf("usage leaked other user: %+v", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/usage?day=2026-06-19", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized usage status = %d, want 401", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/usage?day=2026-99-99", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad day status = %d, want 400", rr.Code)
	}
}

func TestMaxInFlightMiddlewareRejectsWhenBusy(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	h := maxInFlightMiddleware(1, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	go h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow", nil))
	<-entered
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/second", nil))
	close(release)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("busy status = %d, want 503", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonServerUnavailable)
}

func TestRateLimitMiddlewareRejectsAfterLimit(t *testing.T) {
	var hits int
	h := rateLimitMiddleware(2, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "198.51.100.8:1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want 204", i+1, rr.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "198.51.100.8:4567"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d, want 429", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonRateLimited)
	if got := rr.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("retry-after = %q, want 60", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "198.51.100.9:1234"
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("other client status = %d, want 204", rr.Code)
	}
	if hits != 3 {
		t.Fatalf("handler hits = %d, want 3", hits)
	}
}

func TestRateLimitMiddlewareHonorsForwardedClient(t *testing.T) {
	h := rateLimitMiddleware(1, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	first := httptest.NewRequest(http.MethodGet, "/limited", nil)
	first.RemoteAddr = "198.51.100.1:1234"
	first.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.10")
	h.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequest(http.MethodGet, "/limited", nil)
	second.RemoteAddr = "198.51.100.2:1234"
	second.Header.Set("X-Forwarded-For", "203.0.113.7")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, second)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("forwarded client status = %d, want 429", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonRateLimited)
}

func TestUserRateLimitMiddlewareRejectsAfterLimit(t *testing.T) {
	cfg := Config{AuthMode: "oauth", MasterKey: "0123456789abcdef0123456789abcdef"}
	now := time.Now().UTC()
	userA, err := issueSessionToken(cfg, "github:1", "access", time.Hour, now)
	if err != nil {
		t.Fatalf("issue user A token: %v", err)
	}
	userB, err := issueSessionToken(cfg, "github:2", "access", time.Hour, now)
	if err != nil {
		t.Fatalf("issue user B token: %v", err)
	}
	var hits int
	h := userRateLimitMiddleware(1, cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "198.51.100.1:1234"
	req.Header.Set("Authorization", "Bearer "+userA)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("first user request status = %d, want 204", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "198.51.100.2:1234"
	req.Header.Set("Authorization", "Bearer "+userA)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("limited user status = %d, want 429", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonRateLimited)
	if got := rr.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("retry-after = %q, want 60", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "198.51.100.3:1234"
	req.Header.Set("Authorization", "Bearer "+userB)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("other user status = %d, want 204", rr.Code)
	}
	if hits != 2 {
		t.Fatalf("handler hits = %d, want 2", hits)
	}
}

func TestAuthRateLimitMiddlewareRejectsAfterLimit(t *testing.T) {
	h := authRateLimitMiddleware(1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github", nil)
	req.RemoteAddr = "198.51.100.55:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("first auth request status = %d, want 204", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/auth/github", nil)
	req.RemoteAddr = "198.51.100.55:5678"
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("limited auth status = %d, want 429", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("retry-after = %q, want 60", got)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonRateLimited)
}

func TestOAuthStartRedirectsWithPKCEState(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{
		AuthMode: "oauth",
		OAuthProviders: map[string]OAuthProviderConfig{
			"github": {
				ClientID:     "github-id",
				ClientSecret: "github-secret",
				RedirectURL:  "http://127.0.0.1:8081/v1/auth/github/callback",
			},
		},
	}
	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("oauth start status = %d body %q", rr.Code, rr.Body.String())
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if loc.Scheme != "https" || loc.Host != "github.com" || loc.Path != "/login/oauth/authorize" {
		t.Fatalf("redirect location = %s", loc.String())
	}
	q := loc.Query()
	if q.Get("client_id") != "github-id" || q.Get("redirect_uri") != "http://127.0.0.1:8081/v1/auth/github/callback" {
		t.Fatalf("redirect query = %s", loc.RawQuery)
	}
	if q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("state") == "" {
		t.Fatalf("missing pkce/state query = %s", loc.RawQuery)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != oauthStateCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("oauth cookies = %+v", cookies)
	}
	if !strings.HasPrefix(cookies[0].Value, q.Get("state")+".") {
		t.Fatalf("state cookie value does not match redirect state")
	}
	if _, _, nonce, ok := parseOAuthStateCookie(cookies[0].Value); !ok || nonce == "" {
		t.Fatalf("state cookie missing nonce: %q", cookies[0].Value)
	}
}

func TestOAuthStartAddsGoogleNonce(t *testing.T) {
	h := NewMux(Config{AuthMode: "oauth", OAuthProviders: map[string]OAuthProviderConfig{
		"google": {
			ClientID:     "google-id",
			ClientSecret: "google-secret",
			RedirectURL:  "http://127.0.0.1:8081/v1/auth/google/callback",
		},
	}}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("oauth start status = %d body %q", rr.Code, rr.Body.String())
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	q := loc.Query()
	if q.Get("nonce") == "" {
		t.Fatalf("google redirect missing nonce: %s", loc.RawQuery)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("oauth cookies = %+v", cookies)
	}
	_, _, nonce, ok := parseOAuthStateCookie(cookies[0].Value)
	if !ok || nonce != q.Get("nonce") {
		t.Fatalf("nonce cookie/query mismatch cookie=%q query=%q", cookies[0].Value, q.Get("nonce"))
	}
}

func TestOAuthStartRejectsUnconfiguredProvider(t *testing.T) {
	h := NewMux(Config{AuthMode: "oauth", OAuthProviders: map[string]OAuthProviderConfig{
		"github": {ClientID: "id", ClientSecret: "secret", RedirectURL: "http://127.0.0.1/v1/auth/github/callback"},
	}}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unconfigured provider status = %d, want 404", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonNotFound)
}

func TestOAuthCallbackIssuesSessionAndRefreshLogout(t *testing.T) {
	store := openTestStore(t)
	var sawVerifier bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if r.Form.Get("code") != "oauth-code" || r.Form.Get("code_verifier") != "verifier-123" {
				t.Fatalf("token form = %v", r.Form)
			}
			sawVerifier = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"provider-token","token_type":"bearer"}`))
		case "/user":
			if r.Header.Get("Authorization") != "Bearer provider-token" {
				t.Fatalf("user authorization = %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":42,"email":"alice@example.com"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	cfg := Config{
		AuthMode:  "oauth",
		AppOrigin: "http://127.0.0.1:8080",
		MasterKey: "0123456789abcdef0123456789abcdef",
		OAuthProviders: map[string]OAuthProviderConfig{
			"github": {
				ClientID:     "github-id",
				ClientSecret: "github-secret",
				RedirectURL:  "http://127.0.0.1:8081/v1/auth/github/callback",
				TokenURL:     provider.URL + "/token",
				UserURL:      provider.URL + "/user",
			},
		},
	}
	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github/callback?code=oauth-code&state=state-123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "state-123.verifier-123.nonce-123"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("callback status = %d body %q", rr.Code, rr.Body.String())
	}
	if !sawVerifier {
		t.Fatal("token exchange did not receive PKCE verifier")
	}
	var body oauthSessionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode callback: %v", err)
	}
	if body.TokenType != "Bearer" || body.AccessToken == "" || body.UserID != "github:42" {
		t.Fatalf("callback body = %+v", body)
	}
	if _, ok := authUserForToken(body.AccessToken, cfg); !ok {
		t.Fatal("issued access token did not authenticate")
	}
	if _, ok, err := store.GetUserByID("github:42"); err != nil || !ok {
		t.Fatalf("stored oauth user = %v/%v", ok, err)
	}
	var refreshCookie *http.Cookie
	var csrfCookie *http.Cookie
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == sessionRefreshCookie {
			refreshCookie = cookie
		}
		if cookie.Name == sessionCSRFCookie {
			csrfCookie = cookie
		}
	}
	if refreshCookie == nil || !refreshCookie.HttpOnly || refreshCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("refresh cookie = %+v", refreshCookie)
	}
	if csrfCookie == nil || csrfCookie.HttpOnly || csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("csrf cookie = %+v", csrfCookie)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	refreshReq.Header.Set("Origin", "http://127.0.0.1:8080")
	refreshReq.AddCookie(refreshCookie)
	refreshRR := httptest.NewRecorder()
	h.ServeHTTP(refreshRR, refreshReq)
	if refreshRR.Code != http.StatusForbidden {
		t.Fatalf("refresh without csrf status = %d, want 403", refreshRR.Code)
	}
	assertHTTPErrorReason(t, refreshRR, ErrorReasonPermissionDenied)

	refreshReq = httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	refreshReq.Header.Set("Origin", "http://127.0.0.1:8080")
	refreshReq.Header.Set(sessionCSRFHeader, csrfCookie.Value)
	refreshReq.AddCookie(refreshCookie)
	refreshReq.AddCookie(csrfCookie)
	refreshRR = httptest.NewRecorder()
	h.ServeHTTP(refreshRR, refreshReq)
	if refreshRR.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body %q", refreshRR.Code, refreshRR.Body.String())
	}
	var refreshed oauthSessionResponse
	if err := json.Unmarshal(refreshRR.Body.Bytes(), &refreshed); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.UserID != "github:42" {
		t.Fatalf("refresh body = %+v", refreshed)
	}
	oldRefreshCookie := refreshCookie
	for _, cookie := range refreshRR.Result().Cookies() {
		if cookie.Name == sessionRefreshCookie {
			refreshCookie = cookie
		}
		if cookie.Name == sessionCSRFCookie {
			csrfCookie = cookie
		}
	}
	if oldRefreshCookie.Value == refreshCookie.Value {
		t.Fatal("refresh token was not rotated")
	}

	reuseReq := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	reuseReq.Header.Set("Origin", "http://127.0.0.1:8080")
	reuseReq.Header.Set(sessionCSRFHeader, csrfCookie.Value)
	reuseReq.AddCookie(oldRefreshCookie)
	reuseReq.AddCookie(csrfCookie)
	reuseRR := httptest.NewRecorder()
	h.ServeHTTP(reuseRR, reuseReq)
	if reuseRR.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh status = %d body %q", reuseRR.Code, reuseRR.Body.String())
	}
	assertHTTPErrorReason(t, reuseRR, ErrorReasonUnauthenticated)

	revokedReq := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	revokedReq.Header.Set("Origin", "http://127.0.0.1:8080")
	revokedReq.Header.Set(sessionCSRFHeader, csrfCookie.Value)
	revokedReq.AddCookie(refreshCookie)
	revokedReq.AddCookie(csrfCookie)
	revokedRR := httptest.NewRecorder()
	h.ServeHTTP(revokedRR, revokedReq)
	if revokedRR.Code != http.StatusUnauthorized {
		t.Fatalf("family revoked refresh status = %d body %q", revokedRR.Code, revokedRR.Body.String())
	}
	assertHTTPErrorReason(t, revokedRR, ErrorReasonUnauthenticated)

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	logoutReq.Header.Set("Origin", "http://127.0.0.1:8080")
	logoutReq.Header.Set(sessionCSRFHeader, csrfCookie.Value)
	logoutReq.AddCookie(csrfCookie)
	logoutRR := httptest.NewRecorder()
	h.ServeHTTP(logoutRR, logoutReq)
	if logoutRR.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutRR.Code)
	}
}

func TestOAuthCallbackValidatesGoogleIDTokenClaims(t *testing.T) {
	store := openTestStore(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			idToken := testIDToken(t, map[string]any{
				"iss":   "https://accounts.google.com",
				"aud":   "google-id",
				"nonce": "nonce-123",
			})
			_, _ = w.Write([]byte(`{"access_token":"provider-token","id_token":` + strconvQuote(idToken) + `}`))
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sub":"user-123","email":"alice@example.com"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	cfg := Config{
		AuthMode:  "oauth",
		MasterKey: "0123456789abcdef0123456789abcdef",
		OAuthProviders: map[string]OAuthProviderConfig{
			"google": {
				ClientID:     "google-id",
				ClientSecret: "google-secret",
				RedirectURL:  "http://127.0.0.1:8081/v1/auth/google/callback",
				TokenURL:     provider.URL + "/token",
				UserURL:      provider.URL + "/user",
			},
		},
	}
	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=oauth-code&state=state-123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "state-123.verifier-123.nonce-123"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("callback status = %d body %q", rr.Code, rr.Body.String())
	}
	var body oauthSessionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode callback: %v", err)
	}
	if body.UserID != "google:user-123" {
		t.Fatalf("callback body = %+v", body)
	}
}

func TestOAuthCallbackRejectsGoogleIDTokenAudience(t *testing.T) {
	store := openTestStore(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			idToken := testIDToken(t, map[string]any{
				"iss":   "https://accounts.google.com",
				"aud":   "other-client",
				"nonce": "nonce-123",
			})
			_, _ = w.Write([]byte(`{"access_token":"provider-token","id_token":` + strconvQuote(idToken) + `}`))
		case "/user":
			t.Fatal("userinfo should not be fetched after invalid id token")
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	cfg := Config{
		AuthMode:  "oauth",
		MasterKey: "0123456789abcdef0123456789abcdef",
		OAuthProviders: map[string]OAuthProviderConfig{
			"google": {
				ClientID:     "google-id",
				ClientSecret: "google-secret",
				RedirectURL:  "http://127.0.0.1:8081/v1/auth/google/callback",
				TokenURL:     provider.URL + "/token",
				UserURL:      provider.URL + "/user",
			},
		},
	}
	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=oauth-code&state=state-123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "state-123.verifier-123.nonce-123"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway || !strings.Contains(rr.Body.String(), "audience") {
		t.Fatalf("callback status/body = %d/%q, want bad audience", rr.Code, rr.Body.String())
	}
	assertHTTPErrorReason(t, rr, ErrorReasonUpstreamUnavailable)
}

func TestOAuthCallbackRequiresGoogleIDToken(t *testing.T) {
	store := openTestStore(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"provider-token"}`))
		case "/user":
			t.Fatal("userinfo should not be fetched without an id token")
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	cfg := Config{
		AuthMode:  "oauth",
		MasterKey: "0123456789abcdef0123456789abcdef",
		OAuthProviders: map[string]OAuthProviderConfig{
			"google": {
				ClientID:     "google-id",
				ClientSecret: "google-secret",
				RedirectURL:  "http://127.0.0.1:8081/v1/auth/google/callback",
				TokenURL:     provider.URL + "/token",
				UserURL:      provider.URL + "/user",
			},
		},
	}
	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=oauth-code&state=state-123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "state-123.verifier-123.nonce-123"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway || !strings.Contains(rr.Body.String(), "required") {
		t.Fatalf("callback status/body = %d/%q, want missing id token", rr.Code, rr.Body.String())
	}
	assertHTTPErrorReason(t, rr, ErrorReasonUpstreamUnavailable)
}

func TestReadyEndpointRequiresStore(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"})
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("live without store status = %d, want 204", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("health without store status = %d, want 204", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready without store status = %d, want 503", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonServerUnavailable)
}

func TestLegacyAIHTTPEndpointsAreNotMounted(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", AppOrigin: "http://127.0.0.1:8080"})
	for _, path := range []string{"/v1/ai/key", "/v1/ai/chat", "/v1/ai/vision"} {
		for _, method := range []string{http.MethodOptions, http.MethodPost} {
			req := httptest.NewRequest(method, path, nil)
			req.Header.Set("Origin", "http://127.0.0.1:8080")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusNotFound {
				t.Fatalf("%s %s status = %d, want 404", method, path, rr.Code)
			}
		}
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
	user := authUserFromToken("dev-token")
	now := time.Date(2026, time.June, 18, 23, 30, 0, 0, time.UTC)
	seedSyncUser(t, store, user.ID, now)
	if err := store.PutWorkspace(Workspace{ID: "w1", UserID: user.ID, Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	data := []byte("receipt bytes")
	hash := blobHash(data)
	cfg := Config{
		AuthMode:     "token",
		Token:        "dev-token",
		DataDir:      t.TempDir(),
		BlobMaxBytes: 1024,
		Metrics:      NewMetrics(),
	}
	h := NewMux(cfg, store)

	req := httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash+"?workspaceId=w1", bytes.NewReader(data))
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

	req = httptest.NewRequest(http.MethodHead, "/v1/blobs/"+hash+"?workspaceId=w1", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("head blob status = %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "image/png" || rr.Header().Get("ETag") != `"`+hash+`"` {
		t.Fatalf("head headers = content-type %q etag %q", rr.Header().Get("Content-Type"), rr.Header().Get("ETag"))
	}
	if rr.Header().Get("Content-Disposition") != "attachment" {
		t.Fatalf("content-disposition = %q", rr.Header().Get("Content-Disposition"))
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("head body length = %d, want 0", rr.Body.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/blobs/"+hash+"?workspaceId=w1", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != string(data) {
		t.Fatalf("get blob status/body = %d/%q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("cache-control = %q", rr.Header().Get("Cache-Control"))
	}

	var metricsOut bytes.Buffer
	cfg.Metrics.WritePrometheus(&metricsOut)
	for _, want := range []string{
		"cashflux_blob_stored_bytes_total 13",
		"cashflux_blob_transferred_bytes_total 13",
	} {
		if !strings.Contains(metricsOut.String(), want) {
			t.Fatalf("blob metrics missing %q in %q", want, metricsOut.String())
		}
	}
}

func TestBlobEndpointsRejectBadAuthHashAndOversize(t *testing.T) {
	store := openTestStore(t)
	user := authUserFromToken("dev-token")
	now := time.Date(2026, time.June, 18, 23, 35, 0, 0, time.UTC)
	seedSyncUser(t, store, user.ID, now)
	if err := store.PutWorkspace(Workspace{ID: "w1", UserID: user.ID, Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	cfg := Config{AuthMode: "token", Token: "dev-token", DataDir: t.TempDir(), BlobMaxBytes: 4}
	h := NewMux(cfg, store)
	hash := blobHash([]byte("abc"))

	req := httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash+"?workspaceId=w1", bytes.NewReader([]byte("abc")))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonUnauthenticated)

	req = httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash+"?workspaceId=w1", bytes.NewReader([]byte("wrong")))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonPayloadTooLarge)

	req = httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash+"?workspaceId=w1", bytes.NewReader([]byte("abd")))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("hash mismatch status = %d", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonInvalidArgument)

	req = httptest.NewRequest(http.MethodGet, "/v1/blobs/not-a-hash?workspaceId=w1", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad hash status = %d", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonInvalidArgument)

	safeH := NewMux(Config{AuthMode: "token", Token: "dev-token", DataDir: t.TempDir(), BlobMaxBytes: 1024}, store)
	html := []byte("<!doctype html><script>alert(1)</script>")
	req = httptest.NewRequest(http.MethodPut, "/v1/blobs/"+blobHash(html)+"?workspaceId=w1", bytes.NewReader(html))
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Content-Type", "text/plain")
	rr = httptest.NewRecorder()
	safeH.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("html sniff status = %d, want 415", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonUnsupportedMedia)

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	req = httptest.NewRequest(http.MethodPut, "/v1/blobs/"+blobHash(svg)+"?workspaceId=w1", bytes.NewReader(svg))
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Content-Type", "image/svg+xml")
	rr = httptest.NewRecorder()
	safeH.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("svg declared status = %d, want 415", rr.Code)
	}
	assertHTTPErrorReason(t, rr, ErrorReasonUnsupportedMedia)
}

func TestBlobEndpointRejectsStorageQuotaExceeded(t *testing.T) {
	store := openTestStore(t)
	user := authUserFromToken("dev-token")
	now := time.Date(2026, time.June, 19, 3, 30, 0, 0, time.UTC)
	seedSyncUser(t, store, user.ID, now)
	if err := store.PutWorkspace(Workspace{ID: "w1", UserID: user.ID, Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	cfg := Config{AuthMode: "token", Token: "dev-token", DataDir: t.TempDir(), StorageMaxBytes: 3}
	h := NewMux(cfg, store)
	data := []byte("abcd")
	req := httptest.NewRequest(http.MethodPut, "/v1/blobs/"+blobHash(data)+"?workspaceId=w1", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusInsufficientStorage {
		t.Fatalf("quota status = %d body %q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "storage quota exceeded") {
		t.Fatalf("quota body = %q", rr.Body.String())
	}
	assertHTTPErrorReason(t, rr, ErrorReasonResourceExhausted)
}

func assertHTTPErrorReason(t *testing.T, rr *httptest.ResponseRecorder, want ErrorReason) {
	t.Helper()
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type = %q, want application/json", rr.Header().Get("Content-Type"))
	}
	var body ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v body %q", err, rr.Body.String())
	}
	if body.Error.Reason != want {
		t.Fatalf("error reason = %s, want %s (body %+v)", body.Error.Reason, want, body)
	}
}

func TestBlobEndpointsRequireOwnedWorkspaceLink(t *testing.T) {
	store := openTestStore(t)
	u1 := authUserFromToken("dev-token")
	u2 := authUserFromToken("other-token")
	now := time.Date(2026, time.June, 18, 23, 40, 0, 0, time.UTC)
	seedSyncUser(t, store, u1.ID, now)
	seedSyncUser(t, store, u2.ID, now)
	if err := store.PutWorkspace(Workspace{ID: "w1", UserID: u1.ID, Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace u1: %v", err)
	}
	if err := store.PutWorkspace(Workspace{ID: "w2", UserID: u2.ID, Name: "Other", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace u2: %v", err)
	}
	cfg := Config{AuthMode: "token", Token: "dev-token", DataDir: t.TempDir(), BlobMaxBytes: 1024}
	h := NewMux(cfg, store)
	data := []byte("private receipt")
	hash := blobHash(data)

	req := httptest.NewRequest(http.MethodPut, "/v1/blobs/"+hash+"?workspaceId=w1", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put own blob status = %d body %q", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/blobs/"+hash+"?workspaceId=w2", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get other workspace blob status = %d, want 404", rr.Code)
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

func blobHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func testIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "none"})
	if err != nil {
		t.Fatalf("marshal id token header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal id token claims: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
