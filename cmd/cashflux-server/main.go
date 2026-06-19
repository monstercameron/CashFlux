package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/monstercameron/CashFlux/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "rotate-token" {
		token, err := server.GenerateAccessToken()
		if err != nil {
			server.NewLogger(os.Stderr, server.Config{}).Error("generate token failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf("CASHFLUX_SERVER_TOKEN=%s\n", token.Token)
		fmt.Printf("CASHFLUX_SERVER_TOKEN_SHA256=%s\n", token.SHA256)
		return
	}
	cfg, err := server.FromEnv()
	if err != nil {
		server.NewLogger(os.Stderr, server.Config{}).Error("load config failed", "error", err)
		os.Exit(1)
	}
	logger := server.NewLogger(os.Stdout, cfg)
	cfg.Logger = logger
	if token := cfg.TokenForDisplay(); token != "" {
		logger.Warn("generated self-host access token", "token", token)
		logger.Warn("persist generated token", "hint", "set CASHFLUX_SERVER_TOKEN_SHA256 to the sha256 of this token, or CASHFLUX_SERVER_TOKEN for local development, to keep it stable across restarts")
	}
	store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
	if err != nil {
		logger.Error("open store failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.NewMux(cfg, store),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		logger.Info("cashflux server listening", "addr", cfg.Addr)
		errc <- srv.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errc:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("cashflux server shutting down")
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
			os.Exit(1)
		}
		if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited after shutdown", "error", err)
			os.Exit(1)
		}
	}
}
