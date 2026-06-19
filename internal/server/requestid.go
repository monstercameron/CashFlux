package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

func ContextWithRequestID(ctx context.Context, id string) context.Context {
	id = strings.TrimSpace(id)
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDContextKey{}).(string)
	return id, ok && strings.TrimSpace(id) != ""
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestIDFromHTTP(r)
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ContextWithRequestID(r.Context(), id)))
	})
}

func RequestIDUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := requestIDFromMetadata(ctx)
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-request-id", id))
		return handler(ContextWithRequestID(ctx, id), req)
	}
}

func RequestIDStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		id := requestIDFromMetadata(stream.Context())
		_ = stream.SetHeader(metadata.Pairs("x-request-id", id))
		return handler(srv, requestIDServerStream{ServerStream: stream, ctx: ContextWithRequestID(stream.Context(), id)})
	}
}

func requestIDFromHTTP(r *http.Request) string {
	if r != nil {
		if id := sanitizeRequestID(r.Header.Get(requestIDHeader)); id != "" {
			return id
		}
	}
	return newRequestID()
}

func requestIDFromMetadata(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for _, key := range []string{"x-request-id", "request-id"} {
			if values := md.Get(key); len(values) > 0 {
				if id := sanitizeRequestID(values[0]); id != "" {
					return id
				}
			}
		}
	}
	return newRequestID()
}

func sanitizeRequestID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 128 {
		id = id[:128]
	}
	return id
}

func newRequestID() string {
	id, err := randomURLToken(18)
	if err == nil {
		return id
	}
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

type requestIDServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s requestIDServerStream) Context() context.Context { return s.ctx }
