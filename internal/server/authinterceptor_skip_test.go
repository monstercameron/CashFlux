// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"testing"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestAuthUnaryInterceptorSkipList proves exactly which full method names run
// with no bearer token in context: the 8 AuthService enrollment/lifecycle
// methods skip the check, while ListDevices/RevokeDevice and every existing
// SyncService/AIService method still require one (TODOS.md C418).
func TestAuthUnaryInterceptorSkipList(t *testing.T) {
	failValidator := func(context.Context, string) (AuthUser, error) {
		return AuthUser{}, status.Error(codes.Unauthenticated, "no token presented")
	}
	interceptor := AuthUnaryInterceptor(failValidator)
	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		_, hasUser := AuthUserFromContext(ctx)
		return hasUser, nil
	}

	for _, tc := range []struct {
		name       string
		method     string
		wantSkip   bool
		wantErrNil bool
	}{
		{name: "enroll", method: backendrpc.MethodAuthEnroll, wantSkip: true},
		{name: "redeem pairing code", method: backendrpc.MethodAuthRedeemPairingCode, wantSkip: true},
		{name: "register", method: backendrpc.MethodAuthRegister, wantSkip: true},
		{name: "login", method: backendrpc.MethodAuthLogin, wantSkip: true},
		{name: "refresh token", method: backendrpc.MethodAuthRefreshToken, wantSkip: true},
		{name: "logout", method: backendrpc.MethodAuthLogout, wantSkip: true},
		{name: "list devices requires auth", method: backendrpc.MethodAuthListDevices, wantSkip: false},
		{name: "revoke device requires auth", method: backendrpc.MethodAuthRevokeDevice, wantSkip: false},
		{name: "sync list workspaces requires auth", method: backendrpc.MethodSyncListWorkspaces, wantSkip: false},
		{name: "ai chat requires auth", method: backendrpc.MethodAIChat, wantSkip: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled = false
			info := &grpc.UnaryServerInfo{FullMethod: tc.method}
			resp, err := interceptor(context.Background(), nil, info, handler)
			if tc.wantSkip {
				if err != nil {
					t.Fatalf("skip-listed method %q returned error: %v", tc.method, err)
				}
				if !handlerCalled {
					t.Fatalf("skip-listed method %q did not reach the handler", tc.method)
				}
				if hasUser, _ := resp.(bool); hasUser {
					t.Fatalf("skip-listed method %q unexpectedly had an AuthUser in context", tc.method)
				}
				return
			}
			if err == nil {
				t.Fatalf("gated method %q succeeded with no token, want Unauthenticated", tc.method)
			}
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("gated method %q error = %v, want Unauthenticated", tc.method, err)
			}
			if handlerCalled {
				t.Fatalf("gated method %q reached the handler with no token", tc.method)
			}
		})
	}
}

// TestAuthStreamInterceptorSkipList is the streaming-RPC equivalent of
// TestAuthUnaryInterceptorSkipList.
func TestAuthStreamInterceptorSkipList(t *testing.T) {
	failValidator := func(context.Context, string) (AuthUser, error) {
		return AuthUser{}, status.Error(codes.Unauthenticated, "no token presented")
	}
	interceptor := AuthStreamInterceptor(failValidator)

	for _, tc := range []struct {
		name     string
		method   string
		wantSkip bool
	}{
		{name: "refresh token", method: backendrpc.MethodAuthRefreshToken, wantSkip: true},
		{name: "list devices requires auth", method: backendrpc.MethodAuthListDevices, wantSkip: false},
		{name: "ai chat stream requires auth", method: backendrpc.MethodAIChatStream, wantSkip: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			var sawUser bool
			handler := func(srv any, stream grpc.ServerStream) error {
				handlerCalled = true
				_, sawUser = AuthUserFromContext(stream.Context())
				return nil
			}
			info := &grpc.StreamServerInfo{FullMethod: tc.method}
			err := interceptor(nil, fakeServerStream{ctx: context.Background()}, info, handler)
			if tc.wantSkip {
				if err != nil {
					t.Fatalf("skip-listed stream method %q returned error: %v", tc.method, err)
				}
				if !handlerCalled {
					t.Fatalf("skip-listed stream method %q did not reach the handler", tc.method)
				}
				if sawUser {
					t.Fatalf("skip-listed stream method %q unexpectedly had an AuthUser in context", tc.method)
				}
				return
			}
			if err == nil {
				t.Fatalf("gated stream method %q succeeded with no token, want Unauthenticated", tc.method)
			}
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("gated stream method %q error = %v, want Unauthenticated", tc.method, err)
			}
			if handlerCalled {
				t.Fatalf("gated stream method %q reached the handler with no token", tc.method)
			}
		})
	}
}
