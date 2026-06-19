package server

import (
	"io"
	"log/slog"
	"strings"
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
