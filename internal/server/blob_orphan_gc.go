// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// OrphanBlobGracePeriod keeps a newly written file out of the disk inventory
// while an older server version might still be attaching its metadata.
const OrphanBlobGracePeriod = 24 * time.Hour

// SweepOrphanBlobFiles removes aged content-addressed files with no blob row.
// It recovers files that an earlier GC lost track of after a failed removal.
// The live upload and both GC paths share an immediate SQLite write lock, so a
// file cannot be removed while it is being linked to a workspace.
func (s *Store) SweepOrphanBlobFiles(rootPath string, before time.Time) (int, error) {
	root, err := os.OpenRoot(rootPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("server store: orphan blob root: %w", err)
	}
	defer root.Close()
	hashes, err := orphanBlobCandidates(root, before)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, hash := range hashes {
		removed, err := s.sweepOrphanBlobFile(root, hash, before)
		if err != nil {
			return deleted, err
		}
		if removed {
			deleted++
		}
	}
	return deleted, nil
}

func orphanBlobCandidates(root *os.Root, before time.Time) ([]string, error) {
	first, err := readRootDir(root, ".")
	if err != nil {
		return nil, err
	}
	var hashes []string
	for _, one := range first {
		if !one.IsDir() || !validHashPrefix(one.Name()) {
			continue
		}
		second, err := readRootDir(root, one.Name())
		if err != nil {
			return nil, err
		}
		for _, two := range second {
			if !two.IsDir() || !validHashPrefix(two.Name()) {
				continue
			}
			prefix := filepath.Join(one.Name(), two.Name())
			files, err := readRootDir(root, prefix)
			if err != nil {
				return nil, err
			}
			for _, file := range files {
				hash := file.Name()
				if !validBlobHash(hash) || hash[:2] != one.Name() || hash[2:4] != two.Name() {
					continue
				}
				info, err := file.Info()
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("server store: inspect orphan blob %s: %w", hash, err)
				}
				if info.Mode().IsRegular() && !info.ModTime().After(before) {
					hashes = append(hashes, hash)
				}
			}
		}
	}
	return hashes, nil
}

func readRootDir(root *os.Root, path string) ([]os.DirEntry, error) {
	dir, err := root.Open(path)
	if err != nil {
		return nil, fmt.Errorf("server store: open blob directory %s: %w", path, err)
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("server store: read blob directory %s: %w", path, err)
	}
	return entries, nil
}

func validHashPrefix(prefix string) bool {
	if len(prefix) != 2 {
		return false
	}
	for _, c := range prefix {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func (s *Store) sweepOrphanBlobFile(root *os.Root, hash string, before time.Time) (bool, error) {
	relativePath, err := blobRelativePath(hash)
	if err != nil {
		return false, err
	}
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("server store: orphan gc connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return false, fmt.Errorf("server store: begin orphan gc: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	var present int
	if err := conn.QueryRowContext(ctx, `SELECT
EXISTS(SELECT 1 FROM blobs WHERE hash = ?) OR
EXISTS(SELECT 1 FROM workspace_blobs WHERE hash = ?)`, hash, hash).Scan(&present); err != nil {
		return false, fmt.Errorf("server store: check orphan blob row: %w", err)
	}
	if present != 0 {
		return false, nil
	}
	info, err := root.Lstat(relativePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("server store: stat orphan blob %s: %w", hash, err)
	}
	if !info.Mode().IsRegular() || info.ModTime().After(before) {
		return false, nil
	}
	if err := root.Remove(relativePath); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("server store: remove orphan blob %s: %w", hash, err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return false, fmt.Errorf("server store: commit orphan gc: %w", err)
	}
	committed = true
	return true, nil
}
