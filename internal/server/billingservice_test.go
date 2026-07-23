// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestBillingServiceCreateCheckoutSessionHappyPath covers TODOS.md C440: a
// configured, unconfigured-competitor-provider-free Stripe checkout for a
// user with no existing subscription returns the mocked session URL.
func TestBillingServiceCreateCheckoutSessionHappyPath(t *testing.T) {
	stripeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://checkout.stripe.test/session/abc"}`))
	}))
	defer stripeServer.Close()

	store := openTestStore(t)
	cfg := Config{
		Billing:            true,
		StripeAPIBaseURL:   stripeServer.URL,
		StripeSecretKey:    "sk_test_123",
		StripePriceAnnual:  "price_annual",
		StripePriceMonthly: "price_monthly",
	}
	svc := newBillingService(store, cfg)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "user-1"})

	resp, err := svc.CreateCheckoutSession(ctx, backendrpc.CreateCheckoutSessionRequest{Plan: "annual"})
	if err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if resp.CheckoutURL != "https://checkout.stripe.test/session/abc" {
		t.Fatalf("CheckoutURL = %q, want the mocked Stripe session URL", resp.CheckoutURL)
	}
}

func TestBillingServiceCreateCheckoutSessionRejectsAlreadyActive(t *testing.T) {
	store := openTestStore(t)
	if err := store.UpsertUser(User{ID: "user-1", Provider: "github", Subject: "user-1", Email: "u@example.com"}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutSubscription(Subscription{
		UserID: "user-1", ProviderCustomer: "cus_1", ProviderSubscription: "sub_1",
		Status: "active", Plan: "personal_monthly",
	}); err != nil {
		t.Fatalf("PutSubscription: %v", err)
	}
	cfg := Config{Billing: true, StripeSecretKey: "sk_test_123", StripePriceAnnual: "price_annual"}
	svc := newBillingService(store, cfg)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "user-1"})

	_, err := svc.CreateCheckoutSession(ctx, backendrpc.CreateCheckoutSessionRequest{Plan: "annual"})
	if err == nil {
		t.Fatal("expected an error for a user with an already-active subscription")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("status code = %v, want PermissionDenied (rich billing-lapsed error)", got)
	}
}

func TestBillingServiceCreateCheckoutSessionRequiresBillingEnabled(t *testing.T) {
	store := openTestStore(t)
	svc := newBillingService(store, Config{Billing: false})
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "user-1"})
	if _, err := svc.CreateCheckoutSession(ctx, backendrpc.CreateCheckoutSessionRequest{}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestBillingServiceCreateCheckoutSessionRequiresAuth(t *testing.T) {
	store := openTestStore(t)
	svc := newBillingService(store, Config{Billing: true})
	if _, err := svc.CreateCheckoutSession(context.Background(), backendrpc.CreateCheckoutSessionRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestBillingServiceCreateCheckoutSessionInvalidInterval(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{Billing: true, StripeSecretKey: "sk_test_123", StripePriceAnnual: "price_annual"}
	svc := newBillingService(store, cfg)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "user-1"})
	_, err := svc.CreateCheckoutSession(ctx, backendrpc.CreateCheckoutSessionRequest{Plan: "weekly"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for a bad interval, got %v", err)
	}
}
