// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RetentionOptions describes backend data retention pruning.
type RetentionOptions struct {
	DataDir                      string
	AuditRetentionDays           int
	SnapshotHistoryRetentionDays int
	BackupRetentionDays          int
	Now                          time.Time
}

// RetentionResult reports rows and backup directories pruned by a retention run.
type RetentionResult struct {
	AuditEventsDeleted       int64
	SnapshotHistoryDeleted   int64
	BackupDirectoriesDeleted int
}

// RunRetention prunes server-managed retained data according to configured day windows.
func RunRetention(ctx context.Context, store *Store, opts RetentionOptions) (RetentionResult, error) {
	if store == nil {
		return RetentionResult{}, fmt.Errorf("server retention: store is required")
	}
	if opts.AuditRetentionDays < 0 || opts.SnapshotHistoryRetentionDays < 0 || opts.BackupRetentionDays < 0 {
		return RetentionResult{}, fmt.Errorf("server retention: retention days must be non-negative")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	} else {
		opts.Now = opts.Now.UTC()
	}
	var result RetentionResult
	if opts.AuditRetentionDays > 0 {
		deleted, err := store.PruneAuditEventsBefore(ctx, opts.Now.AddDate(0, 0, -opts.AuditRetentionDays))
		if err != nil {
			return RetentionResult{}, err
		}
		result.AuditEventsDeleted = deleted
	}
	if opts.SnapshotHistoryRetentionDays > 0 {
		deleted, err := store.PruneSnapshotHistoryBefore(ctx, opts.Now.AddDate(0, 0, -opts.SnapshotHistoryRetentionDays))
		if err != nil {
			return RetentionResult{}, err
		}
		result.SnapshotHistoryDeleted = deleted
	}
	if opts.BackupRetentionDays > 0 && strings.TrimSpace(opts.DataDir) != "" {
		deleted, err := PruneBackupDirectories(ctx, filepath.Join(opts.DataDir, "backups"), opts.Now.AddDate(0, 0, -opts.BackupRetentionDays))
		if err != nil {
			return RetentionResult{}, err
		}
		result.BackupDirectoriesDeleted = deleted
	}
	return result, nil
}

// PruneBackupDirectories removes local backup directories whose timestamped names are older than before.
func PruneBackupDirectories(ctx context.Context, root string, before time.Time) (int, error) {
	if strings.TrimSpace(root) == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("server retention: read backups: %w", err)
	}
	deleted := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return deleted, fmt.Errorf("server retention: canceled: %w", err)
		}
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "cashflux-backup-") {
			continue
		}
		created, err := time.Parse("20060102T150405Z", strings.TrimPrefix(name, "cashflux-backup-"))
		if err != nil || !created.Before(before.UTC()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return deleted, fmt.Errorf("server retention: remove backup %s: %w", name, err)
		}
		deleted++
	}
	return deleted, nil
}
