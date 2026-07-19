// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const stripeSignatureHeader = "Stripe-Signature"
const idempotencyKeyHeader = "Idempotency-Key"
const maxIdempotencyKeyLength = 128

type billingSessionResponse struct {
	URL string `json:"url"`
}

type checkoutRequest struct {
	Interval string `json:"interval"`
}

func handleBillingCheckout(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedBillingRequest(w, r, cfg, store)
		if !ok {
			return
		}
		var req checkoutRequest
		if !decodeOptionalJSONBody(w, r, &req, 64<<10) {
			return
		}
		provider, ok := billingProviderFromRequest(w, cfg, r)
		if !ok {
			return
		}
		// Validate the interval up front so a bad value is a clean 400 (matching the
		// prior contract) before any allow-check or upstream call.
		if _, _, err := stripePriceForIntervalValue(cfg, req.Interval); err != nil && provider.Name() == "stripe" && strings.Contains(err.Error(), "interval is invalid") {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "billing interval is invalid")
			return
		}
		if !allowBillingCheckout(w, store, user.ID) {
			return
		}
		requestHash := billingRequestHash("checkout", user.ID, provider.Name(), req.Interval)
		if replayBillingIdempotency(w, r, store, user.ID, requestHash) {
			return
		}
		sessionURL, _, err := provider.Checkout(r.Context(), cfg, user.ID, req.Interval)
		if err != nil {
			if isBillingConfigError(err) {
				writeErrorJSON(w, ErrorReasonInvalidArgument, err.Error())
			} else {
				writeErrorJSON(w, ErrorReasonUpstreamUnavailable, "checkout session failed")
			}
			return
		}
		writeBillingSession(w, r, store, user.ID, requestHash, billingSessionResponse{URL: sessionURL})
	}
}

// billingProviderFromRequest resolves the payment provider for a checkout request
// from the ?provider= query (default stripe) and confirms it is configured.
func billingProviderFromRequest(w http.ResponseWriter, cfg Config, r *http.Request) (PaymentProvider, bool) {
	provider, ok := paymentProvider(strings.TrimSpace(r.URL.Query().Get("provider")))
	if !ok {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "unknown payment provider")
		return nil, false
	}
	if !provider.Configured(cfg) {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "payment provider is not configured")
		return nil, false
	}
	return provider, true
}

// isBillingConfigError reports whether a provider error is a caller/config problem
// (bad interval, unconfigured plan) rather than an upstream/network failure, so
// the handler can return 4xx-invalid vs 502-upstream appropriately.
func isBillingConfigError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "interval is invalid") || strings.Contains(msg, "is not configured")
}

func allowBillingCheckout(w http.ResponseWriter, store *Store, userID string) bool {
	sub, ok, err := store.GetSubscription(userID)
	if err != nil {
		writeErrorJSON(w, ErrorReasonInternal, "subscription lookup failed")
		return false
	}
	if !ok {
		return true
	}
	if !sub.TrialEnd.IsZero() {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "cloud trial already used")
		return false
	}
	switch strings.TrimSpace(sub.Status) {
	case "active", "trialing", "past_due":
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "cloud subscription is already active")
		return false
	default:
		return true
	}
}

func decodeOptionalJSONBody(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) bool {
	if r.Body == nil {
		return true
	}
	if contentType := strings.TrimSpace(r.Header.Get("Content-Type")); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			writeErrorJSON(w, ErrorReasonUnsupportedMedia, "request content type must be application/json")
			return false
		}
	}
	reader := http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(reader)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeErrorJSON(w, ErrorReasonPayloadTooLarge, "request body is too large")
			return false
		}
		writeErrorJSON(w, ErrorReasonInvalidArgument, "request body must be valid JSON")
		return false
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "request body must contain a single JSON object")
		return false
	}
	return true
}

