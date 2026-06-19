package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const stripeSignatureHeader = "Stripe-Signature"

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
		if r.Body != nil {
			_ = json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req)
		}
		price, plan, ok := stripePriceForInterval(w, cfg, req.Interval)
		if !ok {
			return
		}
		if !allowBillingCheckout(w, store, user.ID) {
			return
		}
		form := url.Values{}
		form.Set("mode", "subscription")
		form.Set("success_url", strings.TrimSpace(cfg.StripeSuccessURL))
		form.Set("cancel_url", strings.TrimSpace(cfg.StripeCancelURL))
		form.Set("client_reference_id", user.ID)
		form.Set("line_items[0][price]", price)
		form.Set("line_items[0][quantity]", "1")
		form.Set("metadata[user_id]", user.ID)
		form.Set("metadata[plan]", plan)
		form.Set("subscription_data[metadata][user_id]", user.ID)
		form.Set("subscription_data[metadata][plan]", plan)
		form.Set("allow_promotion_codes", "true")
		sessionURL, err := createStripeSession(r, cfg, "/checkout/sessions", form)
		if err != nil {
			writeErrorJSON(w, ErrorReasonUpstreamUnavailable, "stripe checkout session failed")
			return
		}
		writeJSON(w, billingSessionResponse{URL: sessionURL})
	}
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
		if !ok || strings.TrimSpace(sub.StripeCustomer) == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "stripe customer is not configured")
			return
		}
		form := url.Values{}
		form.Set("customer", sub.StripeCustomer)
		form.Set("return_url", strings.TrimSpace(cfg.StripePortalReturnURL))
		sessionURL, err := createStripeSession(r, cfg, "/billing_portal/sessions", form)
		if err != nil {
			writeErrorJSON(w, ErrorReasonUpstreamUnavailable, "stripe portal session failed")
			return
		}
		writeJSON(w, billingSessionResponse{URL: sessionURL})
	}
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
	if strings.TrimSpace(cfg.StripeSecretKey) == "" {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "stripe secret key is not configured")
		return AuthUser{}, false
	}
	user, ok := httpBearerUser(r, cfg)
	if !ok {
		writeErrorJSON(w, ErrorReasonUnauthenticated, "missing bearer token")
		return AuthUser{}, false
	}
	SetLogScope(r.Context(), LogScope{UserID: user.ID})
	return user, true
}

func stripePriceForInterval(w http.ResponseWriter, cfg Config, interval string) (string, string, bool) {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "", "annual", "yearly":
		price := strings.TrimSpace(cfg.StripePriceAnnual)
		if price == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "annual stripe price is not configured")
			return "", "", false
		}
		return price, "personal_annual", true
	case "monthly":
		price := strings.TrimSpace(cfg.StripePriceMonthly)
		if price == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "monthly stripe price is not configured")
			return "", "", false
		}
		return price, "personal_monthly", true
	default:
		writeErrorJSON(w, ErrorReasonInvalidArgument, "billing interval is invalid")
		return "", "", false
	}
}

func createStripeSession(r *http.Request, cfg Config, path string, form url.Values) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.StripeAPIBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.stripe.com/v1"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.StripeSecretKey))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("stripe status %d", resp.StatusCode)
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

type stripeEvent struct {
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

func handleStripeWebhook(cfg Config, store *Store) http.HandlerFunc {
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
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "webhook body is invalid")
			return
		}
		if !validStripeSignature(r.Header.Get(stripeSignatureHeader), body, secret, time.Now().UTC()) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "stripe signature is invalid")
			return
		}
		var event stripeEvent
		if err := json.Unmarshal(body, &event); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "stripe event is invalid")
			return
		}
		if err := applyStripeEvent(store, event, time.Now().UTC(), cfg.Metrics); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, err.Error())
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
			UserID:             userID,
			StripeCustomer:     session.Customer,
			StripeSubscription: session.Subscription,
			Status:             metadataValueDefault(session.Metadata, "trialing", "subscription_status", "status"),
			Plan:               metadataValueDefault(session.Metadata, "unknown", "plan", "price"),
			UpdatedAt:          now,
		}
		if err := store.PutSubscription(next); err != nil {
			return err
		}
		observeBillingTransition(metrics, event.Type, Subscription{}, next)
		return nil
	case "customer.subscription.updated", "customer.subscription.deleted":
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
		existing, ok, err := store.GetSubscriptionByStripeID(invoice.Subscription)
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
			existing.StripeCustomer = invoice.Customer
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

func putStripeSubscription(store *Store, sub stripeSubscriptionObject, now time.Time) error {
	record, err := stripeSubscriptionRecord(store, sub, now)
	if err != nil {
		return err
	}
	return store.PutSubscription(record)
}

func stripeSubscriptionRecord(store *Store, sub stripeSubscriptionObject, now time.Time) (Subscription, error) {
	userID := metadataValue(sub.Metadata, "user_id", "cashflux_user_id")
	if userID == "" {
		if existing, ok, err := store.GetSubscriptionByStripeID(sub.ID); err != nil {
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
		UserID:             userID,
		StripeCustomer:     sub.Customer,
		StripeSubscription: sub.ID,
		Status:             sub.Status,
		Plan:               stripeSubscriptionPlan(sub),
		CurrentPeriodEnd:   unixTime(sub.CurrentPeriodEnd),
		TrialEnd:           unixTime(sub.TrialEnd),
		UpdatedAt:          now,
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
	existing, ok, err := store.GetSubscriptionByStripeID(stripeSubscription)
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
