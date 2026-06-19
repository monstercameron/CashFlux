package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// VersionResponse is returned by /v1/version for client compatibility checks.
type VersionResponse struct {
	APIVersion          string   `json:"apiVersion"`
	MinClientAPIVersion string   `json:"minClientApiVersion"`
	AuthMode            string   `json:"authMode"`
	BillingEnabled      bool     `json:"billingEnabled"`
	AuthProviders       []string `json:"authProviders,omitempty"`
}

// RootResponse is returned by / for local backend discoverability.
type RootResponse struct {
	Service   string   `json:"service"`
	Status    string   `json:"status"`
	Endpoints []string `json:"endpoints"`
}

// StatusResponse is returned by /status for simple status-page checks.
type StatusResponse struct {
	Service    string            `json:"service"`
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

// LegalResponse exposes public legal document metadata for Cloud onboarding.
type LegalResponse struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Version     string   `json:"version"`
	EffectiveAt string   `json:"effectiveAt"`
	Summary     []string `json:"summary"`
}

// NewMux returns the backend HTTP surface that exists before gRPC/proto wiring.
func NewMux(cfg Config, stores ...*Store) http.Handler {
	var store *Store
	if len(stores) > 0 {
		store = stores[0]
	}
	if cfg.Metrics == nil {
		cfg.Metrics = NewMetrics()
	}
	if store != nil {
		store.SetMetrics(cfg.Metrics)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, RootResponse{
			Service: "cashflux-server",
			Status:  "ok",
			Endpoints: []string{
				"/livez",
				"/status",
				"/healthz",
				"/readyz",
				"/v1/version",
				"/v1/audit",
				"/legal/privacy",
				"/legal/terms",
				"/grpc",
			},
		})
	})
	mux.HandleFunc("GET /legal/privacy", handleLegalDocument(privacyPolicyDocument()))
	mux.HandleFunc("GET /legal/terms", handleLegalDocument(termsDocument()))
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		code := http.StatusOK
		status := "ok"
		components := map[string]string{"process": "ok", "database": "ok"}
		if err := store.Ready(); err != nil {
			code = http.StatusServiceUnavailable
			status = "degraded"
			components["database"] = "unavailable"
		}
		writeJSONStatus(w, code, StatusResponse{
			Service:    "cashflux-server",
			Status:     status,
			Components: components,
			UpdatedAt:  time.Now().UTC(),
		})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if err := store.Ready(); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /metrics", handleMetrics(cfg))
	mux.HandleFunc("OPTIONS /v1/audit", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/audit", handleAuditEvents(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/usage", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/admin/usage", handleAdminUsage(cfg, store))
	mux.HandleFunc("OPTIONS /v1/version", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		writeJSON(w, VersionResponse{
			APIVersion:          APIVersion,
			MinClientAPIVersion: MinClientAPIVersion,
			AuthMode:            cfg.AuthMode,
			BillingEnabled:      cfg.Billing,
			AuthProviders:       cfg.OAuthProviderNames(),
		})
	})
	mux.HandleFunc("GET /v1/auth/{provider}", handleOAuthStart(cfg))
	mux.HandleFunc("GET /v1/auth/{provider}/callback", handleOAuthCallback(cfg, store))
	mux.HandleFunc("OPTIONS /v1/auth/refresh", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/auth/refresh", handleOAuthRefresh(cfg, store))
	mux.HandleFunc("OPTIONS /v1/auth/logout", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/auth/logout", handleOAuthLogout(cfg, store))
	mux.Handle("/grpc", NewGRPCBridgeHandler(cfg, store))
	mux.HandleFunc("OPTIONS /v1/blobs/{hash}", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /v1/blobs/{hash}", handlePutBlob(cfg, store))
	mux.HandleFunc("GET /v1/blobs/{hash}", handleGetBlob(cfg, store))
	mux.HandleFunc("HEAD /v1/blobs/{hash}", handleHeadBlob(cfg, store))
	return maxInFlightMiddleware(cfg.HTTPMaxInFlight, securityHeadersMiddleware(requestIDMiddleware(requestLogMiddlewareSampled(cfg.Logger, cfg.Metrics, cfg.LogHotPathSampleRate, userRateLimitMiddleware(cfg.HTTPUserRateLimitPerMinute, cfg, rateLimitMiddleware(cfg.HTTPRateLimitPerMinute, mux))))))
}

func handleLegalDocument(doc LegalResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, doc)
	}
}

