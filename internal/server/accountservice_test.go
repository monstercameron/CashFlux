// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestAccountServiceGetEntitlement covers TODOS.md C428/C431: active, lapsed
// (no subscription), suspended, and tier-insufficient (canceled subscription)
// accounts each get the right Active/Reason/quota figures.
func TestAccountServiceGetEntitlement(t *testing.T) {
	now := time.Now().UTC()

	cases := []struct {
		name       string
		billing    bool
		suspend    bool
		sub        *Subscription
		wantActive bool
		wantReason string
	}{
		{
			name:       "self-host, billing disabled, always active",
			billing:    false,
			wantActive: true,
			wantReason: backendrpc.EntitlementReasonOK,
		},
		{
			name:       "active subscription",
			billing:    true,
			sub:        &Subscription{Status: "active", Plan: "personal_monthly", CurrentPeriodEnd: now.Add(24 * time.Hour)},
			wantActive: true,
			wantReason: backendrpc.EntitlementReasonOK,
		},
		{
			name:       "lapsed: no subscription on record",
			billing:    true,
			wantActive: false,
			wantReason: backendrpc.EntitlementReasonPlanTierInsufficient,
		},
		{
			name:       "lapsed: canceled subscription",
			billing:    true,
			sub:        &Subscription{Status: "canceled", Plan: "personal_monthly"},
			wantActive: false,
			wantReason: backendrpc.EntitlementReasonBillingLapsed,
		},
		{
			name:       "suspended account wins over an otherwise-active subscription",
			billing:    true,
			suspend:    true,
			sub:        &Subscription{Status: "active", Plan: "personal_annual", CurrentPeriodEnd: now.Add(24 * time.Hour)},
			wantActive: false,
			wantReason: backendrpc.EntitlementReasonAdminSuspended,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := openTestStore(t)
			userID := "user-1"
			if err := store.UpsertUser(User{ID: userID, Provider: "github", Subject: userID, Email: "u@example.com", CreatedAt: now}); err != nil {
				t.Fatalf("UpsertUser: %v", err)
			}
			if tc.sub != nil {
				sub := *tc.sub
				sub.UserID = userID
				sub.ProviderCustomer = "cus_" + userID
				sub.ProviderSubscription = "sub_" + userID
				sub.UpdatedAt = now
				if err := store.PutSubscription(sub); err != nil {
					t.Fatalf("PutSubscription: %v", err)
				}
			}
			if tc.suspend {
				if err := store.SetUserSuspended(userID, true, now); err != nil {
					t.Fatalf("SetUserSuspended: %v", err)
				}
			}

			cfg := Config{Billing: tc.billing, StorageMaxBytes: 1 << 30}
			svc := newAccountService(store, cfg)
			ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})

			resp, err := svc.GetEntitlement(ctx, backendrpc.GetEntitlementRequest{})
			if err != nil {
				t.Fatalf("GetEntitlement: %v", err)
			}
			if resp.Active != tc.wantActive {
				t.Errorf("Active = %v, want %v", resp.Active, tc.wantActive)
			}
			if resp.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", resp.Reason, tc.wantReason)
			}
			if resp.BytesLimit != cfg.StorageMaxBytes {
				t.Errorf("BytesLimit = %d, want %d", resp.BytesLimit, cfg.StorageMaxBytes)
			}
		})
	}
}

func TestAccountServiceGetEntitlementRequiresAuth(t *testing.T) {
	store := openTestStore(t)
	svc := newAccountService(store, Config{})
	if _, err := svc.GetEntitlement(context.Background(), backendrpc.GetEntitlementRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

// TestAccountServiceGetEntitlementPlanStorageOverride covers the per-plan
// storage limit override (TODOS.md C431/C439).
func TestAccountServiceGetEntitlementPlanStorageOverride(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	userID := "user-2"
	if err := store.UpsertUser(User{ID: userID, Provider: "github", Subject: userID, Email: "u2@example.com", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutSubscription(Subscription{
		UserID: userID, ProviderCustomer: "cus_" + userID, ProviderSubscription: "sub_" + userID,
		Status: "active", Plan: "personal_annual", CurrentPeriodEnd: now.Add(24 * time.Hour), UpdatedAt: now,
	}); err != nil {
		t.Fatalf("PutSubscription: %v", err)
	}
	cfg := Config{
		Billing:                  true,
		StorageMaxBytes:          1 << 20,
		StoragePlanBytesOverride: map[string]int64{"personal_annual": 5 << 20},
	}
	svc := newAccountService(store, cfg)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})
	resp, err := svc.GetEntitlement(ctx, backendrpc.GetEntitlementRequest{})
	if err != nil {
		t.Fatalf("GetEntitlement: %v", err)
	}
	if resp.BytesLimit != 5<<20 {
		t.Fatalf("BytesLimit = %d, want plan override %d", resp.BytesLimit, 5<<20)
	}
	if resp.PlanTier != "personal_annual" {
		t.Fatalf("PlanTier = %q, want personal_annual", resp.PlanTier)
	}
}
