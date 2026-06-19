// Package syncstate holds pure helpers for client/backend sync decisions.
package syncstate

import "time"

// ShouldApplyRemote reports whether a remote snapshot should replace local data.
func ShouldApplyRemote(localUpdatedAt time.Time, hasLocalMeta, hasLocalDataset bool, remoteUpdatedAt time.Time, hasRemoteDataset bool) bool {
	if !hasRemoteDataset || remoteUpdatedAt.IsZero() {
		return false
	}
	if !hasLocalDataset {
		return true
	}
	if !hasLocalMeta {
		return false
	}
	return remoteUpdatedAt.After(localUpdatedAt)
}
