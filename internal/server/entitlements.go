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
		if err := requireCloudEntitlement(ctx, cfg, store); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func CloudEntitlementStreamInterceptor(cfg Config, store *Store) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
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
		return status.Error(codes.PermissionDenied, "cloud entitlement is inactive")
	}
	return nil
}
