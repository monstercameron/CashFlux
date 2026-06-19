package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupManifestName = "manifest.json"

// BackupOptions describes a filesystem backup of the server SQLite database and blob store.
type BackupOptions struct {
	DataDir string
	OutDir  string
	Now     time.Time
}

// BackupManifest records what was copied and the restore expectations for a backup.
type BackupManifest struct {
	CreatedAt           time.Time        `json:"createdAt"`
	ServerSchemaVersion int              `json:"serverSchemaVersion"`
	RPO                 string           `json:"rpo"`
	RTO                 string           `json:"rto"`
	Files               []BackupFileInfo `json:"files"`
}

// BackupFileInfo records a copied file's relative path, size, and digest.
type BackupFileInfo struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// RunBackup checkpoints SQLite WAL, copies the database and blobs into a timestamped directory,
// and writes a manifest that operators can use for restore rehearsal.
func RunBackup(ctx context.Context, opts BackupOptions) (string, BackupManifest, error) {
	dataDir := strings.TrimSpace(opts.DataDir)
	outDir := strings.TrimSpace(opts.OutDir)
	if dataDir == "" || outDir == "" {
		return "", BackupManifest{}, fmt.Errorf("server backup: data and output directories are required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	} else {
		opts.Now = opts.Now.UTC()
	}
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", BackupManifest{}, fmt.Errorf("server backup: data dir: %w", err)
	}
	outDir, err = filepath.Abs(outDir)
	if err != nil {
		return "", BackupManifest{}, fmt.Errorf("server backup: output dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "cashflux-server.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		return "", BackupManifest{}, err
	}
	if err := store.CheckpointWAL(ctx); err != nil {
		_ = store.Close()
		return "", BackupManifest{}, err
	}
	if err := store.Close(); err != nil {
		return "", BackupManifest{}, fmt.Errorf("server backup: close store: %w", err)
	}

	backupDir := filepath.Join(outDir, "cashflux-backup-"+opts.Now.Format("20060102T150405Z"))
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", BackupManifest{}, fmt.Errorf("server backup: mkdir: %w", err)
	}
	manifest := BackupManifest{
		CreatedAt:           opts.Now,
		ServerSchemaVersion: CurrentServerSchemaVersion,
		RPO:                 "last successful scheduled backup",
		RTO:                 "restore the backup directory, start the server, and verify /readyz",
	}
	if err := copyBackupFile(ctx, dbPath, filepath.Join(backupDir, "cashflux-server.db"), "cashflux-server.db", &manifest); err != nil {
		return "", BackupManifest{}, err
	}
	blobRoot := filepath.Join(dataDir, "blobs")
	if _, err := os.Stat(blobRoot); err == nil {
		if err := filepath.WalkDir(blobRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dataDir, path)
			if err != nil {
				return err
			}
			return copyBackupFile(ctx, path, filepath.Join(backupDir, rel), filepath.ToSlash(rel), &manifest)
		}); err != nil {
			return "", BackupManifest{}, fmt.Errorf("server backup: copy blobs: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", BackupManifest{}, fmt.Errorf("server backup: stat blobs: %w", err)
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	if err := writeBackupManifest(ctx, backupDir, manifest); err != nil {
		return "", BackupManifest{}, err
	}
	return backupDir, manifest, nil
}

func copyBackupFile(ctx context.Context, src, dst, rel string, manifest *BackupManifest) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("server backup: canceled: %w", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("server backup: open %s: %w", rel, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("server backup: mkdir %s: %w", rel, err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("server backup: create %s: %w", rel, err)
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hasher), in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("server backup: copy %s: %w", rel, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("server backup: close %s: %w", rel, closeErr)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("server backup: canceled: %w", err)
	}
	manifest.Files = append(manifest.Files, BackupFileInfo{
		Path:   filepath.ToSlash(rel),
		Size:   written,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	})
	return nil
}

func writeBackupManifest(ctx context.Context, dir string, manifest BackupManifest) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("server backup: canceled: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("server backup: marshal manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, backupManifestName), data, 0o600); err != nil {
		return fmt.Errorf("server backup: write manifest: %w", err)
	}
	return nil
}
