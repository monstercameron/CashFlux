package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"sort"
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
	TokenSHA256                       string
	GeneratedToken                    bool
	OAuthProviders                    map[string]OAuthProviderConfig
	OpenAIBaseURL                     string
	AIAllowedModels                   []string
	AIUpstreamTimeout                 time.Duration
	AIUpstreamRetries                 int
	AIRequestMaxBytes                 int64
	AIRequestsPerDay                  int64
	AITokensPerDay                    int64
	BlobMaxBytes                      int64
	GRPCReadLimitBytes                int64
	GRPCKeepaliveInterval             time.Duration
	GRPCIdleTimeout                   time.Duration
	GRPCMaxActiveConnections          int
	GRPCMaxConnectionsPerClient       int
	GRPCMaxUpgradesPerClientPerMinute int
	HTTPReadTimeout                   time.Duration
	HTTPWriteTimeout                  time.Duration
	HTTPMaxInFlight                   int
	LogFormat                         string
	LogLevel                          string
	Logger                            *slog.Logger
}

type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
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
	cfg.TokenSHA256 = strings.TrimSpace(os.Getenv("CASHFLUX_SERVER_TOKEN_SHA256"))
	cfg.OAuthProviders = oauthProvidersFromEnv()
	cfg.OpenAIBaseURL = strings.TrimSpace(os.Getenv("CASHFLUX_SERVER_OPENAI_BASE_URL"))
	cfg.AIAllowedModels = envCSV("CASHFLUX_SERVER_AI_MODELS")
	cfg.AIUpstreamTimeout = envDuration("CASHFLUX_SERVER_AI_UPSTREAM_TIMEOUT", 45*time.Second)
	cfg.AIUpstreamRetries = int(envInt64("CASHFLUX_SERVER_AI_UPSTREAM_RETRIES", 2))
	cfg.AIRequestMaxBytes = envInt64("CASHFLUX_SERVER_AI_REQUEST_MAX_BYTES", 4<<20)
	cfg.AIRequestsPerDay = envInt64("CASHFLUX_SERVER_AI_REQUESTS_PER_DAY", 0)
	cfg.AITokensPerDay = envInt64("CASHFLUX_SERVER_AI_TOKENS_PER_DAY", 0)
	cfg.BlobMaxBytes = envInt64("CASHFLUX_SERVER_BLOB_MAX_BYTES", 32<<20)
	cfg.GRPCReadLimitBytes = envInt64("CASHFLUX_SERVER_GRPC_READ_LIMIT_BYTES", 16<<20)
	cfg.GRPCKeepaliveInterval = envDuration("CASHFLUX_SERVER_GRPC_KEEPALIVE_INTERVAL", 30*time.Second)
	cfg.GRPCIdleTimeout = envDuration("CASHFLUX_SERVER_GRPC_IDLE_TIMEOUT", 90*time.Second)
	cfg.GRPCMaxActiveConnections = int(envInt64("CASHFLUX_SERVER_GRPC_MAX_ACTIVE_CONNECTIONS", 128))
	cfg.GRPCMaxConnectionsPerClient = int(envInt64("CASHFLUX_SERVER_GRPC_MAX_CONNECTIONS_PER_CLIENT", 8))
	cfg.GRPCMaxUpgradesPerClientPerMinute = int(envInt64("CASHFLUX_SERVER_GRPC_MAX_UPGRADES_PER_CLIENT_PER_MINUTE", 60))
	cfg.HTTPReadTimeout = envDuration("CASHFLUX_SERVER_HTTP_READ_TIMEOUT", 15*time.Second)
	cfg.HTTPWriteTimeout = envDuration("CASHFLUX_SERVER_HTTP_WRITE_TIMEOUT", 60*time.Second)
	cfg.HTTPMaxInFlight = int(envInt64("CASHFLUX_SERVER_HTTP_MAX_IN_FLIGHT", 256))
	cfg.LogFormat = strings.ToLower(envOr("CASHFLUX_SERVER_LOG_FORMAT", "text"))
	cfg.LogLevel = strings.ToLower(envOr("CASHFLUX_SERVER_LOG_LEVEL", "info"))
	if cfg.AuthMode == "token" && cfg.Token == "" && cfg.TokenSHA256 == "" {
		token, err := randomURLToken(32)
		if err != nil {
			return Config{}, fmt.Errorf("server: generate token: %w", err)
		}
		cfg.Token = token
		cfg.GeneratedToken = true
	}
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
	if c.AIRequestMaxBytes < 0 || c.AIRequestsPerDay < 0 || c.AITokensPerDay < 0 || c.AIUpstreamRetries < 0 {
		return fmt.Errorf("server: ai limits must be non-negative")
	}
	if c.AIUpstreamTimeout < 0 {
		return fmt.Errorf("server: ai upstream timeout must be non-negative")
	}
	if c.BlobMaxBytes < 0 {
		return fmt.Errorf("server: blob max bytes must be non-negative")
	}
	if c.TokenSHA256 != "" {
		decoded, err := hex.DecodeString(c.TokenSHA256)
		if err != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("server: token sha256 must be a hex-encoded sha256 digest")
		}
	}
	if c.GRPCReadLimitBytes < 0 || c.GRPCKeepaliveInterval < 0 || c.GRPCIdleTimeout < 0 ||
		c.GRPCMaxActiveConnections < 0 || c.GRPCMaxConnectionsPerClient < 0 || c.GRPCMaxUpgradesPerClientPerMinute < 0 {
		return fmt.Errorf("server: grpc bridge limits must be non-negative")
	}
	if c.HTTPReadTimeout < 0 || c.HTTPWriteTimeout < 0 || c.HTTPMaxInFlight < 0 {
		return fmt.Errorf("server: http limits must be non-negative")
	}
	if c.GRPCIdleTimeout > 0 && c.GRPCKeepaliveInterval <= 0 {
		return fmt.Errorf("server: grpc keepalive interval is required when idle timeout is set")
	}
	if c.GRPCIdleTimeout > 0 && c.GRPCKeepaliveInterval >= c.GRPCIdleTimeout {
		return fmt.Errorf("server: grpc keepalive interval must be less than idle timeout")
	}
	if c.LogFormat != "" && c.LogFormat != "text" && c.LogFormat != "json" {
		return fmt.Errorf("server: log format must be text or json")
	}
	switch c.LogLevel {
	case "", "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("server: log level must be debug, info, warn, or error")
	}
	for name, provider := range c.OAuthProviders {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("server: oauth provider name is required")
		}
		if strings.TrimSpace(provider.ClientID) == "" || strings.TrimSpace(provider.ClientSecret) == "" || strings.TrimSpace(provider.RedirectURL) == "" {
			return fmt.Errorf("server: oauth provider %q requires client id, client secret, and redirect url", name)
		}
	}
	switch c.AuthMode {
	case "token", "oauth":
		if c.AuthMode == "oauth" && len(c.OAuthProviders) == 0 {
			return fmt.Errorf("server: oauth auth mode requires at least one provider")
		}
		return nil
	default:
		return fmt.Errorf("server: unsupported auth mode %q", c.AuthMode)
	}
}

func (c Config) OAuthProviderNames() []string {
	names := make([]string, 0, len(c.OAuthProviders))
	for name := range c.OAuthProviders {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
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

func oauthProvidersFromEnv() map[string]OAuthProviderConfig {
	providers := map[string]OAuthProviderConfig{}
	for _, name := range []string{"google", "github"} {
		prefix := "CASHFLUX_SERVER_OAUTH_" + strings.ToUpper(name) + "_"
		cfg := OAuthProviderConfig{
			ClientID:     strings.TrimSpace(os.Getenv(prefix + "CLIENT_ID")),
			ClientSecret: strings.TrimSpace(os.Getenv(prefix + "CLIENT_SECRET")),
			RedirectURL:  strings.TrimSpace(os.Getenv(prefix + "REDIRECT_URL")),
		}
		if cfg.ClientID != "" || cfg.ClientSecret != "" || cfg.RedirectURL != "" {
			providers[name] = cfg
		}
	}
	return providers
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
