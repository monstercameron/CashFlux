// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

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
