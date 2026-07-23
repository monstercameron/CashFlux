// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsCloudActive is the single entitlement seam for sync/AI/blob cloud features.
// Self-host deployments run with billing disabled, which means entitlement is
// always active. Billing-enabled Cloud reads the user's current subscription row.
func IsCloudActive(ctx context.Context, cfg Config, store *Store, user AuthUser) (bool, error) {
	if strings.TrimSpace(user.ID) == "" {
		return false, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	// A suspended account is denied cloud features regardless of billing mode — this
	// is the operator's moderation lever, so it must apply even on self-host.
	if store != nil {
		if suspended, err := store.IsUserSuspended(user.ID); err != nil {
			return false, err
		} else if suspended {
			return false, nil
		}
	}
	if !cfg.Billing {
		return true, nil
	}
	if store == nil {
		return false, status.Error(codes.FailedPrecondition, "store is not configured")
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	sub, ok, err := store.GetSubscription(user.ID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return subscriptionCloudActive(sub, time.Now().UTC()), nil
}

// validSubscriptionStatus reports whether s is a status an operator may set on a
// subscription. It is the closed set the entitlement seam understands — the active
// verdict in subscriptionCloudActive keys off exactly these — so an admin cannot
// write a free-text status that silently reads as inactive everywhere.
func validSubscriptionStatus(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "active", "trialing", "past_due", "canceled", "none":
		return true
	default:
		return false
	}
}

func subscriptionCloudActive(sub Subscription, now time.Time) bool {
	status := strings.ToLower(strings.TrimSpace(sub.Status))
	switch status {
	case "active":
		return sub.CurrentPeriodEnd.IsZero() || now.Before(sub.CurrentPeriodEnd)
	case "trialing":
		if !sub.TrialEnd.IsZero() {
			return now.Before(sub.TrialEnd)
		}
		return sub.CurrentPeriodEnd.IsZero() || now.Before(sub.CurrentPeriodEnd)
	case "past_due":
		return !sub.CurrentPeriodEnd.IsZero() && now.Before(sub.CurrentPeriodEnd)
	default:
		return false
	}
}

func CloudEntitlementUnaryInterceptor(cfg Config, store *Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info != nil && skipsEntitlementCheck(info.FullMethod) {
			return handler(ctx, req)
		}
		if err := requireCloudEntitlement(ctx, cfg, store); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func CloudEntitlementStreamInterceptor(cfg Config, store *Store) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if info != nil && skipsEntitlementCheck(info.FullMethod) {
			return handler(srv, stream)
		}
		if err := requireCloudEntitlement(stream.Context(), cfg, store); err != nil {
			return err
		}
		return handler(srv, stream)
	}
}

func requireCloudEntitlement(ctx context.Context, cfg Config, store *Store) error {
	user, ok := AuthUserFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	active, err := IsCloudActive(ctx, cfg, store, user)
	if err != nil {
		return err
	}
	if !active {
		return richEntitlementRejection(ctx, store, user)
	}
	return nil
}

// richEntitlementRejection classifies WHY the caller's cloud entitlement is
// inactive and returns the matching rich error (TODOS.md C428/C433) instead
// of the bare status this used to return, so the client can render a
// suspended/billing-lapsed/no-subscription state without re-deriving it from
// a plain message string. The classification re-reads the same rows
// IsCloudActive just consulted; it does not change the active/inactive
// verdict itself, only what's told to the caller after that verdict lands.
func richEntitlementRejection(ctx context.Context, store *Store, user AuthUser) error {
	if store != nil {
		if suspended, err := store.IsUserSuspended(user.ID); err == nil && suspended {
			return RichAdminSuspendedError("this account has been suspended")
		}
		if sub, ok, err := store.GetSubscription(user.ID); err == nil && ok {
			return RichBillingLapsedError("your cloud subscription is not active (status: " + sub.Status + ")")
		}
	}
	return RichBillingLapsedError("cloud entitlement is inactive")
}
