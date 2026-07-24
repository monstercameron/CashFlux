// SPDX-License-Identifier: MIT

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
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
	// PaymentProviders lists the configured, ready-to-use payment providers
	// ("stripe", "paypal") so the client offers only the buttons that will work.
	PaymentProviders []string `json:"paymentProviders,omitempty"`
	// CustomAuthEnabled reports whether AuthServiceServer is registered on this
	// backend (username/password and pairing-code sign-in) — true
	// on the full server and NewSyncAndAuthBridgeHandler, false on
	// NewSyncBridgeHandler (SyncService only, single static token). Lets the
	// client show only the sign-in methods this specific backend actually
	// supports instead of every method CashFlux can ever offer.
	CustomAuthEnabled bool `json:"customAuthEnabled"`
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
	authLimiter := authRateLimitMiddleware(cfg.AuthRateLimitPerMinute, cfg)
	// webhookMu serializes the check-apply-record critical section of every provider
	// webhook so a replay is deduped and a failed apply is never marked "seen".
	webhookMu := &sync.Mutex{}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/console/", http.StatusFound)
			return
		}
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
				"/v1/account/export",
				"/v1/billing/checkout",
				"/v1/billing/portal",
				"/v1/billing/status",
				"/v1/billing/stripe/webhook",
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
			writeErrorJSON(w, ErrorReasonServerUnavailable, "store is not ready")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /metrics", handleMetrics(cfg))
	mux.HandleFunc("OPTIONS /v1/audit", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/audit", handleAuditEvents(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/usage", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/admin/usage", handleAdminUsage(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/overview", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/admin/overview", handleAdminOverview(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/users", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/admin/users", handleAdminUsers(cfg, store))
	// Admin user-management surface (admin_manage.go): single-user detail, per-user usage
	// analytics, and the account actions (set plan, revoke sessions, delete).
	mux.HandleFunc("OPTIONS /v1/admin/users/{id}", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/admin/users/{id}", handleAdminUserDetail(cfg, store))
	mux.HandleFunc("DELETE /v1/admin/users/{id}", handleAdminUserDelete(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/users/{id}/usage", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/admin/users/{id}/usage", handleAdminUserUsage(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/users/{id}/plan", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/admin/users/{id}/plan", handleAdminUserSetPlan(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/users/{id}/revoke-sessions", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/admin/users/{id}/revoke-sessions", handleAdminUserRevokeSessions(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/users/{id}/suspend", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/admin/users/{id}/suspend", handleAdminUserSuspend(cfg, store))
	mux.HandleFunc("OPTIONS /v1/admin/dev/seed", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/admin/dev/seed", handleAdminDevSeed(cfg, store))
	mux.HandleFunc("OPTIONS /v1/account/export", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/account/export", handleAccountExport(cfg, store))
	mux.HandleFunc("OPTIONS /v1/account", handleCORSPreflight(cfg))
	mux.HandleFunc("DELETE /v1/account", handleAccountDelete(cfg, store))
	mux.HandleFunc("OPTIONS /v1/devices/pair", handleCORSPreflight(cfg))
	// authLimiter here, unlike the gRPC AuthService doors, has a real client IP to key
	// on (see rateLimitClientIP) — this is a plain HTTP endpoint. Without it, an
	// authenticated caller could mint pairing codes at an unbounded rate: each mint
	// is a fresh 5-minute-lived, 6-digit account-takeover credential (see
	// pairingcode.go), so spamming this endpoint grows both storage and the
	// standing attack surface with no cost to the caller.
	mux.Handle("POST /v1/devices/pair", authLimiter(handleMintPairingCode(cfg, store)))
	mux.HandleFunc("OPTIONS /v1/billing/checkout", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/billing/checkout", handleBillingCheckout(cfg, store))
	mux.HandleFunc("OPTIONS /v1/billing/portal", handleCORSPreflight(cfg))
	mux.HandleFunc("POST /v1/billing/portal", handleBillingPortal(cfg, store))
	mux.HandleFunc("OPTIONS /v1/billing/status", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/billing/status", handleBillingStatus(cfg, store))
	mux.HandleFunc("POST /v1/billing/stripe/webhook", handleStripeWebhook(cfg, store, webhookMu))
	mux.HandleFunc("POST /v1/billing/paypal/webhook", handleProviderWebhook(cfg, store, "paypal", webhookMu))
	mux.HandleFunc("OPTIONS /v1/version", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		writeJSON(w, VersionResponse{
			APIVersion:          APIVersion,
			MinClientAPIVersion: MinClientAPIVersion,
			AuthMode:            cfg.AuthMode,
			BillingEnabled:      cfg.Billing,
			AuthProviders:       cfg.OAuthProviderNames(),
			PaymentProviders:    cfg.ConfiguredPaymentProviders(),
			CustomAuthEnabled:   true,
		})
	})
	mux.Handle("GET /v1/auth/{provider}", authLimiter(handleOAuthStart(cfg)))
	mux.Handle("GET /v1/auth/{provider}/callback", authLimiter(handleOAuthCallback(cfg, store)))
	mux.HandleFunc("OPTIONS /v1/auth/refresh", handleCORSPreflight(cfg))
	mux.Handle("POST /v1/auth/refresh", authLimiter(handleOAuthRefresh(cfg, store)))
	mux.HandleFunc("OPTIONS /v1/auth/sessions", handleCORSPreflight(cfg))
	mux.Handle("GET /v1/auth/sessions", authLimiter(handleOAuthListSessions(cfg, store)))
	mux.HandleFunc("OPTIONS /v1/auth/sessions/{family}", handleCORSPreflight(cfg))
	mux.Handle("DELETE /v1/auth/sessions/{family}", authLimiter(handleOAuthRevokeSession(cfg, store)))
	mux.HandleFunc("OPTIONS /v1/auth/logout", handleCORSPreflight(cfg))
	mux.Handle("POST /v1/auth/logout", authLimiter(handleOAuthLogout(cfg, store)))
	mux.HandleFunc("OPTIONS /v1/auth/logout-all", handleCORSPreflight(cfg))
	mux.Handle("POST /v1/auth/logout-all", authLimiter(handleOAuthLogoutAll(cfg, store)))
	mux.Handle("/grpc", NewGRPCBridgeHandler(cfg, store))
	mux.HandleFunc("OPTIONS /v1/blobs/{hash}", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /v1/blobs/{hash}", handlePutBlob(cfg, store))
	mux.HandleFunc("GET /v1/blobs/{hash}", handleGetBlob(cfg, store))
	mux.HandleFunc("HEAD /v1/blobs/{hash}", handleHeadBlob(cfg, store))
	// Operator console SPA: /console/ serves the web/admin static assets.
	// /console (no trailing slash) redirects to /console/ for clean URLs.
	// /console/devcreds is registered BEFORE the /console/ catch-all so the
	// more-specific pattern takes precedence (Go 1.22 ServeMux routing rules).
	mux.HandleFunc("GET /console", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/console/", http.StatusFound)
	})
	mux.HandleFunc("GET /console/devcreds", devCredsHandler(cfg))
	mux.Handle("GET /console/", consoleHandler(cfg))
	// Customer self-service portal SPA (Phase 4): /portal/ serves web/portal assets,
	// mirroring the operator console. /v1/me is the portal's scoped dashboard read.
	mux.HandleFunc("GET /portal", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/portal/", http.StatusFound)
	})
	mux.Handle("GET /portal/", portalHandler(cfg))
	mux.HandleFunc("OPTIONS /v1/me", handleCORSPreflight(cfg))
	mux.HandleFunc("GET /v1/me", handleMe(cfg, store))
	return maxInFlightMiddleware(cfg.HTTPMaxInFlight, securityHeadersMiddleware(requestIDMiddleware(requestLogMiddlewareSampled(cfg.Logger, cfg.Metrics, cfg.LogHotPathSampleRate, userRateLimitMiddleware(cfg.HTTPUserRateLimitPerMinute, cfg, rateLimitMiddleware(cfg.HTTPRateLimitPerMinute, cfg, mux))))))
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
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleMetrics(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := httpBearerUser(r, cfg)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
			return
		}
		// Prometheus internals (per-user usage, request/token counters, queue depth)
		// are operator-only: the endpoint used to serve them to any authenticated
		// user. A scraper authenticates with the static server token (operator) or an
		// admin user; a regular Cloud user is denied.
		if !httpOperatorAuthorized(user, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "operator access required")
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		cfg.Metrics.WritePrometheus(w)
	}
}

// httpOperatorAuthorized reports whether an authenticated request carries
// operator authority: it presents the configured static server token (whose
// holder is the operator in self-host token mode), or its user is an
// operator-designated admin (CASHFLUX_SERVER_ADMIN_USER_IDS). Used to gate the
// cross-tenant operator surfaces (metrics, the global audit log).
func httpOperatorAuthorized(user AuthUser, cfg Config) bool {
	return cfg.IsAdmin(user.ID) || cfg.matchesStaticToken(user.Token)
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
			writeErrorJSON(w, ErrorReasonServerUnavailable, "server is busy")
		}
	})
}

type rateLimitBucket struct {
	windowStart time.Time
	count       int
}

type fixedWindowLimiter struct {
	limit     int
	mu        sync.Mutex
	buckets   map[string]rateLimitBucket
	lastSweep time.Time
}

func newFixedWindowLimiter(limit int) *fixedWindowLimiter {
	return &fixedWindowLimiter{limit: limit, buckets: map[string]rateLimitBucket{}}
}

func (l *fixedWindowLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(now)
	bucket := l.buckets[key]
	if bucket.windowStart.IsZero() || now.Sub(bucket.windowStart) >= time.Minute {
		bucket = rateLimitBucket{windowStart: now}
	}
	bucket.count++
	l.buckets[key] = bucket
	return bucket.count <= l.limit
}

// sweepLocked reclaims buckets whose fixed window has fully elapsed. Without it the
// bucket map only ever grows, so a flood of distinct keys (e.g. many source IPs)
// would pin memory for the process lifetime — a slow denial-of-service. The sweep
// runs at most once per window (amortized), under the lock already held by allow,
// so per-request cost stays O(1) and the map size is bounded to the keys actually
// seen within the last minute rather than every key ever seen.
func (l *fixedWindowLimiter) sweepLocked(now time.Time) {
	if l.lastSweep.IsZero() {
		l.lastSweep = now
		return
	}
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for key, bucket := range l.buckets {
		if now.Sub(bucket.windowStart) >= time.Minute {
			delete(l.buckets, key)
		}
	}
}

func rateLimitMiddleware(limit int, cfg Config, next http.Handler) http.Handler {
	if limit <= 0 {
		return next
	}
	limiter := newFixedWindowLimiter(limit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(rateLimitClientIP(r, cfg), time.Now()) {
			w.Header().Set("Retry-After", "60")
			writeErrorJSON(w, ErrorReasonRateLimited, "rate limit exceeded")
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
			writeErrorJSON(w, ErrorReasonRateLimited, "user rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authRateLimitMiddleware(limit int, cfg Config) func(http.Handler) http.Handler {
	limiter := newFixedWindowLimiter(limit)
	return func(next http.Handler) http.Handler {
		if limit <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.allow(rateLimitClientIP(r, cfg), time.Now()) {
				w.Header().Set("Retry-After", "60")
				writeErrorJSON(w, ErrorReasonRateLimited, "auth rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP returns a best-effort source address for informational audit logging. It
// prefers forwarding headers when present. It is intentionally NOT used for any
// security decision — see rateLimitClientIP for the trusted-proxy-aware resolver
// that gates rate limiting. Audit rows also carry the authenticated actor id, which
// (unlike this header-derived IP) cannot be spoofed.
func clientIP(r *http.Request) string {
	for _, part := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		if ip := strings.TrimSpace(part); ip != "" {
			return ip
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	return remoteHost(r.RemoteAddr)
}

// remoteHost extracts the host portion of a RemoteAddr ("ip:port" → "ip"), falling
// back to the raw value, then to "unknown".
func remoteHost(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil && host != "" {
		return host
	}
	if remoteAddr != "" {
		return remoteAddr
	}
	return "unknown"
}

// ipInNets reports whether ip parses and falls inside any of the given networks.
func ipInNets(ip string, nets []*net.IPNet) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}
	for _, network := range nets {
		if network != nil && network.Contains(parsed) {
			return true
		}
	}
	return false
}

// rateLimitClientIP resolves the client key used for IP-based rate limiting. It
// trusts X-Forwarded-For / X-Real-IP ONLY when the direct socket peer (RemoteAddr)
// is a configured trusted proxy; for any other peer those headers are
// attacker-controlled, so they are ignored and the socket peer address is used. This
// closes two attacks at once: a client can no longer rotate spoofed X-Forwarded-For
// values to (a) mint unlimited fresh rate-limit buckets and bypass the limiter, or
// (b) flood the bucket map with distinct keys to exhaust memory. When the peer IS a
// trusted proxy, the real client is the rightmost forwarded address that is not
// itself a trusted proxy (i.e. the closest hop our own edge actually saw).
func rateLimitClientIP(r *http.Request, cfg Config) string {
	direct := remoteHost(r.RemoteAddr)
	if len(cfg.TrustedProxies) == 0 || !ipInNets(direct, cfg.TrustedProxies) {
		return direct
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" || net.ParseIP(ip) == nil || ipInNets(ip, cfg.TrustedProxies) {
			continue
		}
		return ip
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(xrip) != nil && !ipInNets(xrip, cfg.TrustedProxies) {
		return xrip
	}
	return direct
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
		AlertRequests:   cfg.AIAlertRequestsPerDay,
		AlertTokens:     cfg.AIAlertTokensPerDay,
		BlockedUserIDs:  cfg.AIBlockedUserIDs,
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
		writeErrorJSON(w, ErrorReasonInternal, "json encode failed")
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
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CashFlux-CSRF")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, ETag, X-CashFlux-CSRF")
	w.Header().Set("Access-Control-Max-Age", "600")
	// All-origins mode (AppOrigin="*"): NEVER reflect an arbitrary caller origin
	// together with Allow-Credentials — that combination lets any website drive
	// credentialed (cookie-bearing) requests as the victim. Emit a literal wildcard
	// and withhold credentials; token-authenticated (non-credentialed) requests still
	// work. Credential-bearing flows (the cookie refresh) require a specific origin.
	if strings.TrimSpace(cfg.AppOrigin) == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Vary", "Origin")
		return true
	}
	// A single configured origin is reflected WITH credentials so the one trusted app
	// origin can use the cookie-based session/refresh flow.
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	return true
}

func allowedOrigin(origin, configured string) bool {
	configured = strings.TrimSpace(configured)
	if configured == "*" {
		return true
	}
	if configured != "" {
		return origin == configured || sameLoopbackOrigin(origin, configured)
	}
	return isLoopbackOrigin(origin)
}

// isLoopbackOrigin reports whether origin is an http loopback origin.
func isLoopbackOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://[::1]:")
}

// sameLoopbackOrigin reports whether two origins are the same loopback origin differing only by the
// interchangeable loopback host spelling (localhost / 127.0.0.1 / [::1]), with the same scheme and
// port. It spares self-hosters the classic footgun where the app is opened on localhost but the
// backend origin is configured as 127.0.0.1 (or vice-versa) — technically distinct origins that are
// nonetheless the same machine.
func sameLoopbackOrigin(a, b string) bool {
	ua, err := url.Parse(a)
	if err != nil {
		return false
	}
	ub, err := url.Parse(b)
	if err != nil {
		return false
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) &&
		ua.Port() == ub.Port() &&
		isLoopbackHost(ua.Hostname()) && isLoopbackHost(ub.Hostname())
}
