// SPDX-License-Identifier: MIT

package server

import (
	"context"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegisterAccountServiceServer registers the hand-rolled AccountService
// ServiceDesc against s, following the same JSON-codec pattern as
// SyncService/AIService/AuthService.
func RegisterAccountServiceServer(s grpc.ServiceRegistrar, srv AccountServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "cashflux.v1.AccountService",
		HandlerType: (*AccountServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "GetEntitlement", Handler: accountGetEntitlementHandler},
		},
		Metadata: "cashflux/v1/cashflux.proto",
	}, srv)
}

// AccountServiceServer is the server-side contract for AccountService
// (TODOS.md C431).
type AccountServiceServer interface {
	GetEntitlement(context.Context, backendrpc.GetEntitlementRequest) (backendrpc.GetEntitlementResponse, error)
}

// accountServer implements AccountServiceServer. GetEntitlement is a thin
// read of the entitlement logic that already exists and is already wired
// ahead of every Sync/AI call — see IsCloudActive/subscriptionCloudActive in
// entitlements.go.
type accountServer struct {
	store *Store
	cfg   Config
}

func newAccountService(store *Store, cfg Config) *accountServer {
	return &accountServer{store: store, cfg: cfg}
}

// GetEntitlement reports whether the caller's account may use cloud sync
// right now, and why not if it can't, plus the storage-quota figures a
// Custom Sync toggle screen needs before enrolling (TODOS.md C431). This is
// the pre-flight check the client calls BEFORE any enrollment RPC, so it
// never itself rejects on an inactive entitlement — Active=false with a
// Reason is the whole point of the call.
func (s *accountServer) GetEntitlement(ctx context.Context, _ backendrpc.GetEntitlementRequest) (backendrpc.GetEntitlementResponse, error) {
	user, ok := AuthUserFromContext(ctx)
	if !ok {
		return backendrpc.GetEntitlementResponse{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}

	active, err := IsCloudActive(ctx, s.cfg, s.store, user)
	if err != nil {
		return backendrpc.GetEntitlementResponse{}, err
	}

	resp := backendrpc.GetEntitlementResponse{Active: active}
	if active {
		resp.Reason = backendrpc.EntitlementReasonOK
	} else {
		resp.Reason = s.inactiveReason(user)
	}

	if s.store != nil {
		if used, err := s.store.UserBlobBytes(user.ID); err == nil {
			resp.BytesUsed = used
		}
	}

	plan := ""
	if s.store != nil {
		if sub, ok, err := s.store.GetSubscription(user.ID); err == nil && ok {
			plan = sub.Plan
		}
	}
	resp.PlanTier = plan
	resp.BytesLimit = s.cfg.StorageLimitForPlan(plan)

	return resp, nil
}

// inactiveReason classifies why the caller's entitlement is inactive, reusing
// the same precedence IsCloudActive checks (suspension first, then
// subscription state) so the client-facing reason string matches the verdict
// IsCloudActive already computed for this same request.
func (s *accountServer) inactiveReason(user AuthUser) string {
	if s.store != nil {
		if suspended, err := s.store.IsUserSuspended(user.ID); err == nil && suspended {
			return backendrpc.EntitlementReasonAdminSuspended
		}
	}
	if !s.cfg.Billing {
		// Billing disabled but still inactive only happens via suspension, handled
		// above; this is defensive and should be unreachable in practice.
		return backendrpc.EntitlementReasonBillingLapsed
	}
	if s.store != nil {
		if _, ok, err := s.store.GetSubscription(user.ID); err == nil && !ok {
			return backendrpc.EntitlementReasonPlanTierInsufficient
		}
	}
	return backendrpc.EntitlementReasonBillingLapsed
}

func accountGetEntitlementHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.GetEntitlementRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AccountServiceServer).GetEntitlement(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodAccountGetEntitlement}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AccountServiceServer).GetEntitlement(ctx, req.(backendrpc.GetEntitlementRequest))
	}
	return interceptor(ctx, in, info, handler)
}
