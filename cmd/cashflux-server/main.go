// SPDX-License-Identifier: MIT

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
	if len(os.Args) > 1 && os.Args[1] == "backup" {
		outDir := filepath.Join(cfg.DataDir, "backups")
		if len(os.Args) > 2 {
			outDir = os.Args[2]
		}
		backupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		backupDir, manifest, err := server.RunBackup(backupCtx, server.BackupOptions{
			DataDir: cfg.DataDir,
			OutDir:  outDir,
		})
		if err != nil {
			logger.Error("server backup failed", "error", err)
			os.Exit(1)
		}
		logger.Info("server backup complete", "path", backupDir, "files", len(manifest.Files), "rpo", manifest.RPO, "rto", manifest.RTO)
		fmt.Println(backupDir)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "retention" {
		store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
		if err != nil {
			logger.Error("open store failed", "error", err)
			os.Exit(1)
		}
		defer func() { _ = store.Close() }()
		retentionCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := server.RunRetention(retentionCtx, store, server.RetentionOptions{
			DataDir:                      cfg.DataDir,
			AuditRetentionDays:           cfg.AuditRetentionDays,
			SnapshotHistoryRetentionDays: cfg.SnapshotHistoryRetentionDays,
			BackupRetentionDays:          cfg.BackupRetentionDays,
		})
		if err != nil {
			logger.Error("server retention failed", "error", err)
			os.Exit(1)
		}
		logger.Info("server retention complete", "audit_events_deleted", result.AuditEventsDeleted, "snapshot_history_deleted", result.SnapshotHistoryDeleted, "backup_directories_deleted", result.BackupDirectoriesDeleted)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate-check" {
		version, err := server.DryRunStoreMigrations(filepath.Join(cfg.DataDir, "cashflux-server.db"))
		if err != nil {
			logger.Error("server migration dry-run failed", "error", err)
			os.Exit(1)
		}
		logger.Info("server migration dry-run complete", "schema_version", version)
		fmt.Printf("schema_version=%d\n", version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "rotate-ai-master-key" {
		oldMasterKey := os.Getenv("CASHFLUX_SERVER_OLD_MASTER_KEY")
		if oldMasterKey == "" {
			logger.Error("old master key is required", "env", "CASHFLUX_SERVER_OLD_MASTER_KEY")
			os.Exit(1)
		}
		store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
		if err != nil {
			logger.Error("open store failed", "error", err)
			os.Exit(1)
		}
		defer func() { _ = store.Close() }()
		count, err := store.RotateAIKeys([]byte(oldMasterKey), []byte(cfg.MasterKey))
		if err != nil {
			logger.Error("server ai master-key rotation failed", "error", err)
			os.Exit(1)
		}
		logger.Info("server ai master-key rotation complete", "rotated_count", count)
		fmt.Printf("ai_keys_rotated=%d\n", count)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "gc-blobs" {
		store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
		if err != nil {
			logger.Error("open store failed", "error", err)
			os.Exit(1)
		}
		defer func() { _ = store.Close() }()
		deleted, err := store.SweepUnreferencedBlobs(filepath.Join(cfg.DataDir, "blobs"))
		if err != nil {
			logger.Error("server blob gc failed", "error", err)
			os.Exit(1)
		}
		cfg.Metrics.ObserveBlobGC(deleted)
		logger.Info("server blob gc complete", "deleted", deleted)
		return
	}
	if token := cfg.TokenForDisplay(); token != "" {
		logger.Warn("generated self-host access token", "token", token)
		logger.Warn("persist generated token", "hint", "set CASHFLUX_SERVER_TOKEN_SHA256 to the sha256 of this token, or CASHFLUX_SERVER_TOKEN for local development, to keep it stable across restarts")
	}
	// Multi-tenant Cloud should sign sessions with a dedicated key so an AES
	// master-key rotation doesn't log every user out and neither secret's leak
	// compromises the other. Falling back to MasterKey works but couples them.
	if cfg.AuthMode == "oauth" && cfg.SessionKey == "" {
		logger.Warn("session signing key not set", "hint", "set CASHFLUX_SERVER_SESSION_KEY to a dedicated random secret; falling back to the AES master key couples session signing to encryption-key rotation")
	}
	traceCtx, traceCancel := context.WithTimeout(context.Background(), 10*time.Second)
	traceShutdown, err := server.ConfigureTracing(traceCtx, cfg)
	traceCancel()
	if err != nil {
		logger.Error("configure tracing failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceShutdown(ctx); err != nil {
			logger.Debug("tracing shutdown failed", "error", err)
		}
	}()
	store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
	if err != nil {
		logger.Error("open store failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.NewMux(cfg, store),
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
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

	// Reap orphaned partial-upload artifacts from interrupted BlobService
	// streaming uploads (TODOS.md C444) — otherwise they'd silently consume
	// the C434 storage quota forever. Tied to the same shutdown context as
	// the HTTP server; stopBlobCleanup blocks until its goroutine exits so
	// shutdown doesn't return while a sweep is still touching disk.
	stopBlobCleanup := server.StartBlobCleanup(ctx, cfg.DataDir, 0, 0, logger)
	defer stopBlobCleanup()

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
		if err := store.CheckpointWAL(shutdownCtx); err != nil {
			logger.Error("server wal checkpoint failed", "error", err)
			os.Exit(1)
		}
		if err := os.Stdout.Sync(); err != nil {
			logger.Debug("server log flush skipped", "error", err)
		}
		if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited after shutdown", "error", err)
			os.Exit(1)
		}
	}
}
