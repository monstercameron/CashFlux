// SPDX-License-Identifier: MIT

package syncengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	"github.com/monstercameron/CashFlux/internal/store"
)

// DefaultSnapshotInterval is the number of applied v2 log entries after which
// a client uploads a fresh compacted snapshot to keep bootstrap fast.
const DefaultSnapshotInterval = 64

// maxAckedChangeIDs caps the per-workspace deduplication ring so old change
// IDs never grow bookmark storage unboundedly.
const maxAckedChangeIDs = 256

// WorkspaceCursor is the durable per-workspace v2 sync bookmark persisted in
// browser storage. Every field is scoped to an exact (ServerID, AccountID,
// WorkspaceID) triple so account or server switches can never cross-contaminate.
type WorkspaceCursor struct {
	ServerID             string   `json:"serverId"`
	AccountID            string   `json:"accountId"`
	WorkspaceID          string   `json:"workspaceId"`
	AppliedSeq           int64    `json:"appliedSeq"`
	SnapshotSeq          int64    `json:"snapshotSeq"`
	ChangesSinceSnapshot int      `json:"changesSinceSnapshot"`
	LastSyncedHash       string   `json:"lastSyncedHash,omitempty"`
	AckedChangeIDs       []string `json:"ackedChangeIds,omitempty"`
}

// Scope returns the conflictcheckpoint.Scope corresponding to this cursor.
func (c WorkspaceCursor) Scope() conflictcheckpoint.Scope {
	return conflictcheckpoint.Scope{
		ServerID:    c.ServerID,
		AccountID:   c.AccountID,
		WorkspaceID: c.WorkspaceID,
	}
}

// HasAcked reports whether changeID was already appended by this device.
func (c WorkspaceCursor) HasAcked(changeID string) bool {
	for _, id := range c.AckedChangeIDs {
		if id == changeID {
			return true
		}
	}
	return false
}

// RecordAck remembers changeID in the bounded ring and advances AppliedSeq if
// this change immediately follows the applied cursor.
func (c *WorkspaceCursor) RecordAck(changeID string, seq int64) {
	changeID = strings.TrimSpace(changeID)
	if changeID != "" && !c.HasAcked(changeID) {
		c.AckedChangeIDs = append(c.AckedChangeIDs, changeID)
		if len(c.AckedChangeIDs) > maxAckedChangeIDs {
			c.AckedChangeIDs = c.AckedChangeIDs[len(c.AckedChangeIDs)-maxAckedChangeIDs:]
		}
	}
	if seq > c.AppliedSeq {
		c.AppliedSeq = seq
	}
	c.ChangesSinceSnapshot++
}

// OutboxEntry is one durable, idempotent v2 change batch waiting to be
// appended to the server. Entries are append-only in the outbox and keyed by
// ChangeID — a failed push attempt NEVER overwrites a newer queued entry.
type OutboxEntry struct {
	ChangeID     string               `json:"changeId"`
	ServerID     string               `json:"serverId"`
	AccountID    string               `json:"accountId"`
	WorkspaceID  string               `json:"workspaceId"`
	DeviceID     string               `json:"deviceId"`
	BaseSeq      int64                `json:"baseSeq"`
	Kind         BatchKind            `json:"kind"`
	Changes      []store.RecordChange `json:"changes,omitempty"`
	SnapshotJSON json.RawMessage      `json:"snapshotJson,omitempty"`
	QueuedAt     time.Time            `json:"queuedAt"`
	Attempts     int                  `json:"attempts,omitempty"`
	LastError    string               `json:"lastError,omitempty"`
}

// MatchesScope reports whether the outbox entry belongs to the exact scope.
func (e OutboxEntry) MatchesScope(scope conflictcheckpoint.Scope) bool {
	return e.ServerID == scope.ServerID &&
		e.AccountID == scope.AccountID &&
		e.WorkspaceID == scope.WorkspaceID
}

// ToBatch constructs the wire ChangeBatch for this outbox entry.
func (e OutboxEntry) ToBatch() ChangeBatch {
	return ChangeBatch{
		Version:      BatchFormatVersion,
		ChangeID:     e.ChangeID,
		WorkspaceID:  e.WorkspaceID,
		DeviceID:     e.DeviceID,
		BaseSeq:      e.BaseSeq,
		Kind:         e.Kind,
		Changes:      e.Changes,
		SnapshotJSON: e.SnapshotJSON,
		CreatedAt:    e.QueuedAt,
	}
}

