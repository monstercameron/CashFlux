// SPDX-License-Identifier: MIT

// Package syncstate holds pure helpers for client/backend sync decisions.
package syncstate

import "time"

// ShouldResetBackoff decides whether the watch-stream reconnect backoff counter
// should reset to zero after a stream ends. Resetting on mere stream
// ESTABLISHMENT is wrong: a stream that opens and then immediately errors (the
// per-user stream cap, an auth error surfacing on the first receive, an instant
// idle-close) would reconnect at the backoff floor forever and never back off.
// The backoff is a health signal, so it resets only when the stream proved
// healthy — it either delivered at least one message, or stayed up for at least
// healthyAfter. Otherwise the caller keeps incrementing the backoff so a flapping
// or immediately-rejected stream backs off instead of thrashing.
func ShouldResetBackoff(received bool, connectedFor, healthyAfter time.Duration) bool {
	return received || connectedFor >= healthyAfter
}

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
