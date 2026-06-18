package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	Addr      string
	DataDir   string
	AuthMode  string
	Billing   bool
	AppOrigin string
	MasterKey string
	Token     string
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