func privacyPolicyDocument() LegalResponse {
	return LegalResponse{
		Slug:        "privacy",
		Title:       "CashFlux Privacy Policy",
		Version:     "draft-2026-06-19",
		EffectiveAt: "2026-06-19",
		Summary: []string{
			"CashFlux Cloud stores sync snapshots, blob metadata, account metadata, usage counters, and encrypted BYO AI keys needed to provide optional sync and AI proxy services.",
			"CashFlux does not sell personal data. Payment card processing is delegated to Stripe when billing is enabled.",
			"Users can keep using CashFlux locally without Cloud. Self-hosted servers keep data under the operator's control.",
			"Account export and deletion are planned compliance surfaces and remain tracked separately before public Cloud launch.",
		},
	}
}

func termsDocument() LegalResponse {
	return LegalResponse{
		Slug:        "terms",
		Title:       "CashFlux Terms of Service",
		Version:     "draft-2026-06-19",
		EffectiveAt: "2026-06-19",
		Summary: []string{
			"CashFlux Cloud is optional. The local-first app remains usable without a paid server account.",
			"Users are responsible for the financial data and provider keys they add, including any self-hosted deployment configuration.",
			"Cloud billing, trials, entitlements, and subscription management are planned Stripe-backed surfaces and remain subject to launch configuration.",
			"CashFlux provides budgeting tools and automation helpers, not financial, tax, legal, or investment advice.",
		},
	}
}

func handleCORSPreflight(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleMetrics(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := httpBearerUser(r, cfg); !ok {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		cfg.Metrics.WritePrometheus(w)
	}
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		header.Set("Cross-Origin-Embedder-Policy", "require-corp")
		header.Set("Content-Security-Policy", "frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func maxInFlightMiddleware(limit int, next http.Handler) http.Handler {
	if limit <= 0 {
		return next
	}
	sem := make(chan struct{}, limit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "server is busy", http.StatusServiceUnavailable)
		}
	})
}

type rateLimitBucket struct {
	windowStart time.Time
	count       int
}

type fixedWindowLimiter struct {
	limit   int
	mu      sync.Mutex
	buckets map[string]rateLimitBucket
}

func newFixedWindowLimiter(limit int) *fixedWindowLimiter {
	return &fixedWindowLimiter{limit: limit, buckets: map[string]rateLimitBucket{}}
}

func (l *fixedWindowLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket := l.buckets[key]
	if bucket.windowStart.IsZero() || now.Sub(bucket.windowStart) >= time.Minute {
		bucket = rateLimitBucket{windowStart: now}
	}
	bucket.count++
	l.buckets[key] = bucket
	return bucket.count <= l.limit
}

func rateLimitMiddleware(limit int, next http.Handler) http.Handler {
	if limit <= 0 {
		return next
	}
	limiter := newFixedWindowLimiter(limit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r), time.Now()) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func userRateLimitMiddleware(limit int, cfg Config, next http.Handler) http.Handler {
	if limit <= 0 {
		return next
	}
	limiter := newFixedWindowLimiter(limit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := httpBearerUser(r, cfg)
		if ok && !limiter.allow(user.ID, time.Now()) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "user rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	for _, part := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		if ip := strings.TrimSpace(part); ip != "" {
			return ip
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if remote := strings.TrimSpace(r.RemoteAddr); remote != "" {
		return remote
	}
	return "unknown"
}

func newAIService(store *Store, cfg Config) *AIService {
	return NewAIService(store, AIServiceConfig{
		MasterKey:       []byte(cfg.MasterKey),
		BaseURL:         cfg.OpenAIBaseURL,
		Disabled:        cfg.AIProxyDisabled,
		AllowedModels:   cfg.AIAllowedModels,
		UpstreamTimeout: cfg.AIUpstreamTimeout,
		UpstreamRetries: cfg.AIUpstreamRetries,
		RequestMaxBytes: cfg.AIRequestMaxBytes,
		RequestsPerDay:  cfg.AIRequestsPerDay,
		TokensPerDay:    cfg.AITokensPerDay,
		Metrics:         cfg.Metrics,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func httpBearerUser(r *http.Request, cfg Config) (AuthUser, bool) {
	header := r.Header.Get("Authorization")
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") || strings.TrimSpace(fields[1]) == "" {
		return AuthUser{}, false
	}
	user, ok := authUserForToken(strings.TrimSpace(fields[1]), cfg)
	if !ok {
		return AuthUser{}, false
	}
	return user, true
}

func authUserFromToken(token string) AuthUser {
	sum := sha256.Sum256([]byte(token))
	id := "token:" + hex.EncodeToString(sum[:])[:24]
	return AuthUser{ID: id, Token: token}
}

func writeCORS(w http.ResponseWriter, r *http.Request, cfg Config) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if !allowedOrigin(origin, cfg.AppOrigin) {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, POST, OPTIONS")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, ETag")
	w.Header().Set("Access-Control-Max-Age", "600")
	return true
}

func allowedOrigin(origin, configured string) bool {
	configured = strings.TrimSpace(configured)
	if configured == "*" {
		return true
	}
	if configured != "" {
		return origin == configured
	}
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://[::1]:")
}
