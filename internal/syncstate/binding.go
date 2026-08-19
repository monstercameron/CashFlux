// SPDX-License-Identifier: MIT

package syncstate

import "strings"

// This file makes sync state answer the question it previously could not: whose
// workspace is this?
//
// A workspace id alone is not an identity. When the signed-in account changes —
// a second device account is created, a household member signs in, a pairing is
// redeemed — the local workspace id and everything queued against it stay
// exactly as they were, and the client keeps pushing them under a token that
// does not own that workspace. The server answers `workspace not found`, which
// is correct and unhelpful: the client retries forever because nothing in its
// own state records that the work belongs to somebody else.
//
// So sync state is keyed by (user id, workspace id), and an upload that cannot
// possibly succeed is stopped with a decision to make rather than retried.

// Binding is the pair that owns a piece of sync state: the authenticated user
// and the workspace. A zero UserID means the state predates user binding (or
// was written while signed out) — it is adoptable rather than foreign, because
// refusing to sync data that has never belonged to anyone else would strand
// every existing install.
type Binding struct {
	UserID      string
	WorkspaceID string
}

// Bound reports whether the binding names a user.
func (b Binding) Bound() bool { return strings.TrimSpace(b.UserID) != "" }

// Valid reports whether the binding names a workspace. A binding with no
// workspace cannot address anything.
func (b Binding) Valid() bool { return strings.TrimSpace(b.WorkspaceID) != "" }

// OwnedBy reports whether userID may act on this binding. Unbound state is
// adoptable by whoever is signed in; bound state belongs to exactly one user.
func (b Binding) OwnedBy(userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	if !b.Bound() {
		return true
	}
	return strings.TrimSpace(b.UserID) == userID
}

// UploadDecision is what the client should do with queued work right now.
type UploadDecision string

const (
	// UploadProceed means the queued mutation may be pushed.
	UploadProceed UploadDecision = "proceed"
	// UploadSignIn means nobody is signed in AND the server has already refused
	// this workspace, so signing in is the only thing that can change the
	// outcome.
	//
	// It is deliberately NOT returned merely because the identity is unknown.
	// Treating "we cannot say who you are" as "do not upload" stops sync dead on
	// every deployment whose server does not answer an identity lookup — token
	// auth, older servers, an offline first run. The server is the authority on
	// ownership and rejects what it should; the client's job is to stop retrying
	// what the server has already refused, not to pre-empt it.
	UploadSignIn UploadDecision = "sign-in"
	// UploadRebind means the signed-in user does not own the workspace the work
	// is queued against — either because the identity changed under it, or
	// because the server has said so. Retrying cannot succeed; the user has to
	// choose which workspace this data belongs to.
	UploadRebind UploadDecision = "rebind"
	// UploadNothing means there is nothing queued.
	UploadNothing UploadDecision = "nothing"
)

// ReasonWorkspaceNotFound is the server's answer when the authenticated user
// does not own the workspace being written. It is matched as a substring
// because it arrives wrapped in gRPC/HTTP error text.
const ReasonWorkspaceNotFound = "workspace not found"

// IsWorkspaceNotFound reports whether a failure reason is the server saying the
// signed-in user cannot address that workspace.
func IsWorkspaceNotFound(reason string) bool {
	return strings.Contains(strings.ToLower(reason), ReasonWorkspaceNotFound)
}

// DecideUpload says what to do with the next queued mutation for the signed-in
// user. lastReason is the failure the previous attempt reported, if any.
//
// The rule that matters: a workspace-not-found is TERMINAL for that binding.
// Before this, the client treated it as a transient failure and retried on
// every tick, which is why the settings page could sit at "1 change waiting to
// upload" indefinitely while nothing on either side was going to change.
func DecideUpload(signedInUserID string, next PendingMutation, lastReason string) UploadDecision {
	if strings.TrimSpace(next.WorkspaceID) == "" {
		return UploadNothing
	}
	if strings.TrimSpace(signedInUserID) == "" {
		// Unknown identity: proceed unless the server has already said no.
		if IsWorkspaceNotFound(lastReason) {
			return UploadSignIn
		}
		return UploadProceed
	}
	if !next.Binding().OwnedBy(signedInUserID) {
		return UploadRebind
	}
	if IsWorkspaceNotFound(lastReason) {
		return UploadRebind
	}
	return UploadProceed
}

// PendingFor returns the queued mutations the given user may push, and the ones
// belonging to somebody else. Nothing is deleted here: foreign work is data the
// user has not yet decided about, and a sync layer that quietly dropped it
// would be destroying the very thing a rebind is meant to rescue.
func PendingFor(queue []PendingMutation, userID string) (mine, foreign []PendingMutation) {
	for _, m := range queue {
		if m.Binding().OwnedBy(userID) {
			mine = append(mine, m)
			continue
		}
		foreign = append(foreign, m)
	}
	return mine, foreign
}