// OutboxForScope returns only the entries belonging to scope, preserving FIFO
// append order. Entries belonging to other accounts/servers/workspaces remain
// untouched (fail-closed isolation).
func OutboxForScope(entries []OutboxEntry, scope conflictcheckpoint.Scope) []OutboxEntry {
	if scope.Validate() != nil {
		return nil
	}
	var out []OutboxEntry
	for _, e := range entries {
		if e.MatchesScope(scope) {
			out = append(out, e)
		}
	}
	return out
}

// EnqueueOutbox appends entry to the outbox without ever overwriting an
// existing newer entry. If an entry with the exact same ChangeID is already
// queued, it is kept as-is (idempotent).
func EnqueueOutbox(entries []OutboxEntry, entry OutboxEntry) []OutboxEntry {
	for _, existing := range entries {
		if existing.ChangeID == entry.ChangeID {
			return entries
		}
	}
	if entry.Kind == BatchKindRecords {
		entry.Changes = store.CoalesceChanges(entry.Changes)
		if len(entry.Changes) == 0 {
			return entries
		}
	}
	out := make([]OutboxEntry, len(entries)+1)
	copy(out, entries)
	out[len(entries)] = entry
	return out
}

// RemoveOutboxByIDs drops acknowledged entries by ChangeID while leaving any
// newer entries that arrived concurrently completely intact.
func RemoveOutboxByIDs(entries []OutboxEntry, changeIDs map[string]bool) []OutboxEntry {
	if len(entries) == 0 || len(changeIDs) == 0 {
		return entries
	}
	out := make([]OutboxEntry, 0, len(entries))
	for _, e := range entries {
		if !changeIDs[e.ChangeID] {
			out = append(out, e)
		}
	}
	return out
}

// MarkOutboxAttempt records a failed attempt on the matching ChangeIDs without
// dropping or reverting any newer entries that arrived while the push ran.
func MarkOutboxAttempt(entries []OutboxEntry, changeIDs map[string]bool, errMsg string) []OutboxEntry {
	out := make([]OutboxEntry, len(entries))
	copy(out, entries)
	for i := range out {
		if changeIDs[out[i].ChangeID] {
			out[i].Attempts++
			out[i].LastError = errMsg
		}
	}
	return out
}

// ClearOutboxForScope removes all outbox entries belonging to scope (used when
// the user explicitly resolves a conflict via Use Server or after a clean
// conflict-resolving snapshot push).
func ClearOutboxForScope(entries []OutboxEntry, scope conflictcheckpoint.Scope) []OutboxEntry {
	out := make([]OutboxEntry, 0, len(entries))
	for _, e := range entries {
		if !e.MatchesScope(scope) {
			out = append(out, e)
		}
	}
	return out
}

// FlattenedPendingChanges concatenates and coalesces all pending record changes
// in the outbox for a given scope. It also reports whether any outbox entry is
// a full snapshot replacement.
func FlattenedPendingChanges(entries []OutboxEntry, scope conflictcheckpoint.Scope) (changes []store.RecordChange, hasFullSnapshot bool) {
	scoped := OutboxForScope(entries, scope)
	var all []store.RecordChange
	for _, e := range scoped {
		if e.Kind == BatchKindSnapshot {
			hasFullSnapshot = true
			continue
		}
		all = append(all, e.Changes...)
	}
	return store.CoalesceChanges(all), hasFullSnapshot
}

// DetectLogTruncation reports whether server-side retention pruning has removed
// log entries between the client's applied cursor (since) and the returned page.
// Because v2 BlobSync sequences are strictly gap-free (1, 2, 3, ...), any gap
// after since+1 (or an empty page while since < head) means the client must
// bootstrap from GetSnapshot before replaying the tail.
func DetectLogTruncation(since, head int64, changes []backendrpc.BlobChange) bool {
	if head <= since {
		return false
	}
	if len(changes) == 0 {
		return true
	}
	return changes[0].Seq > since+1
}

// HashBytes returns the lowercase hex SHA-256 digest of b.
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// StoredConflict bundles a durable conflictcheckpoint.Checkpoint together with
// both decrypted snapshots and the colliding record keys so the UI can render
// an exact diff and resolve with Keep Mine, Use Server, or Merge without
// losing a single byte.
type StoredConflict struct {
	Checkpoint     conflictcheckpoint.Checkpoint `json:"checkpoint"`
	LocalJSON      json.RawMessage               `json:"localJson"`
	ServerJSON     json.RawMessage               `json:"serverJson"`
	ServerHeadSeq  int64                         `json:"serverHeadSeq"`
	CollidingKeys  []RecordKey                   `json:"collidingKeys,omitempty"`
	PendingChanges []store.RecordChange          `json:"pendingChanges,omitempty"`
}
