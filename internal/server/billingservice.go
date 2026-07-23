// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegisterBillingServiceServer registers the hand-rolled BillingService
// ServiceDesc against s, following the same JSON-codec pattern as
// SyncService/AIService/AuthService.
func RegisterBillingServiceServer(s grpc.ServiceRegistrar, srv BillingServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "cashflux.v1.BillingService",
		HandlerType: (*BillingServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "CreateCheckoutSession", Handler: billingCreateCheckoutSessionHandler},
		},
		Metadata: "cashflux/v1/cashflux.proto",
	}, srv)
}

// BillingServiceServer is the server-side contract for BillingService
// (TODOS.md C440 — replaces the REST createBillingSession call from
// internal/app/backend.go).
type BillingServiceServer interface {
	CreateCheckoutSession(context.Context, backendrpc.CreateCheckoutSessionRequest) (backendrpc.CreateCheckoutSessionResponse, error)
}

// billingServer implements BillingServiceServer by porting the existing
// REST createBillingSession logic (billing_http.go's handleBillingCheckout)
// onto this RPC shape. It deliberately does NOT retire the REST route in
// this change — see docs/CUSTOM_SYNC_TRANSPORT.md.
type billingServer struct {
	store *Store
	cfg   Config
}

func newBillingService(store *Store, cfg Config) *billingServer {
	return &billingServer{store: store, cfg: cfg}
}

// CreateCheckoutSession starts a hosted checkout session with a payment
// provider and returns the URL the client redirects the browser to
// (TODOS.md C440). req.Plan carries the same interval value the REST route's
// "interval" query/body field does ("annual" | "monthly", empty = annual);
// req.Provider carries the same optional ?provider= value (empty = stripe).
func (s *billingServer) CreateCheckoutSession(ctx context.Context, req backendrpc.CreateCheckoutSessionRequest) (backendrpc.CreateCheckoutSessionResponse, error) {
	user, ok := AuthUserFromContext(ctx)
	if !ok {
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	if !s.cfg.Billing {
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.FailedPrecondition, "billing is disabled")
	}
	if s.store == nil {
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.FailedPrecondition, "store is not configured")
	}

	provider, ok := paymentProvider(strings.TrimSpace(req.Provider))
	if !ok {
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.InvalidArgument, "unknown payment provider")
	}
	if !provider.Configured(s.cfg) {
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.FailedPrecondition, "payment provider is not configured")
	}

	// Validate the interval up front, matching handleBillingCheckout's contract:
	// a bad Stripe interval is a clean invalid-argument before any allow-check
	// or upstream call.
	if _, _, err := stripePriceForIntervalValue(s.cfg, req.Plan); err != nil && provider.Name() == "stripe" && strings.Contains(err.Error(), "interval is invalid") {
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.InvalidArgument, "billing interval is invalid")
	}

	allowed, reason, err := checkoutAllowed(s.store, user.ID)
	if err != nil {
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.Internal, "subscription lookup failed")
	}
	if !allowed {
		return backendrpc.CreateCheckoutSessionResponse{}, RichBillingLapsedError(reason)
	}

	sessionURL, _, err := provider.Checkout(ctx, s.cfg, user.ID, req.Plan)
	if err != nil {
		if isBillingConfigError(err) {
			return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.InvalidArgument, err.Error())
		}
		return backendrpc.CreateCheckoutSessionResponse{}, status.Error(codes.Unavailable, "checkout session failed")
	}

	return backendrpc.CreateCheckoutSessionResponse{CheckoutURL: sessionURL}, nil
}

func billingCreateCheckoutSessionHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.CreateCheckoutSessionRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BillingServiceServer).CreateCheckoutSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodBillingCreateCheckoutSession}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(BillingServiceServer).CreateCheckoutSession(ctx, req.(backendrpc.CreateCheckoutSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}