func handleBillingPortal(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedBillingRequest(w, r, cfg, store)
		if !ok {
			return
		}
		sub, ok, err := store.GetSubscription(user.ID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "subscription lookup failed")
			return
		}
		if !ok || strings.TrimSpace(sub.ProviderCustomer) == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "no active subscription to manage")
			return
		}
		// The manage/cancel surface belongs to whichever provider owns the existing
		// subscription, not a request param.
		provider, ok := paymentProvider(sub.Provider)
		if !ok || !provider.Configured(cfg) {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "payment provider is not configured")
			return
		}
		requestHash := billingRequestHash("portal", user.ID, provider.Name(), sub.ProviderCustomer)
		if replayBillingIdempotency(w, r, store, user.ID, requestHash) {
			return
		}
		sessionURL, err := provider.Portal(r.Context(), cfg, sub)
		if err != nil {
			writeErrorJSON(w, ErrorReasonUpstreamUnavailable, "portal session failed")
			return
		}
		writeBillingSession(w, r, store, user.ID, requestHash, billingSessionResponse{URL: sessionURL})
	}
}

func billingRequestHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func replayBillingIdempotency(w http.ResponseWriter, r *http.Request, store *Store, userID, requestHash string) bool {
	key := strings.TrimSpace(r.Header.Get(idempotencyKeyHeader))
	if key == "" {
		return false
	}
	if len(key) > maxIdempotencyKeyLength {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "idempotency key is too long")
		return true
	}
	result, ok, err := store.GetIdempotencyResult(userID, r.URL.Path, key)
	if err != nil {
		writeErrorJSON(w, ErrorReasonInternal, "idempotency lookup failed")
		return true
	}
	if !ok {
		return false
	}
	if result.RequestHash != requestHash {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "idempotency key was used for a different request")
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.ResponseBody)
	return true
}

func writeBillingSession(w http.ResponseWriter, r *http.Request, store *Store, userID, requestHash string, response billingSessionResponse) {
	data, err := json.Marshal(response)
	if err != nil {
		writeErrorJSON(w, ErrorReasonInternal, "encode billing response")
		return
	}
	if key := strings.TrimSpace(r.Header.Get(idempotencyKeyHeader)); key != "" && len(key) <= maxIdempotencyKeyLength {
		if err := store.PutIdempotencyResult(IdempotencyResult{
			UserID:       userID,
			Route:        r.URL.Path,
			Key:          key,
			RequestHash:  requestHash,
			ResponseBody: append(data, '\n'),
			CreatedAt:    time.Now().UTC(),
		}); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "idempotency store failed")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(data, '\n'))
}

func authorizedBillingRequest(w http.ResponseWriter, r *http.Request, cfg Config, store *Store) (AuthUser, bool) {
	if !writeCORS(w, r, cfg) {
		writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
		return AuthUser{}, false
	}
	if store == nil {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
		return AuthUser{}, false
	}
	if !cfg.Billing {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "billing is disabled")
		return AuthUser{}, false
	}
	// Provider-specific configuration is checked per-provider in
	// billingProviderFromRequest / at portal time, so this gate no longer requires
	// Stripe specifically — a PayPal-only deployment is valid.
	user, ok := httpBearerUser(r, cfg)
	if !ok {
		writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
		return AuthUser{}, false
	}
	SetLogScope(r.Context(), LogScope{UserID: user.ID})
	return user, true
}

// billingStatusResponse is the client-facing subscription snapshot returned by
// GET /v1/billing/status so the app can render trial/past-due/canceled banners and
// the graceful downgrade-to-local state (§7.11). Status is one of:
// disabled (self-host, billing off → always-on) | none | trialing | active |
// past_due | canceled.
type billingStatusResponse struct {
	Status           string `json:"status"`
	Plan             string `json:"plan,omitempty"`
	CurrentPeriodEnd string `json:"currentPeriodEnd,omitempty"`
	TrialEnd         string `json:"trialEnd,omitempty"`
}

