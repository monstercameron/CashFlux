// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// paypalProvider implements PaymentProvider against PayPal's Subscriptions API v2.
//
// Flow: Checkout mints an OAuth token, creates a subscription (APPROVAL_PENDING),
// and returns the payer approval URL; the subscription becomes real when PayPal
// posts BILLING.SUBSCRIPTION.ACTIVATED to the webhook (custom_id carries our user
// id). Webhooks are authenticated by calling PayPal's verify-webhook-signature
// API with the configured webhook id and the transmission headers. PayPal has no
// per-subscription hosted portal, so Portal returns the payer's auto-payments page.
type paypalProvider struct{}

var paypalHTTPClient = &http.Client{Timeout: 20 * time.Second}

func (paypalProvider) Name() string { return "paypal" }

func (paypalProvider) Configured(cfg Config) bool {
	return strings.TrimSpace(cfg.PayPalClientID) != "" &&
		strings.TrimSpace(cfg.PayPalClientSecret) != ""
}

// paypalAPIBase returns the configured PayPal REST base, defaulting to SANDBOX so
// a misconfigured deployment can't accidentally transact against live PayPal.
func paypalAPIBase(cfg Config) string {
	if b := strings.TrimRight(strings.TrimSpace(cfg.PayPalAPIBaseURL), "/"); b != "" {
		return b
	}
	return "https://api-m.sandbox.paypal.com"
}

// paypalManageURL maps the API base to the payer-facing subscription management
// page (auto-payments), so Portal can hand the user somewhere to cancel/manage.
func paypalManageURL(cfg Config) string {
	if strings.Contains(paypalAPIBase(cfg), "sandbox") {
		return "https://www.sandbox.paypal.com/myaccount/autopay/"
	}
	return "https://www.paypal.com/myaccount/autopay/"
}

func paypalPlanForInterval(cfg Config, interval string) (planID, plan string, err error) {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "", "annual", "yearly":
		if id := strings.TrimSpace(cfg.PayPalPlanAnnual); id != "" {
			return id, "personal_annual", nil
		}
		return "", "", fmt.Errorf("annual paypal plan is not configured")
	case "monthly":
		if id := strings.TrimSpace(cfg.PayPalPlanMonthly); id != "" {
			return id, "personal_monthly", nil
		}
		return "", "", fmt.Errorf("monthly paypal plan is not configured")
	default:
		return "", "", fmt.Errorf("billing interval is invalid")
	}
}

// paypalAccessToken fetches an OAuth2 client-credentials access token.
func paypalAccessToken(ctx context.Context, cfg Config) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, paypalAPIBase(cfg)+"/v1/oauth2/token",
		strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}
	basic := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(cfg.PayPalClientID) + ":" + strings.TrimSpace(cfg.PayPalClientSecret)))
	req.Header.Set("Authorization", "Basic "+basic)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := paypalHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("paypal oauth status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return "", fmt.Errorf("paypal oauth returned no access token")
	}
	return out.AccessToken, nil
}

