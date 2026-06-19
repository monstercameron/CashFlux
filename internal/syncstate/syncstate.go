// Package syncstate holds pure helpers for client/backend sync decisions.
package syncstate

import "time"

// PendingMutation is the client-side representation of one queued workspace
// snapshot waiting to be pushed to the backend.
type PendingMutation struct {
	WorkspaceID string
	Hash        string
	UpdatedAt   string
}

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

// UpsertPending keeps only the latest queued mutation for a workspace. The
// browser stores the full dataset separately; the pure helper tracks ordering.
func UpsertPending(queue []PendingMutation, next PendingMutation) []PendingMutation {
	if next.WorkspaceID == "" || next.Hash == "" {
		return queue
	}
	out := make([]PendingMutation, 0, len(queue)+1)
	replaced := false
	for _, item := range queue {
		if item.WorkspaceID == next.WorkspaceID {
			if !replaced {
				out = append(out, next)
				replaced = true
			}
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, next)
	}
	return out
}

// RemovePending drops a queued mutation after the backend accepts or supersedes it.
func RemovePending(queue []PendingMutation, workspaceID, hash string) []PendingMutation {
	out := make([]PendingMutation, 0, len(queue))
	for _, item := range queue {
		if item.WorkspaceID == workspaceID && (hash == "" || item.Hash == hash) {
			continue
		}
		out = append(out, item)
	}
	return out
}
