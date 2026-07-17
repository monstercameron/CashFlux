// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// portalHandler serves the customer self-service portal SPA from cfg.PortalDir
// under the /portal/ prefix, mirroring the operator console: existing assets are
// served directly, and unknown sub-paths fall back to index.html for SPA routing.
// The same path-traversal guard as consoleHandler applies.
func portalHandler(cfg Config) http.Handler {
	dir := cfg.PortalDir
	fs := http.FileServer(http.Dir(dir))
	return http.StripPrefix("/portal/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := filepath.Clean("/" + filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	}))
}

// MeResponse is the customer portal's one-call dashboard payload: the authenticated
// user's own subscription and today's usage. It is scoped to the caller (no admin),
// carries no other users' data, and no secrets.
type MeResponse struct {
	UserID       string          `json:"userId"`
	Subscription MeSubscription  `json:"subscription"`
	Usage        MeUsageToday    `json:"usage"`
	Billing      MeBillingConfig `json:"billing"`
}

type MeSubscription struct {
	// Status: none | trialing | active | past_due | canceled (or "disabled" when
	// billing is off — a self-host/always-on deployment).
	Status           string `json:"status"`
	Plan             string `json:"plan,omitempty"`
	Provider         string `json:"provider,omitempty"`
	CurrentPeriodEnd string `json:"currentPeriodEnd,omitempty"`
	TrialEnd         string `json:"trialEnd,omitempty"`
	// Active is the entitlement verdict the client can trust for gating.
	Active bool `json:"active"`
}

type MeUsageToday struct {
	Day      string `json:"day"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}

type MeBillingConfig struct {
	Enabled          bool     `json:"enabled"`
	PaymentProviders []string `json:"paymentProviders,omitempty"`
}

// handleMe serves GET /v1/me — the authenticated user's own account snapshot for
// the customer portal dashboard. Scoped strictly to the caller; no admin gate.
func handleMe(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		user, ok := httpBearerUser(r, cfg)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
			return
		}
		resp := MeResponse{
			UserID: user.ID,
			Billing: MeBillingConfig{
				Enabled:          cfg.Billing,
				PaymentProviders: cfg.ConfiguredPaymentProviders(),
			},
		}
		now := time.Now().UTC()
		if !cfg.Billing || store == nil {
			resp.Subscription = MeSubscription{Status: "disabled", Active: true}
		} else {
			sub, found, err := store.GetSubscription(user.ID)
			if err != nil {
				writeErrorJSON(w, ErrorReasonInternal, "subscription lookup failed")
				return
			}
			if !found {
				resp.Subscription = MeSubscription{Status: "none"}
			} else {
				resp.Subscription = MeSubscription{
					Status:           sub.Status,
					Plan:             sub.Plan,
					Provider:         sub.Provider,
					CurrentPeriodEnd: formatOptionalTime(sub.CurrentPeriodEnd),
					TrialEnd:         formatOptionalTime(sub.TrialEnd),
					Active:           subscriptionCloudActive(sub, now),
				}
			}
		}
		if store != nil {
			if usage, found, err := store.GetUsage(user.ID, now); err == nil && found {
				resp.Usage = MeUsageToday{Day: usage.Day, Requests: usage.Requests, Tokens: usage.Tokens}
			} else {
				resp.Usage = MeUsageToday{Day: usageDay(now)}
			}
		}
		writeJSON(w, resp)
	}
}
