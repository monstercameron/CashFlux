// SPDX-License-Identifier: MIT

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

const (
	requestIDHeader    = "X-Request-ID"
	traceparentHeader  = "Traceparent"
	traceIDMetadataKey = "traceparent"
)

type requestIDContextKey struct{}
type traceIDContextKey struct{}

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

func ContextWithTraceID(ctx context.Context, id string) context.Context {
	id = sanitizeTraceID(id)
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDContextKey{}, id)
}

func TraceIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(traceIDContextKey{}).(string)
	return id, ok && strings.TrimSpace(id) != ""
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestIDFromHTTP(r)
		w.Header().Set(requestIDHeader, id)
		ctx := ContextWithRequestID(r.Context(), id)
		ctx = ContextWithTraceID(ctx, traceIDFromHTTP(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := requestIDFromMetadata(ctx)
		traceID := traceIDFromMetadata(ctx)
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-request-id", id))
		nextCtx := ContextWithRequestID(ctx, id)
		nextCtx = ContextWithTraceID(nextCtx, traceID)
		return handler(nextCtx, req)
	}
}

func RequestIDStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		id := requestIDFromMetadata(stream.Context())
		traceID := traceIDFromMetadata(stream.Context())
		_ = stream.SetHeader(metadata.Pairs("x-request-id", id))
		ctx := ContextWithRequestID(stream.Context(), id)
		ctx = ContextWithTraceID(ctx, traceID)
		return handler(srv, requestIDServerStream{ServerStream: stream, ctx: ctx})
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

func traceIDFromHTTP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id := traceIDFromTraceparent(r.Header.Get(traceparentHeader)); id != "" {
		return id
	}
	return sanitizeTraceID(r.Header.Get("X-Trace-ID"))
}

func traceIDFromMetadata(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(traceIDMetadataKey); len(values) > 0 {
			if id := traceIDFromTraceparent(values[0]); id != "" {
				return id
			}
		}
		if values := md.Get("x-trace-id"); len(values) > 0 {
			return sanitizeTraceID(values[0])
		}
	}
	return ""
}

func sanitizeRequestID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 128 {
		id = id[:128]
	}
	return id
}

func traceIDFromTraceparent(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 4 {
		return ""
	}
	return sanitizeTraceID(parts[1])
}

func sanitizeTraceID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if len(id) != 32 || id == "00000000000000000000000000000000" {
		return ""
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
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
