// SPDX-License-Identifier: MIT

package syncstate

// SyncPhase is the single canonical client sync state shared by the sync
// engine, status chip, and Cloud settings UI.
type SyncPhase string

const (
	PhaseLocal                   SyncPhase = "local"
	PhaseSyncing                 SyncPhase = "syncing"
	PhaseSynced                  SyncPhase = "synced"
	PhaseOffline                 SyncPhase = "offline"
	PhaseConflict                SyncPhase = "conflict"
	PhaseRebind                  SyncPhase = "rebind"
	PhaseLocked                  SyncPhase = "locked"
	PhaseLockedDifferentPasscode SyncPhase = "locked-different-passcode"
	PhaseNeedsDataProtection     SyncPhase = "needs-data-protection"
	PhaseIdentityUnknown         SyncPhase = "identity-unknown"
	PhaseError                   SyncPhase = "error"
)

// IsAttention reports whether the phase requires explicit user intervention
// and must not be silently replaced by routine background transitions (such as
// an empty queue flush or an offline browser event).
func (p SyncPhase) IsAttention() bool {
	switch p {
	case PhaseConflict, PhaseRebind, PhaseLocked, PhaseLockedDifferentPasscode, PhaseNeedsDataProtection, PhaseIdentityUnknown:
		return true
	default:
		return false
	}
}

// CanRoutineTransitionTo reports whether a routine background status update
// (e.g. empty flush -> synced, navigator offline -> offline) may overwrite the
// current phase. Attention phases stay sticky until explicitly cleared by a
// real resolution, unlock, or successful authenticated sync.
func (p SyncPhase) CanRoutineTransitionTo(next SyncPhase) bool {
	if p == next {
		return true
	}
	if p.IsAttention() {
		// Routine "offline" or empty-queue "synced" must never mask an active
		// conflict, rebind, lock, or identity error.
		if next == PhaseOffline || next == PhaseSynced || next == PhaseLocal {
			return false
		}
	}
	return true
}

// NormalizePhase maps legacy or empty status strings onto a canonical SyncPhase.
func NormalizePhase(raw string) SyncPhase {
	switch SyncPhase(raw) {
	case PhaseLocal, PhaseSyncing, PhaseSynced, PhaseOffline, PhaseConflict,
		PhaseRebind, PhaseLocked, PhaseLockedDifferentPasscode,
		PhaseNeedsDataProtection, PhaseIdentityUnknown, PhaseError:
		return SyncPhase(raw)
	case "needs-rebind":
		return PhaseRebind
	default:
		return PhaseLocal
	}
}

// PullDecision describes how a version/sequence-based pull should proceed
// without consulting wall-clock timestamps.
type PullDecision int

const (
	// PullNoop means local state is already at or ahead of the server head.
	PullNoop PullDecision = iota
	// PullFastForward means local has no unpushed edits and can apply remote
	// updates directly in place.
	PullFastForward
	// PullReconcile means both local and remote have changes; the engine must
	// inspect record-level overlap to auto-merge or create a conflict checkpoint.
	PullReconcile
)

// DecidePullVersion chooses the pull action using monotonic server sequence
// numbers and local dirty state rather than wall-clock timestamps.
func DecidePullVersion(localAppliedSeq, remoteHeadSeq int64, hasPendingLocal bool) PullDecision {
	if remoteHeadSeq <= localAppliedSeq {
		return PullNoop
	}
	if !hasPendingLocal {
		return PullFastForward
	}
	return PullReconcile
}
