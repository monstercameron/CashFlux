package server

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsCloudActive is the single entitlement seam for sync/AI/blob cloud features.
// Self-host deployments run with billing disabled, which means entitlement is
// always active. Billing-enabled Cloud will later check subscription state here.
func IsCloudActive(ctx context.Context, cfg Config, user AuthUser) (bool, error) {
	if strings.TrimSpace(user.ID) == "" {
		return false, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	if !cfg.Billing {
		return true, nil
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	return false, nil
}
