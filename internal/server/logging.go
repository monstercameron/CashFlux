// SPDX-License-Identifier: MIT

package server

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// NewLogger builds the server logger from Config.
func NewLogger(w io.Writer, cfg Config) *slog.Logger {
	level := new(slog.LevelVar)
	level.Set(parseLogLevel(cfg.LogLevel))
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactLogAttr,
	}
	if strings.EqualFold(strings.TrimSpace(cfg.LogFormat), "json") {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func redactLogAttr(_ []string, attr slog.Attr) slog.Attr {
	key := strings.ToLower(attr.Key)
	for _, marker := range []string{"token", "secret", "key", "authorization", "cookie", "password"} {
		if strings.Contains(key, marker) {
			return slog.String(attr.Key, "[REDACTED]")
		}
	}
	return attr
}

type loggerContextKey struct{}
type logScopeContextKey struct{}

type LogScope struct {
	UserID      string
	WorkspaceID string
	DeviceID    string
}

func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

func ContextWithLogScope(ctx context.Context) context.Context {
	if _, ok := ctx.Value(logScopeContextKey{}).(*LogScope); ok {
		return ctx
	}
	return context.WithValue(ctx, logScopeContextKey{}, &LogScope{})
}

func SetLogScope(ctx context.Context, scope LogScope) {
	current, ok := ctx.Value(logScopeContextKey{}).(*LogScope)
	if !ok || current == nil {
		return
	}
	if scope.UserID != "" {
		current.UserID = scope.UserID
	}
	if scope.WorkspaceID != "" {
		current.WorkspaceID = scope.WorkspaceID
	}
	if scope.DeviceID != "" {
		current.DeviceID = scope.DeviceID
	}
}

func LogScopeFromContext(ctx context.Context) (LogScope, bool) {
	scope, ok := ctx.Value(logScopeContextKey{}).(*LogScope)
	if !ok || scope == nil {
		return LogScope{}, false
	}
	return *scope, true
}

func requestLogMiddleware(logger *slog.Logger, metrics *Metrics, next http.Handler) http.Handler {
	return requestLogMiddlewareSampled(logger, metrics, 0, next)
}

func requestLogMiddlewareSampled(logger *slog.Logger, metrics *Metrics, hotPathSampleRate int, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	var hotPathCount uint64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		ctx := ContextWithLogScope(ContextWithLogger(r.Context(), logger))
		next.ServeHTTP(rec, r.WithContext(ctx))
		if metrics != nil {
			metrics.ObserveHTTP(r.URL.Path, rec.status, time.Since(start))
		}
		id, _ := RequestIDFromContext(ctx)
		args := []any{
			"request_id", id,
			"method", r.Method,
			"route", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if traceID, ok := TraceIDFromContext(ctx); ok {
			args = append(args, "trace_id", traceID)
		}
		if user, ok := AuthUserFromContext(ctx); ok {
			args = append(args, "user_id", user.ID)
		}
		args = appendLogScopeArgs(ctx, args)
		if rec.status >= 500 {
			args = append(args, "cause", http.StatusText(rec.status))
			logger.Error("http request failed", args...)
			return
		}
		if shouldSampleHTTPLog(r, rec.status, hotPathSampleRate, &hotPathCount) {
			if hotPathSampleRate > 1 && isHotHTTPPath(r.Method, r.URL.Path) && rec.status < 400 {
				args = append(args, "sample_rate", hotPathSampleRate)
			}
			logger.Info("http request", args...)
		}
	})
}

func shouldSampleHTTPLog(r *http.Request, status, sampleRate int, counter *uint64) bool {
	if sampleRate <= 1 || status >= 400 || !isHotHTTPPath(r.Method, r.URL.Path) {
		return true
	}
	n := atomic.AddUint64(counter, 1)
	return n == 1 || n%uint64(sampleRate) == 0
}

func isHotHTTPPath(method, path string) bool {
	if method != http.MethodGet {
		return false
	}
	switch path {
	case "/livez", "/healthz", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func LoggingUnaryInterceptor(logger *slog.Logger, metrics *Metrics) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		ctx = ContextWithLogScope(ContextWithLogger(ctx, logger))
		resp, err := handler(ctx, req)
		elapsed := time.Since(start)
		if metrics != nil {
			metrics.ObserveGRPC(info.FullMethod, status.Code(err).String(), elapsed)
		}
		logRPC(ctx, logger, info.FullMethod, status.Code(err).String(), elapsed, err)
		return resp, err
	}
}

func LoggingStreamInterceptor(logger *slog.Logger, metrics *Metrics) grpc.StreamServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := ContextWithLogScope(ContextWithLogger(stream.Context(), logger))
		err := handler(srv, loggingServerStream{ServerStream: stream, ctx: ctx})
		elapsed := time.Since(start)
		if metrics != nil {
			metrics.ObserveGRPC(info.FullMethod, status.Code(err).String(), elapsed)
		}
		logRPC(ctx, logger, info.FullMethod, status.Code(err).String(), elapsed, err)
		return err
	}
}

func logRPC(ctx context.Context, logger *slog.Logger, method, code string, elapsed time.Duration, err error) {
	id, _ := RequestIDFromContext(ctx)
	args := []any{
		"request_id", id,
		"rpc", method,
		"status", code,
		"duration_ms", elapsed.Milliseconds(),
	}
	if traceID, ok := TraceIDFromContext(ctx); ok {
		args = append(args, "trace_id", traceID)
	}
	if user, ok := AuthUserFromContext(ctx); ok {
		args = append(args, "user_id", user.ID)
	}
	args = appendLogScopeArgs(ctx, args)
	if err != nil {
		args = append(args, "cause", err.Error())
		logger.Error("grpc request failed", args...)
		return
	}
	logger.Info("grpc request", args...)
}

func appendLogScopeArgs(ctx context.Context, args []any) []any {
	scope, ok := LogScopeFromContext(ctx)
	if !ok {
		return args
	}
	if scope.UserID != "" && !logArgsContain(args, "user_id") {
		args = append(args, "user_id", scope.UserID)
	}
	if scope.WorkspaceID != "" {
		args = append(args, "workspace_id", scope.WorkspaceID)
	}
	if scope.DeviceID != "" {
		args = append(args, "device_id", scope.DeviceID)
	}
	return args
}

func logArgsContain(args []any, key string) bool {
	for i := 0; i+1 < len(args); i += 2 {
		if got, ok := args[i].(string); ok && got == key {
			return true
		}
	}
	return false
}

type loggingServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s loggingServerStream) Context() context.Context { return s.ctx }
