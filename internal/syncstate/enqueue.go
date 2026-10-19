// SPDX-License-Identifier: MIT

package syncstate

import "strings"

// This file binds a queued push to the scope it was created under (SV0.1).
//
// The 2026-08-19 incident class is a scope captured too late: a mutation was
// queued under one account or workspace, and by the time the flush ran, the
// ACTIVE workspace, the signed-in account and the last-seen server version
// were read again at send time — so the snapshot went out under whatever
// scope happened to be current then. Capturing the scope at enqueue and
// pushing exactly that removes the whole class: an entry can only ever be
// sent as the workspace, owner and base version it was queued with, and an
// entry whose owner is not the signed-in account is parked, never sent.

// QueueScope is the identity a queued mutation is bound to at enqueue time:
// the workspace it snapshots, the account that owned the edit, and the server
// version that edit was based on.
type QueueScope struct {
	WorkspaceID string
	OwnerUserID string
	BaseVersion int64
}

// CaptureScope builds a QueueScope from the values current at enqueue time.
// It is the only moment those values may be read: every later send uses the
// captured copy.
func CaptureScope(workspaceID, ownerUserID string, baseVersion int64) QueueScope {
	return QueueScope{
		WorkspaceID: strings.TrimSpace(workspaceID),
		OwnerUserID: strings.TrimSpace(ownerUserID),
		BaseVersion: baseVersion,
	}
}

// Entry projects the scope onto a ScopedEntry for queue bookkeeping.
func (s QueueScope) Entry(hash string) ScopedEntry {
	return ScopedEntry{
		WorkspaceID: s.WorkspaceID,
		OwnerUserID: s.OwnerUserID,
		BaseVersion: s.BaseVersion,
		Hash:        hash,
	}
}

// ScopedEntry is the subset of a queued mutation the scope decisions need.
// The browser queue carries the full dataset beside these fields; the pure
// logic never has to touch the bytes to decide whether they may move.
type ScopedEntry struct {
	WorkspaceID string
	OwnerUserID string
	BaseVersion int64
	Hash        string
}

// PushableEntry reports whether the signed-in account may send entry now.
//
// An empty signed-in user ID pushes NOTHING. This is deliberately fail-closed
// and stricter than Binding.OwnedBy's callers used to be: "identity unknown"
// previously classified nothing as foreign, so signing out (which clears the
// recorded user ID) left the previous account's queue pushable under the next
// account's token. Unknown identity is a reason to park work, not to send it;
// the work is kept and becomes pushable again once the identity resolves.
//
// An entry with no owner (queued before ownership was recorded, or while
// signed out) is adoptable by a KNOWN signed-in account, matching
// Binding.OwnedBy: refusing it would strand every pre-binding install.
func PushableEntry(entry ScopedEntry, signedInUserID string) bool {
	signedInUserID = strings.TrimSpace(signedInUserID)
	if signedInUserID == "" {
		return false
	}
	if strings.TrimSpace(entry.WorkspaceID) == "" {
		return false
	}
	owner := strings.TrimSpace(entry.OwnerUserID)
	if owner == "" {
		return true
	}
	return owner == signedInUserID
}

// SplitPushable divides queued work into entries the signed-in account may
// send and entries that must stay parked. Parked work is returned, never
// dropped: it belongs to another account (or waits for the identity to
// resolve), and discarding it would destroy unpushed data.
func SplitPushable(queue []ScopedEntry, signedInUserID string) (pushable, parked []ScopedEntry) {
	for _, e := range queue {
		if PushableEntry(e, signedInUserID) {
			pushable = append(pushable, e)
			continue
		}
		parked = append(parked, e)
	}
	return pushable, parked
}

// ParkedOwners names the distinct accounts parked work is waiting for, in
// first-seen order, so the sync status can say who the queue is waiting for
// instead of a bare count. Unowned entries name nobody and are skipped.
func ParkedOwners(parked []ScopedEntry) []string {
	var owners []string
	seen := map[string]bool{}
	for _, e := range parked {
		owner := strings.TrimSpace(e.OwnerUserID)
		if owner == "" || seen[owner] {
			continue
		}
		seen[owner] = true
		owners = append(owners, owner)
	}
	return owners
}

// Reasons CheckPullScope rejects a pulled snapshot. They are stable strings
// for logging; user-facing text is the caller's job.
const (
	// PullRejectWorkspaceChanged means the pulled workspace is not the one
	// currently active, so applying it would write one workspace's data over
	// another's.
	PullRejectWorkspaceChanged = "workspace-no-longer-active"
	// PullRejectAccountChanged means the signed-in account at apply time is
	// not the account the pull was requested under, so the snapshot the server
	// answered with cannot be assumed to belong to whoever is signed in now.
	PullRejectAccountChanged = "account-changed-during-pull"
	// PullRejectUnknownAccount means the current account is unknown, so the
	// client cannot prove this snapshot belongs to the current local scope.
	PullRejectUnknownAccount = "account-unknown-at-apply"
)

// CheckPullScope decides whether a pulled snapshot may still be applied at
// apply time. pulledWorkspaceID is the workspace the snapshot belongs to,
// activeWorkspaceID the workspace open NOW, requestedByUserID the account
// signed in when the pull started, and signedInUserID the account signed in
// now. A workspace switch or an account switch in another tab while the pull
// was in flight rejects the snapshot; the caller discards it and lets the
// new scope pull its own copy.
//
// An unknown current identity cannot establish ownership, even if it was
// also unknown at request time. Any transition (unknown→known, known→unknown,
// A→B) rejects because the snapshot was answered under a different identity
// than the one that would own it locally.
func CheckPullScope(pulledWorkspaceID, activeWorkspaceID, requestedByUserID, signedInUserID string) (bool, string) {
	pulledWorkspaceID = strings.TrimSpace(pulledWorkspaceID)
	if pulledWorkspaceID == "" || pulledWorkspaceID != strings.TrimSpace(activeWorkspaceID) {
		return false, PullRejectWorkspaceChanged
	}
	requestedByUserID = strings.TrimSpace(requestedByUserID)
	signedInUserID = strings.TrimSpace(signedInUserID)
	if requestedByUserID != signedInUserID {
		return false, PullRejectAccountChanged
	}
	if signedInUserID == "" {
		return false, PullRejectUnknownAccount
	}
	return true, ""
}
