// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunBlobCleanup removes partial-upload artifacts under root's "blobs/partials"
// directory whose modification time is older than before. It is the pure,
// directly testable sweep; StartBlobCleanup wraps it in a periodic loop.
func RunBlobCleanup(ctx context.Context, dataDir string, before time.Time) (int, error) {
	dir := filepath.Join(dataDir, "blobs", "partials")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("server blob cleanup: read partials: %w", err)
	}
	deleted := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return deleted, fmt.Errorf("server blob cleanup: canceled: %w", err)
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".partial") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue // best-effort: a raced/removed entry is not an error
		}
		if info.ModTime().After(before) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			// Best-effort, like the entry.Info() case above: a partial that
			// looks stale by mtime but is still open — a slow-but-active
			// upload sending chunks with gaps wider than maxAge, not an
			// abandoned one — fails to remove here (a sharing violation on
			// Windows; still-open-but-unlinkable in some POSIX setups). That
			// must not abort the whole sweep: one busy file would otherwise
			// permanently block reaping every genuinely abandoned partial
			// that sorts after it in the directory listing, on every
			// subsequent sweep for as long as that connection stays open.
			// It's simply left in place to be retried (and reaped once truly
			// idle) on the next sweep.
			continue
		}
		deleted++
	}
	return deleted, nil
}

// StartBlobCleanup launches the smallest reasonable background-job mechanism
// for reaping orphaned partial uploads: a goroutine plus a time.Ticker,
// started alongside the server (no scheduling library — the server has no
// existing periodic in-process job runner to hook into; RunRetention is
// invoked out-of-process via the "retention" CLI subcommand instead). It
// sweeps once immediately, then every interval, until ctx is canceled. The
// returned func blocks until the goroutine has exited, for clean shutdown.
func StartBlobCleanup(ctx context.Context, dataDir string, interval time.Duration, maxAge time.Duration, logger *slog.Logger) (stop func()) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if maxAge <= 0 {
		maxAge = blobPartialCutoff
	}
	done := make(chan struct{})
	sweep := func() {
		// Each sweep runs to completion on its own background context: ctx
		// governs the loop's lifetime (when to stop scheduling further
		// sweeps), not any single sweep's cancellation — a sweep already
		// running when the server starts shutting down should still finish
		// cleanly rather than abort partway through the directory listing.
		deleted, err := RunBlobCleanup(context.Background(), dataDir, time.Now().UTC().Add(-maxAge))
		if err != nil {
			if logger != nil {
				logger.Warn("blob partial cleanup failed", "error", err)
			}
			return
		}
		if deleted > 0 && logger != nil {
			logger.Info("blob partial cleanup", "deleted", deleted)
		}
	}
	go func() {
		defer close(done)
		sweep()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
	return func() { <-done }
}
