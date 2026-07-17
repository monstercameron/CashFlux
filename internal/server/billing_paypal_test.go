// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockPayPal stands in for PayPal's REST API: it hands out an access token,
// returns a created subscription with an approval link, and verifies webhooks
// with a configurable verdict.
func mockPayPal(t *testing.T, verifyStatus string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "A21AAtok"})
	})
	mux.HandleFunc("/v1/billing/subscriptions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "I-SUB123",
			"status": "APPROVAL_PENDING",
			"links": []map[string]string{
				{"rel": "self", "href": "https://api/self"},
				{"rel": "approve", "href": "https://paypal/checkout/I-SUB123"},
			},
		})
	})
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"verification_status": verifyStatus})
	})
	return httptest.NewServer(mux)
}

func paypalTestConfig(base string) Config {
	return Config{
		PayPalClientID:     "cid",
		PayPalClientSecret: "csecret",
		PayPalAPIBaseURL:   base,
		PayPalWebhookID:    "WH-1",
		PayPalPlanAnnual:   "P-ANNUAL",
		PayPalPlanMonthly:  "P-MONTHLY",
		PayPalReturnURL:    "https://app/success",
		PayPalCancelURL:    "https://app/cancel",
	}
}

func TestPayPalCheckoutReturnsApprovalURL(t *testing.T) {
	srv := mockPayPal(t, "SUCCESS")
	defer srv.Close()
	cfg := paypalTestConfig(srv.URL)
	url, plan, err := (paypalProvider{}).Checkout(context.Background(), cfg, "u1", "annual")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if url != "https://paypal/checkout/I-SUB123" || plan != "personal_annual" {
		t.Fatalf("checkout url=%q plan=%q", url, plan)
	}
}

func TestPayPalVerifyWebhook(t *testing.T) {
	body := []byte(`{"id":"WH-EVT-1","event_type":"BILLING.SUBSCRIPTION.ACTIVATED","resource":{"id":"I-SUB123","custom_id":"u1","status":"ACTIVE","plan_id":"P-ANNUAL"}}`)
	header := http.Header{}
	for _, h := range []string{"PAYPAL-AUTH-ALGO", "PAYPAL-CERT-URL", "PAYPAL-TRANSMISSION-ID", "PAYPAL-TRANSMISSION-SIG", "PAYPAL-TRANSMISSION-TIME"} {
		header.Set(h, "x")
	}

	ok := mockPayPal(t, "SUCCESS")
	defer ok.Close()
	ev, err := (paypalProvider{}).VerifyWebhook(context.Background(), paypalTestConfig(ok.URL), header, body, time.Now())
	if err != nil {
		t.Fatalf("verify SUCCESS: %v", err)
	}
	if ev.ID != "WH-EVT-1" || ev.Type != "BILLING.SUBSCRIPTION.ACTIVATED" {
		t.Fatalf("verified event = %+v", ev)
	}

	bad := mockPayPal(t, "FAILURE")
	defer bad.Close()
	if _, err := (paypalProvider{}).VerifyWebhook(context.Background(), paypalTestConfig(bad.URL), header, body, time.Now()); err == nil {
		t.Fatal("FAILURE verdict accepted")
	}
}

func TestPayPalApplyWebhookUpsertsSubscription(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "paypal", Subject: "u1", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	raw := []byte(`{"id":"WH-EVT-1","event_type":"BILLING.SUBSCRIPTION.ACTIVATED","resource":{"id":"I-SUB123","custom_id":"u1","status":"ACTIVE","plan_id":"P-ANNUAL","billing_info":{"next_billing_time":"2026-08-01T00:00:00Z"}}}`)
	if err := (paypalProvider{}).ApplyWebhook(store, WebhookEvent{ID: "WH-EVT-1", Type: "BILLING.SUBSCRIPTION.ACTIVATED", Raw: raw}, now, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got, ok, err := store.GetSubscription("u1")
	if err != nil || !ok {
		t.Fatalf("GetSubscription = %v/%v", ok, err)
	}
	if got.Provider != "paypal" || got.ProviderSubscription != "I-SUB123" || got.Status != "active" || got.Plan != "P-ANNUAL" || got.CurrentPeriodEnd.IsZero() {
		t.Fatalf("subscription = %+v", got)
	}
	// The entitlement seam treats it as active (provider-neutral).
	if active, err := IsCloudActive(context.Background(), Config{Billing: true}, store, AuthUser{ID: "u1"}); err != nil || !active {
		t.Fatalf("entitlement active=%v err=%v", active, err)
	}
}

func TestPayPalStatusMapping(t *testing.T) {
	cases := []struct {
		event, resource, want string
		ok                    bool
	}{
		{"BILLING.SUBSCRIPTION.ACTIVATED", "", "active", true},
		{"BILLING.SUBSCRIPTION.CANCELLED", "", "canceled", true},
		{"BILLING.SUBSCRIPTION.SUSPENDED", "", "past_due", true},
		{"BILLING.SUBSCRIPTION.UPDATED", "ACTIVE", "active", true},
		{"BILLING.SUBSCRIPTION.UPDATED", "CANCELLED", "canceled", true},
		{"BILLING.SUBSCRIPTION.UPDATED", "PENDING", "", false},
		{"CUSTOMER.SOMETHING.ELSE", "", "", false},
	}
	for _, c := range cases {
		got, ok := paypalSubscriptionStatus(c.event, c.resource)
		if got != c.want || ok != c.ok {
			t.Errorf("%s/%s => %q,%v want %q,%v", c.event, c.resource, got, ok, c.want, c.ok)
		}
	}
}
