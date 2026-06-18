package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAddr         = ":8081"
	DefaultDataDir      = "data"
	DefaultAuthMode     = "token"
	APIVersion          = "v1"
	MinClientAPIVersion = "v1"
)

// Config contains the runtime settings for the optional CashFlux backend.
type Config struct {
	Addr                              string
	DataDir                           string
	AuthMode                          string
	Billing                           bool
	AppOrigin                         string
	MasterKey                         string
	Token                             string
	OpenAIBaseURL                     string
	AIAllowedModels                   []string
	AIRequestMaxBytes                 int64
	AIRequestsPerDay                  int64
	AITokensPerDay                    int64
	GRPCReadLimitBytes                int64
	GRPCKeepaliveInterval             time.Duration
	GRPCIdleTimeout                   time.Duration
	GRPCMaxActiveConnections          int
	GRPCMaxConnectionsPerClient       int
	GRPCMaxUpgradesPerClientPerMinute int
}

// FromEnv builds server config from CASHFLUX_SERVER_* environment variables.
func FromEnv() (Config, error) {
	cfg := Config{
		Addr:     envOr("CASHFLUX_SERVER_ADDR", DefaultAddr),
		DataDir:  envOr("CASHFLUX_SERVER_DATA_DIR", DefaultDataDir),
		AuthMode: envOr("CASHFLUX_SERVER_AUTH_MODE", DefaultAuthMode),
		Billing:  envBool("CASHFLUX_SERVER_BILLING", false),
	}
	cfg.AppOrigin = strings.TrimSpace(os.Getenv("CASHFLUX_SERVER_APP_ORIGIN"))
	cfg.MasterKey = strings.TrimSpace(os.Getenv("CASHFLUX_SERVER_MASTER_KEY"))
	cfg.Token = strings.TrimSpace(os.Getenv("CASHFLUX_SERVER_TOKEN"))
	cfg.OpenAIBaseURL = strings.TrimSpace(os.Getenv("CASHFLUX_SERVER_OPENAI_BASE_URL"))
	cfg.AIAllowedModels = envCSV("CASHFLUX_SERVER_AI_MODELS")
	cfg.AIRequestMaxBytes = envInt64("CASHFLUX_SERVER_AI_REQUEST_MAX_BYTES", 4<<20)
	cfg.AIRequestsPerDay = envInt64("CASHFLUX_SERVER_AI_REQUESTS_PER_DAY", 0)
	cfg.AITokensPerDay = envInt64("CASHFLUX_SERVER_AI_TOKENS_PER_DAY", 0)
	cfg.GRPCReadLimitBytes = envInt64("CASHFLUX_SERVER_GRPC_READ_LIMIT_BYTES", 16<<20)
	cfg.GRPCKeepaliveInterval = envDuration("CASHFLUX_SERVER_GRPC_KEEPALIVE_INTERVAL", 30*time.Second)
	cfg.GRPCIdleTimeout = envDuration("CASHFLUX_SERVER_GRPC_IDLE_TIMEOUT", 90*time.Second)
	cfg.GRPCMaxActiveConnections = int(envInt64("CASHFLUX_SERVER_GRPC_MAX_ACTIVE_CONNECTIONS", 128))
	cfg.GRPCMaxConnectionsPerClient = int(envInt64("CASHFLUX_SERVER_GRPC_MAX_CONNECTIONS_PER_CLIENT", 8))
	cfg.GRPCMaxUpgradesPerClientPerMinute = int(envInt64("CASHFLUX_SERVER_GRPC_MAX_UPGRADES_PER_CLIENT_PER_MINUTE", 60))
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate rejects unsupported server modes before the HTTP server starts.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("server: addr is required")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("server: data dir is required")
	}
	if c.MasterKey != "" && !validAESKeyLength(len(c.MasterKey)) {
		return fmt.Errorf("server: master key must be 16, 24, or 32 bytes")
	}
	if c.AIRequestMaxBytes < 0 || c.AIRequestsPerDay < 0 || c.AITokensPerDay < 0 {
		return fmt.Errorf("server: ai limits must be non-negative")
	}
	if c.GRPCReadLimitBytes < 0 || c.GRPCKeepaliveInterval < 0 || c.GRPCIdleTimeout < 0 ||
		c.GRPCMaxActiveConnections < 0 || c.GRPCMaxConnectionsPerClient < 0 || c.GRPCMaxUpgradesPerClientPerMinute < 0 {
		return fmt.Errorf("server: grpc bridge limits must be non-negative")
	}
	if c.GRPCIdleTimeout > 0 && c.GRPCKeepaliveInterval <= 0 {
		return fmt.Errorf("server: grpc keepalive interval is required when idle timeout is set")
	}
	if c.GRPCIdleTimeout > 0 && c.GRPCKeepaliveInterval >= c.GRPCIdleTimeout {
		return fmt.Errorf("server: grpc keepalive interval must be less than idle timeout")
	}
	switch c.AuthMode {
	case "token", "oauth":
		return nil
	default:
		return fmt.Errorf("server: unsupported auth mode %q", c.AuthMode)
	}
}

func validAESKeyLength(n int) bool { return n == 16 || n == 24 || n == 32 }

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