// Adopt stamps unbound queued work with the signed-in user, so state written
// before this binding existed (or while signed out) joins the current identity
// instead of being stranded. Work already bound to another user is left alone —
// adopting that would be a silent transfer of somebody else's data.
func Adopt(queue []PendingMutation, userID string) []PendingMutation {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return queue
	}
	out := make([]PendingMutation, 0, len(queue))
	for _, m := range queue {
		if !m.Binding().Bound() {
			m.UserID = userID
		}
		out = append(out, m)
	}
	return out
}

// Rebind repoints queued work from one binding to another — the write half of
// the account-switch flow, after the user has confirmed the mapping.
//
// It rewrites only entries matching `from` exactly, so a rebind of one
// workspace cannot sweep up unrelated queued work. Rebinding onto a binding
// that already has queued work COLLAPSES the two into the target's single
// entry, keeping the incoming one: the queue holds at most one snapshot per
// workspace by design, and the mutation being rebound is the one the user just
// chose to keep.
func Rebind(queue []PendingMutation, from, to Binding) []PendingMutation {
	if !from.Valid() || !to.Valid() {
		return queue
	}
	out := make([]PendingMutation, 0, len(queue))
	var moved []PendingMutation
	for _, m := range queue {
		if m.Binding() == from {
			m.UserID = to.UserID
			m.WorkspaceID = to.WorkspaceID
			moved = append(moved, m)
			continue
		}
		out = append(out, m)
	}
	for _, m := range moved {
		out = UpsertPending(out, m)
	}
	return out
}

// Binding returns the mutation's owning pair.
func (p PendingMutation) Binding() Binding {
	return Binding{UserID: p.UserID, WorkspaceID: p.WorkspaceID}
}

// RemoteWorkspace is the subset of a server-side workspace the adoption
// decision needs.
type RemoteWorkspace struct {
	ID        string
	UpdatedAt string
	Deleted   bool
}

// AdoptTarget names the workspace a device should OPEN after reconciling with
// the account, or "" to keep whatever is already open.
//
// Merging the account's workspaces into the switcher is not the same as opening
// one, and conflating them is what made a freshly signed-in browser look like
// sync had failed: it sits on a local workspace whose id the account has never
// heard of, so the pull finds nothing, seeds that empty workspace to the server
// as a brand new one, and leaves the person staring at an empty app while their
// records sit behind a switcher they have no reason to open.
//
// The three guards are what make this safe rather than destructive. The open
// workspace must never have synced, must hold nothing the user typed, and must
// be unknown to the account — under all three there is provably nothing on the
// device to lose, because the only thing replaced is an empty or sample
// workspace the app itself invented.
func AdoptTarget(activeID string, activeHasSynced, activeHasUserData bool, remote []RemoteWorkspace) string {
	activeID = strings.TrimSpace(activeID)
	if activeID == "" || activeHasSynced || activeHasUserData {
		return ""
	}
	best := ""
	bestUpdated := ""
	for _, w := range remote {
		id := strings.TrimSpace(w.ID)
		if id == "" || w.Deleted {
			continue
		}
		// The account already knows this device's workspace, so the ordinary
		// pull addresses it correctly and there is nothing to adopt.
		if id == activeID {
			return ""
		}
		// Most recently updated wins: with several workspaces on the account,
		// the one worked on last is the best guess at which to open, and the
		// rest stay one click away in the switcher.
		if best == "" || w.UpdatedAt > bestUpdated {
			best, bestUpdated = id, w.UpdatedAt
		}
	}
	return best
}

// DatasetForeign reports whether the dataset sitting in local storage belongs to
// a DIFFERENT account than the one signed in now.
//
// Local per-workspace state — the dataset itself and its sync metadata — is
// keyed by workspace, never by account. That is fine while a browser has one
// account for ever, and it loses data the moment two accounts share a browser:
// signing out deliberately keeps the dataset (unpushed work is the user's, and
// signing out is not a request to discard it), so the next account inherits
// whatever the previous one left loaded. If that inherited copy then looks
// "newer" than the arriving account's server snapshot, it wins the
// last-write-wins comparison and is pushed up over their real data.
//
// An empty owner is adoptable rather than foreign, for the same reason
// Binding.OwnedBy treats unbound state that way: a dataset written before
// ownership was recorded belongs to whoever is here now, and refusing to sync it
// would strand every existing install on the upgrade.
func DatasetForeign(ownerUserID, signedInUserID string) bool {
	signedInUserID = strings.TrimSpace(signedInUserID)
	if signedInUserID == "" {
		// Unknown identity classifies nothing as foreign — see DecideUpload. The
		// server is the authority on ownership, and pre-empting it here would
		// stop sync dead on any deployment whose identity lookup does not answer.
		return false
	}
	return !Binding{UserID: ownerUserID}.OwnedBy(signedInUserID)
}
