package server

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthUser is the authenticated user carried through service contexts.
type AuthUser struct {
	ID    string
	Token string
}

// TokenValidator resolves a bearer token to an authenticated user.
type TokenValidator func(context.Context, string) (AuthUser, error)

type authUserContextKey struct{}

// ContextWithAuthUser stores the authenticated user in a context.
func ContextWithAuthUser(ctx context.Context, user AuthUser) context.Context {
	return context.WithValue(ctx, authUserContextKey{}, user)
}

// AuthUserFromContext returns the authenticated user from a service context.
func AuthUserFromContext(ctx context.Context) (AuthUser, bool) {
	user, ok := ctx.Value(authUserContextKey{}).(AuthUser)
	return user, ok
}

// BearerTokenFromContext extracts an Authorization: Bearer token from gRPC metadata.
func BearerTokenFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", fmt.Errorf("missing authorization metadata")
	}
	fields := strings.Fields(values[0])
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") || strings.TrimSpace(fields[1]) == "" {
		return "", fmt.Errorf("malformed bearer token")
	}
	return fields[1], nil
}

// AuthUnaryInterceptor validates bearer metadata and puts the user in the RPC context.
func AuthUnaryInterceptor(validate TokenValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		user, err := authenticateContext(ctx, validate)
		if err != nil {
			return nil, err
		}
		return handler(ContextWithAuthUser(ctx, user), req)
	}
}

// AuthStreamInterceptor validates bearer metadata and puts the user in the stream context.
func AuthStreamInterceptor(validate TokenValidator) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		user, err := authenticateContext(stream.Context(), validate)
		if err != nil {
			return err
		}
		return handler(srv, authServerStream{ServerStream: stream, ctx: ContextWithAuthUser(stream.Context(), user)})
	}
}

func authenticateContext(ctx context.Context, validate TokenValidator) (AuthUser, error) {
	if validate == nil {
		return AuthUser{}, status.Error(codes.Unauthenticated, "auth validator is not configured")
	}
	token, err := BearerTokenFromContext(ctx)
	if err != nil {
		return AuthUser{}, status.Error(codes.Unauthenticated, err.Error())
	}
	user, err := validate(ctx, token)
	if err != nil {
		return AuthUser{}, status.Error(codes.Unauthenticated, "invalid bearer token")
	}
	if strings.TrimSpace(user.ID) == "" {
		return AuthUser{}, status.Error(codes.Unauthenticated, "invalid bearer token")
	}
	user.Token = token
	return user, nil
}

type authServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s authServerStream) Context() context.Context { return s.ctx }