// paypalDoWithConfig performs an authenticated PayPal JSON API call and decodes
// the response, returning the HTTP status and (on 4xx/5xx) a truncated error body.
func paypalDoWithConfig(ctx context.Context, cfg Config, token, method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, paypalAPIBase(cfg)+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := paypalHTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		snippet := strings.TrimSpace(string(data))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return resp.StatusCode, fmt.Errorf("paypal status %d: %s", resp.StatusCode, snippet)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func (paypalProvider) Checkout(ctx context.Context, cfg Config, userID, interval string) (string, string, error) {
	planID, plan, err := paypalPlanForInterval(cfg, interval)
	if err != nil {
		return "", "", err
	}
	token, err := paypalAccessToken(ctx, cfg)
	if err != nil {
		return "", "", err
	}
	reqBody := map[string]any{
		"plan_id":   planID,
		"custom_id": userID,
		"application_context": map[string]any{
			"return_url":          strings.TrimSpace(cfg.PayPalReturnURL),
			"cancel_url":          strings.TrimSpace(cfg.PayPalCancelURL),
			"user_action":         "SUBSCRIBE_NOW",
			"shipping_preference": "NO_SHIPPING",
		},
	}
	var out struct {
		ID    string `json:"id"`
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if _, err := paypalDoWithConfig(ctx, cfg, token, http.MethodPost, "/v1/billing/subscriptions", reqBody, &out); err != nil {
		return "", "", err
	}
	for _, l := range out.Links {
		if strings.EqualFold(l.Rel, "approve") && strings.TrimSpace(l.Href) != "" {
			return l.Href, plan, nil
		}
	}
	return "", "", fmt.Errorf("paypal subscription is missing an approval link")
}

func (paypalProvider) Portal(_ context.Context, cfg Config, _ Subscription) (string, error) {
	return paypalManageURL(cfg), nil
}

// paypalWebhookEvent is the shape of a PayPal webhook notification we care about.
type paypalWebhookEvent struct {
	ID           string `json:"id"`
	EventType    string `json:"event_type"`
	ResourceType string `json:"resource_type"`
	Resource     struct {
		ID          string `json:"id"`
		CustomID    string `json:"custom_id"`
		Status      string `json:"status"`
		PlanID      string `json:"plan_id"`
		BillingInfo struct {
			NextBillingTime string `json:"next_billing_time"`
		} `json:"billing_info"`
		// PAYMENT.SALE.* events nest the subscription id under billing_agreement_id.
		BillingAgreementID string `json:"billing_agreement_id"`
	} `json:"resource"`
}

func (paypalProvider) VerifyWebhook(ctx context.Context, cfg Config, header http.Header, body []byte, _ time.Time) (WebhookEvent, error) {
	webhookID := strings.TrimSpace(cfg.PayPalWebhookID)
	if webhookID == "" {
		return WebhookEvent{}, fmt.Errorf("paypal webhook id is not configured")
	}
	token, err := paypalAccessToken(ctx, cfg)
	if err != nil {
		return WebhookEvent{}, err
	}
	// PayPal authenticates a webhook by echoing the transmission headers + the raw
	// event back to its verify API, which checks the signature against the cert.
	verifyReq := map[string]any{
		"auth_algo":         header.Get("PAYPAL-AUTH-ALGO"),
		"cert_url":          header.Get("PAYPAL-CERT-URL"),
		"transmission_id":   header.Get("PAYPAL-TRANSMISSION-ID"),
		"transmission_sig":  header.Get("PAYPAL-TRANSMISSION-SIG"),
		"transmission_time": header.Get("PAYPAL-TRANSMISSION-TIME"),
		"webhook_id":        webhookID,
		"webhook_event":     json.RawMessage(body),
	}
	var verifyOut struct {
		VerificationStatus string `json:"verification_status"`
	}
	if _, err := paypalDoWithConfig(ctx, cfg, token, http.MethodPost, "/v1/notifications/verify-webhook-signature", verifyReq, &verifyOut); err != nil {
		return WebhookEvent{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(verifyOut.VerificationStatus), "SUCCESS") {
		return WebhookEvent{}, fmt.Errorf("paypal webhook signature is invalid")
	}
	var ev paypalWebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return WebhookEvent{}, fmt.Errorf("paypal event is invalid")
	}
	return WebhookEvent{ID: strings.TrimSpace(ev.ID), Type: strings.TrimSpace(ev.EventType), Raw: body}, nil
}

func (paypalProvider) ApplyWebhook(store *Store, ev WebhookEvent, now time.Time, metrics *Metrics) error {
	var event paypalWebhookEvent
	if err := json.Unmarshal(ev.Raw, &event); err != nil {
		return fmt.Errorf("paypal event is invalid")
	}
	status, ok := paypalSubscriptionStatus(event.EventType, event.Resource.Status)
	if !ok {
		return nil // an event we don't map (e.g. payment metadata) — accept, no-op
	}
	subscriptionID := strings.TrimSpace(event.Resource.ID)
	if subscriptionID == "" {
		subscriptionID = strings.TrimSpace(event.Resource.BillingAgreementID)
	}
	if subscriptionID == "" {
		return fmt.Errorf("paypal event is missing a subscription id")
	}
	userID := strings.TrimSpace(event.Resource.CustomID)
	previous, hadPrevious, err := store.GetSubscriptionByProviderID("paypal", subscriptionID)
	if err != nil {
		return err
	}
	if userID == "" && hadPrevious {
		userID = previous.UserID
	}
	if userID == "" {
		return fmt.Errorf("paypal event is missing a user id")
	}
	plan := paypalPlanName(event.Resource.PlanID, previous.Plan)
	next := Subscription{
		UserID:               userID,
		Provider:             "paypal",
		ProviderCustomer:     userID, // PayPal has no separate customer id; the payer maps to our user
		ProviderSubscription: subscriptionID,
		Status:               status,
		Plan:                 plan,
		CurrentPeriodEnd:     paypalTime(event.Resource.BillingInfo.NextBillingTime, previous.CurrentPeriodEnd),
		TrialEnd:             previous.TrialEnd,
		UpdatedAt:            now,
	}
	if err := store.PutSubscription(next); err != nil {
		return err
	}
	observeBillingTransition(metrics, event.EventType, previous, next)
	return nil
}

// paypalSubscriptionStatus maps a PayPal subscription webhook to our status. ok is
// false for events that carry no subscription-status meaning (ignored no-op).
func paypalSubscriptionStatus(eventType, resourceStatus string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(eventType)) {
	case "BILLING.SUBSCRIPTION.ACTIVATED", "PAYMENT.SALE.COMPLETED":
		return "active", true
	case "BILLING.SUBSCRIPTION.CANCELLED", "BILLING.SUBSCRIPTION.EXPIRED":
		return "canceled", true
	case "BILLING.SUBSCRIPTION.SUSPENDED", "PAYMENT.SALE.DENIED":
		return "past_due", true
	case "BILLING.SUBSCRIPTION.CREATED":
		return "trialing", true
	case "BILLING.SUBSCRIPTION.UPDATED":
		// Trust the resource status on a generic update.
		switch strings.ToUpper(strings.TrimSpace(resourceStatus)) {
		case "ACTIVE":
			return "active", true
		case "SUSPENDED":
			return "past_due", true
		case "CANCELLED", "EXPIRED":
			return "canceled", true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func paypalPlanName(planID, fallback string) string {
	if p := strings.TrimSpace(planID); p != "" {
		return p
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "unknown"
}

func paypalTime(value string, fallback time.Time) time.Time {
	if v := strings.TrimSpace(value); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC()
		}
	}
	return fallback
}