// handleBillingStatus reports the authenticated user's subscription state. Unlike
// checkout/portal it does NOT require Stripe to be configured: a billing-disabled
// self-host returns "disabled" (always-on), and a user with no subscription
// returns "none" — so the client can branch its Cloud UI without a Stripe key.
func handleBillingStatus(cfg Config, store *Store) http.HandlerFunc {
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
		if !cfg.Billing || store == nil {
			writeJSON(w, billingStatusResponse{Status: "disabled"})
			return
		}
		sub, found, err := store.GetSubscription(user.ID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "subscription lookup failed")
			return
		}
		if !found {
			writeJSON(w, billingStatusResponse{Status: "none"})
			return
		}
		resp := billingStatusResponse{Status: sub.Status, Plan: sub.Plan}
		if !sub.CurrentPeriodEnd.IsZero() {
			resp.CurrentPeriodEnd = sub.CurrentPeriodEnd.UTC().Format(time.RFC3339)
		}
		if !sub.TrialEnd.IsZero() {
			resp.TrialEnd = sub.TrialEnd.UTC().Format(time.RFC3339)
		}
		writeJSON(w, resp)
	}
}

// stripeHTTPClient bounds every Stripe API call with an explicit timeout so a
// hung upstream can't pin a request goroutine (the shared http.DefaultClient has
// no timeout — it relied solely on the request context).
var stripeHTTPClient = &http.Client{Timeout: 20 * time.Second}

func createStripeSession(ctx context.Context, cfg Config, path string, form url.Values) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.StripeAPIBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.stripe.com/v1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.StripeSecretKey))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := stripeHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		// Surface Stripe's error message (truncated) so misconfig (bad price id, bad
		// key) is diagnosable from logs instead of a bare status code.
		return "", fmt.Errorf("stripe status %d: %s", resp.StatusCode, stripeErrorMessage(data))
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.URL) == "" {
		return "", fmt.Errorf("stripe session missing url")
	}
	return strings.TrimSpace(out.URL), nil
}

