// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestBearerTokenFromContext(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer abc123"))
	token, err := BearerTokenFromContext(ctx)
	if err != nil || token != "abc123" {
		t.Fatalf("BearerTokenFromContext = %q/%v, want abc123", token, err)
	}

	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing metadata", ctx: context.Background()},
		{name: "missing authorization", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("x", "y"))},
		{name: "wrong scheme", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic abc"))},
		{name: "missing token", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BearerTokenFromContext(tc.ctx); err == nil {
				t.Fatal("expected bearer parse error")
			}
		})
	}
}

func TestAuthUnaryInterceptorPutsUserInContext(t *testing.T) {
	interceptor := AuthUnaryInterceptor(func(_ context.Context, token string) (AuthUser, error) {
		if token != "token-1" {
			return AuthUser{}, errors.New("bad token")
		}
		return AuthUser{ID: "u1"}, nil
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token-1"))
	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/cashflux.Sync/List"}, func(ctx context.Context, req any) (any, error) {
		user, ok := AuthUserFromContext(ctx)
		if !ok || user.ID != "u1" || user.Token != "token-1" {
			t.Fatalf("auth user = %+v/%v", user, ok)
		}
		if req != "request" {
			t.Fatalf("request = %v", req)
		}
		return "ok", nil
	})
	if err != nil || resp != "ok" {
		t.Fatalf("interceptor = %v/%v, want ok", resp, err)
	}
}

func TestAuthUnaryInterceptorRejectsInvalidToken(t *testing.T) {
	interceptor := AuthUnaryInterceptor(func(context.Context, string) (AuthUser, error) {
		return AuthUser{}, errors.New("bad token")
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer wrong"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		t.Fatal("handler should not run")
		return nil, nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("status = %v, want unauthenticated", status.Code(err))
	}
}

func TestAuthUserForTokenAcceptsSHA256Hash(t *testing.T) {
	sum := sha256.Sum256([]byte("self-host-token"))
	cfg := Config{AuthMode: "token", TokenSHA256: hex.EncodeToString(sum[:])}
	user, ok := authUserForToken("self-host-token", cfg)
	if !ok || user.ID == "" || user.Token != "self-host-token" {
		t.Fatalf("hashed token auth = %+v/%v", user, ok)
	}
	if _, ok := authUserForToken("wrong", cfg); ok {
		t.Fatal("wrong token accepted against hash")
	}
}

// TestAuthUserForTokenAcceptsAuthServiceJWTRegardlessOfAuthMode proves
// authUserForToken accepts a valid AuthService session-JWT access token (the
// same shape Register/Login/RedeemPairingCode all mint via
// issueStoredSessionPair) under AuthMode=="token" — the default self-host
// mode, and exactly the mode "Custom Sync" (TODOS.md C418-C427) targets per
// its own "fixed, built-in server endpoint, no OAuth setup" premise (C419).
//
// Before this test's fix, this path was gated on AuthMode=="oauth" (a
// leftover from when third-party OAuth cloud sign-in was the only source of
// these JWTs): a self-hosted Custom Sync session could complete password/
// pairing sign-in and look "signed in" client-side (Register/Login/
// RedeemPairingCode/RefreshToken are all interceptor-exempt — see
// authinterceptor_skip.go) while every OTHER authenticated call it actually
// needs (ListDevices, AccountService.GetEntitlement, SyncService/
// BlobService — the whole point of syncing) silently failed Unauthenticated
// and the session eventually degraded to local-only (C427) with no visible
// error, in the server's DEFAULT configuration.
func TestAuthUserForTokenAcceptsAuthServiceJWTRegardlessOfAuthMode(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	if err := store.UpsertUser(User{ID: "u1", Provider: "local", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	for _, mode := range []string{"token", "oauth"} {
		t.Run(mode, func(t *testing.T) {
			cfg := Config{AuthMode: mode, Token: "self-host-token", SessionKey: "0123456789abcdef0123456789abcdef"}
			access, _, err := issueStoredSessionPair(cfg, store, "u1", now, "fam1", "test-device")
			if err != nil {
				t.Fatalf("issueStoredSessionPair: %v", err)
			}
			user, ok := authUserForToken(access, cfg)
			if !ok || user.ID != "u1" {
				t.Fatalf("AuthMode=%q: authUserForToken(sessionJWT) = %+v/%v, want u1/true", mode, user, ok)
			}
			// The static self-host token must keep working unaffected in either mode.
			staticUser, ok := authUserForToken("self-host-token", cfg)
			if !ok || staticUser.Token != "self-host-token" {
				t.Fatalf("AuthMode=%q: authUserForToken(staticToken) = %+v/%v, want accepted", mode, staticUser, ok)
			}
			// A garbage token must still be rejected in either mode.
			if _, ok := authUserForToken("not-a-real-token", cfg); ok {
				t.Fatalf("AuthMode=%q: garbage token was accepted", mode)
			}
		})
	}
}

func TestAuthStreamInterceptorPutsUserInContext(t *testing.T) {
	interceptor := AuthStreamInterceptor(func(_ context.Context, token string) (AuthUser, error) {
		if token != "stream-token" {
			return AuthUser{}, errors.New("bad token")
		}
		return AuthUser{ID: "u2"}, nil
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer stream-token"))
	err := interceptor(nil, fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "/cashflux.Sync/Watch"}, func(_ any, stream grpc.ServerStream) error {
		user, ok := AuthUserFromContext(stream.Context())
		if !ok || user.ID != "u2" || user.Token != "stream-token" {
			t.Fatalf("stream auth user = %+v/%v", user, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream interceptor: %v", err)
	}
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s fakeServerStream) Context() context.Context { return s.ctx }
func (fakeServerStream) RecvMsg(any) error          { return io.EOF }
func (fakeServerStream) SendMsg(any) error          { return nil }
