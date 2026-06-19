package server

import (
	"context"
	"strings"
	"time"

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