// stripeErrorMessage extracts Stripe's human-readable error message from an API
// error body ({"error":{"message":"..."}}), truncated, or falls back to a short
// raw snippet. Never returns secrets — Stripe error messages don't echo the key.
func stripeErrorMessage(body []byte) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if msg := strings.TrimSpace(parsed.Error.Message); msg != "" {
			if len(msg) > 200 {
				msg = msg[:200]
			}
			return msg
		}
	}
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 120 {
		snippet = snippet[:120]
	}
	return snippet
}

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type stripeSubscriptionObject struct {
	ID               string            `json:"id"`
	Customer         string            `json:"customer"`
	Status           string            `json:"status"`
	CurrentPeriodEnd int64             `json:"current_period_end"`
	TrialEnd         int64             `json:"trial_end"`
	Metadata         map[string]string `json:"metadata"`
	Items            struct {
		Data []struct {
			Price struct {
				ID        string `json:"id"`
				LookupKey string `json:"lookup_key"`
				Nickname  string `json:"nickname"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

type stripeCheckoutSessionObject struct {
	Customer          string            `json:"customer"`
	Subscription      string            `json:"subscription"`
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
}

type stripeInvoiceObject struct {
	Customer     string `json:"customer"`
	Subscription string `json:"subscription"`
}

func handleStripeWebhook(cfg Config, store *Store, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		if !cfg.Billing {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "billing is disabled")
			return
		}
		secret := strings.TrimSpace(cfg.StripeWebhookSecret)
		if secret == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "stripe webhook secret is not configured")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeErrorJSON(w, ErrorReasonPayloadTooLarge, "webhook body is too large")
				return
			}
			writeErrorJSON(w, ErrorReasonInvalidArgument, "webhook body is invalid")
			return
		}
		now := time.Now().UTC()
		if !validStripeSignature(r.Header.Get(stripeSignatureHeader), body, secret, now) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "stripe signature is invalid")
			return
		}
		var event stripeEvent
		if err := json.Unmarshal(body, &event); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "stripe event is invalid")
			return
		}
		// Replay dedupe MUST be atomic with apply: Stripe retries until it gets a 2xx.
		// The check-apply-record sequence runs under a lock and records the event id
		// ONLY after apply succeeds — so (a) a re-sent event is deduped and can't
		// overwrite newer state, and (b) a failed apply is never marked "seen", so
		// Stripe's retry re-applies it instead of the change being silently lost.
		mu.Lock()
		defer mu.Unlock()
		seen, err := store.HasWebhookEvent("stripe", event.ID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "webhook dedupe failed")
			return
		}
		if seen {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := applyStripeEvent(store, event, now, cfg.Metrics); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, err.Error())
			return
		}
		if _, err := store.RecordWebhookEventOnce("stripe", event.ID, now); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "webhook record failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleProviderWebhook is the provider-neutral webhook endpoint (used by PayPal;
// Stripe keeps its dedicated handler). It authenticates via the provider's own
// scheme, dedupes replays by event id, and applies the mapped subscription
// mutation. A verify failure is 403; a mapping/apply failure is 400.
func handleProviderWebhook(cfg Config, store *Store, providerName string, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		if !cfg.Billing {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "billing is disabled")
			return
		}
		provider, ok := paymentProvider(providerName)
		if !ok || !provider.Configured(cfg) {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "payment provider is not configured")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeErrorJSON(w, ErrorReasonPayloadTooLarge, "webhook body is too large")
				return
			}
			writeErrorJSON(w, ErrorReasonInvalidArgument, "webhook body is invalid")
			return
		}
		now := time.Now().UTC()
		ev, err := provider.VerifyWebhook(r.Context(), cfg, r.Header, body, now)
		if err != nil {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "webhook signature is invalid")
			return
		}
		// Atomic check-apply-record under the shared webhook lock: dedupe a replay,
		// apply, and only then mark the event seen — so a failed apply is retried by
		// the provider rather than silently swallowed. See handleStripeWebhook.
		mu.Lock()
		defer mu.Unlock()
		seen, err := store.HasWebhookEvent(provider.Name(), ev.ID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "webhook dedupe failed")
			return
		}
		if seen {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := provider.ApplyWebhook(store, ev, now, cfg.Metrics); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, err.Error())
			return
		}
		if _, err := store.RecordWebhookEventOnce(provider.Name(), ev.ID, now); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "webhook record failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func validStripeSignature(header string, body []byte, secret string, now time.Time) bool {
	var timestamp, signature string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = strings.TrimSpace(value)
		case "v1":
			signature = strings.TrimSpace(value)
		}
	}
	if timestamp == "" || signature == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	eventTime := time.Unix(ts, 0).UTC()
	if now.Sub(eventTime) > 5*time.Minute || eventTime.Sub(now) > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%s.", timestamp)
	_, _ = mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(signature))
}

func applyStripeEvent(store *Store, event stripeEvent, now time.Time, metrics *Metrics) error {
	switch strings.TrimSpace(event.Type) {
	case "checkout.session.completed":
		var session stripeCheckoutSessionObject
		if err := json.Unmarshal(event.Data.Object, &session); err != nil {
			return fmt.Errorf("stripe checkout session is invalid")
		}
		userID := metadataValue(session.Metadata, "user_id", "cashflux_user_id")
		if userID == "" {
			userID = strings.TrimSpace(session.ClientReferenceID)
		}
		if userID == "" || strings.TrimSpace(session.Customer) == "" || strings.TrimSpace(session.Subscription) == "" {
			return fmt.Errorf("stripe checkout session is missing subscription identity")
		}
		next := Subscription{
			UserID:               userID,
			ProviderCustomer:     session.Customer,
			ProviderSubscription: session.Subscription,
			Status:               metadataValueDefault(session.Metadata, "trialing", "subscription_status", "status"),
			Plan:                 metadataValueDefault(session.Metadata, "unknown", "plan", "price"),
			UpdatedAt:            now,
		}
		if err := store.PutSubscription(next); err != nil {
			return err
		}
		observeBillingTransition(metrics, event.Type, Subscription{}, next)
		return nil
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripeSubscriptionObject
		if err := json.Unmarshal(event.Data.Object, &sub); err != nil {
			return fmt.Errorf("stripe subscription is invalid")
		}
		if event.Type == "customer.subscription.deleted" {
			sub.Status = "canceled"
		}
		previous := existingSubscriptionForStripe(store, sub.ID)
		next, err := stripeSubscriptionRecord(store, sub, now)
		if err != nil {
			return err
		}
		if err := store.PutSubscription(next); err != nil {
			return err
		}
		observeBillingTransition(metrics, event.Type, previous, next)
		return nil
	case "invoice.payment_failed":
		var invoice stripeInvoiceObject
		if err := json.Unmarshal(event.Data.Object, &invoice); err != nil {
			return fmt.Errorf("stripe invoice is invalid")
		}
		existing, ok, err := store.GetSubscriptionByProviderID("stripe", invoice.Subscription)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("stripe invoice subscription is unknown")
		}
		previous := existing
		existing.Status = "past_due"
		existing.UpdatedAt = now
		if strings.TrimSpace(invoice.Customer) != "" {
			existing.ProviderCustomer = invoice.Customer
		}
		if err := store.PutSubscription(existing); err != nil {
			return err
		}
		observeBillingTransition(metrics, event.Type, previous, existing)
		return nil
	default:
		return nil
	}
}

func stripeSubscriptionRecord(store *Store, sub stripeSubscriptionObject, now time.Time) (Subscription, error) {
	userID := metadataValue(sub.Metadata, "user_id", "cashflux_user_id")
	if userID == "" {
		if existing, ok, err := store.GetSubscriptionByProviderID("stripe", sub.ID); err != nil {
			return Subscription{}, err
		} else if ok {
			userID = existing.UserID
		}
	}
	if userID == "" || strings.TrimSpace(sub.Customer) == "" || strings.TrimSpace(sub.ID) == "" ||
		strings.TrimSpace(sub.Status) == "" {
		return Subscription{}, fmt.Errorf("stripe subscription is missing required fields")
	}
	return Subscription{
		UserID:               userID,
		ProviderCustomer:     sub.Customer,
		ProviderSubscription: sub.ID,
		Status:               sub.Status,
		Plan:                 stripeSubscriptionPlan(sub),
		CurrentPeriodEnd:     unixTime(sub.CurrentPeriodEnd),
		TrialEnd:             unixTime(sub.TrialEnd),
		UpdatedAt:            now,
	}, nil
}

func stripeSubscriptionPlan(sub stripeSubscriptionObject) string {
	if plan := metadataValue(sub.Metadata, "plan", "price"); plan != "" {
		return plan
	}
	if len(sub.Items.Data) == 0 {
		return "unknown"
	}
	price := sub.Items.Data[0].Price
	for _, value := range []string{price.LookupKey, price.Nickname, price.ID} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown"
}

func existingSubscriptionForStripe(store *Store, stripeSubscription string) Subscription {
	existing, ok, err := store.GetSubscriptionByProviderID("stripe", stripeSubscription)
	if err != nil || !ok {
		return Subscription{}
	}
	return existing
}

func observeBillingTransition(metrics *Metrics, eventType string, previous, next Subscription) {
	if metrics == nil {
		return
	}
	plan := strings.TrimSpace(next.Plan)
	if plan == "" {
		plan = previous.Plan
	}
	status := strings.TrimSpace(next.Status)
	switch strings.TrimSpace(eventType) {
	case "checkout.session.completed":
		metrics.ObserveBillingEvent("signup", plan, status)
		if status == "trialing" {
			metrics.ObserveBillingEvent("trial_start", plan, status)
		}
	case "customer.subscription.updated":
		if !billableSubscriptionStatus(previous.Status) && billableSubscriptionStatus(next.Status) {
			metrics.ObserveBillingEvent("conversion", plan, status)
		}
	case "customer.subscription.deleted":
		metrics.ObserveBillingEvent("cancellation", plan, status)
	case "invoice.payment_failed":
		metrics.ObserveBillingEvent("payment_failed", plan, status)
	}
	metrics.ObserveBillingMRRDelta(subscriptionMRRCents(next) - subscriptionMRRCents(previous))
}

func subscriptionMRRCents(sub Subscription) int64 {
	if !billableSubscriptionStatus(sub.Status) {
		return 0
	}
	switch strings.TrimSpace(sub.Plan) {
	case "personal_monthly":
		return 399
	case "personal_annual":
		return 292
	default:
		return 0
	}
}

func billableSubscriptionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "active", "past_due":
		return true
	default:
		return false
	}
}

func metadataValue(metadata map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func metadataValueDefault(metadata map[string]string, fallback string, keys ...string) string {
	if value := metadataValue(metadata, keys...); value != "" {
		return value
	}
	return fallback
}

func unixTime(seconds int64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}
