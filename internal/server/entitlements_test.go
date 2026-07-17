// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsCloudActiveWhenBillingDisabled(t *testing.T) {
	active, err := IsCloudActive(context.Background(), Config{Billing: false}, nil, AuthUser{ID: "u1"})
	if err != nil || !active {
		t.Fatalf("IsCloudActive billing disabled = %v/%v, want true nil", active, err)
	}
}

func TestIsCloudActiveRequiresUser(t *testing.T) {
	active, err := IsCloudActive(context.Background(), Config{Billing: false}, nil, AuthUser{})
	if status.Code(err) != codes.Unauthenticated || active {
		t.Fatalf("IsCloudActive missing user = %v/%v, want unauthenticated false", active, err)
	}
}

func TestIsCloudActiveBillingEnabledRequiresStore(t *testing.T) {
	active, err := IsCloudActive(context.Background(), Config{Billing: true}, nil, AuthUser{ID: "u1"})
	if status.Code(err) != codes.FailedPrecondition || active {
		t.Fatalf("IsCloudActive missing store = %v/%v, want failed precondition false", active, err)
	}
}

func TestIsCloudActiveWhenBillingEnabledDefaultsInactive(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	active, err := IsCloudActive(context.Background(), Config{Billing: true}, store, AuthUser{ID: "u1"})
	if err != nil || active {
		t.Fatalf("IsCloudActive billing enabled = %v/%v, want false nil until subscriptions land", active, err)
	}
}

func TestSubscriptionCloudActiveStates(t *testing.T) {
	now := time.Date(2026, time.June, 19, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		sub  Subscription
		want bool
	}{
		{name: "active current", sub: Subscription{Status: "active", CurrentPeriodEnd: now.Add(time.Hour)}, want: true},
		{name: "active expired", sub: Subscription{Status: "active", CurrentPeriodEnd: now.Add(-time.Hour)}, want: false},
		{name: "trialing trial", sub: Subscription{Status: "trialing", TrialEnd: now.Add(time.Hour)}, want: true},
		{name: "trialing expired", sub: Subscription{Status: "trialing", TrialEnd: now.Add(-time.Hour)}, want: false},
		{name: "past due grace", sub: Subscription{Status: "past_due", CurrentPeriodEnd: now.Add(time.Hour)}, want: true},
		{name: "canceled", sub: Subscription{Status: "canceled", CurrentPeriodEnd: now.Add(time.Hour)}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := subscriptionCloudActive(tc.sub, now); got != tc.want {
				t.Fatalf("subscriptionCloudActive(%+v) = %v, want %v", tc.sub, got, tc.want)
			}
		})
	}
}

func TestIsCloudActiveReadsSubscription(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutSubscription(Subscription{
		UserID:             "u1",
		ProviderCustomer:     "cus_active",
		ProviderSubscription: "sub_active",
		Status:             "trialing",
		Plan:               "personal_annual",
		TrialEnd:           now.Add(time.Hour),
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("PutSubscription: %v", err)
	}
	active, err := IsCloudActive(context.Background(), Config{Billing: true}, store, AuthUser{ID: "u1"})
	if err != nil || !active {
		t.Fatalf("IsCloudActive subscription = %v/%v, want true nil", active, err)
	}
}

func TestCloudEntitlementUnaryInterceptorRejectsInactiveBilling(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	interceptor := CloudEntitlementUnaryInterceptor(Config{Billing: true}, store)
	called := false
	_, err := interceptor(
		ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"}),
		struct{}{},
		&grpc.UnaryServerInfo{FullMethod: "/cashflux.v1.SyncService/ListWorkspaces"},
		func(context.Context, any) (any, error) {
			called = true
			return struct{}{}, nil
		},
	)
	if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "cloud entitlement is inactive") {
		t.Fatalf("entitlement err = %v code %v", err, status.Code(err))
	}
	if called {
		t.Fatal("handler called for inactive entitlement")
	}
}
