// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMeEndpointReturnsScopedAccount(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	cfg := Config{AuthMode: "oauth", MasterKey: "0123456789abcdef0123456789abcdef", Billing: true, StripeSecretKey: "sk_test"}
	if err := store.UpsertUser(User{ID: "github:1", Provider: "github", Subject: "1", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutSubscription(Subscription{UserID: "github:1", Provider: "stripe", ProviderCustomer: "cus_1", ProviderSubscription: "sub_1", Status: "active", Plan: "personal_annual", CurrentPeriodEnd: now.Add(30 * 24 * time.Hour)}); err != nil {
		t.Fatalf("PutSubscription: %v", err)
	}
	h := NewMux(cfg, store)
	tok, err := issueSessionToken(cfg, "github:1", "access", time.Hour, now)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/me status = %d body %q", rr.Code, rr.Body.String())
	}
	var me MeResponse
	if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&me); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if me.UserID != "github:1" || me.Subscription.Status != "active" || !me.Subscription.Active || me.Subscription.Plan != "personal_annual" {
		t.Fatalf("me = %+v", me)
	}
	if !me.Billing.Enabled || len(me.Billing.PaymentProviders) == 0 {
		t.Fatalf("me billing = %+v", me.Billing)
	}

	// A different user sees their OWN (none) subscription, never github:1's.
	if err := store.UpsertUser(User{ID: "github:2", Provider: "github", Subject: "2", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser2: %v", err)
	}
	tok2, _ := issueSessionToken(cfg, "github:2", "access", time.Hour, now)
	req = httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok2)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var me2 MeResponse
	_ = json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&me2)
	if me2.Subscription.Status != "none" {
		t.Fatalf("user2 leaked another sub: %+v", me2.Subscription)
	}

	// Unauthenticated is rejected.
	req = httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth /v1/me = %d", rr.Code)
	}
}
