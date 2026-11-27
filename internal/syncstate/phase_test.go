// SPDX-License-Identifier: MIT

package syncstate

import "testing"

func TestSyncPhaseRoutineTransitions(t *testing.T) {
	cases := []struct {
		name string
		cur  SyncPhase
		next SyncPhase
		want bool
	}{
		{"synced to offline", PhaseSynced, PhaseOffline, true},
		{"offline to syncing", PhaseOffline, PhaseSyncing, true},
		{"conflict resists offline", PhaseConflict, PhaseOffline, false},
		{"conflict resists empty synced", PhaseConflict, PhaseSynced, false},
		{"conflict resists local", PhaseConflict, PhaseLocal, false},
		{"conflict allows syncing", PhaseConflict, PhaseSyncing, true},
		{"locked resists offline", PhaseLocked, PhaseOffline, false},
		{"locked resists synced", PhaseLocked, PhaseSynced, false},
		{"different passcode resists synced", PhaseLockedDifferentPasscode, PhaseSynced, false},
		{"rebind resists offline", PhaseRebind, PhaseOffline, false},
		{"needs data protection resists synced", PhaseNeedsDataProtection, PhaseSynced, false},
		{"identity unknown resists offline", PhaseIdentityUnknown, PhaseOffline, false},
		{"error allows syncing", PhaseError, PhaseSyncing, true},
		{"error allows offline", PhaseError, PhaseOffline, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cur.CanRoutineTransitionTo(tc.next); got != tc.want {
				t.Fatalf("%q.CanRoutineTransitionTo(%q) = %v, want %v", tc.cur, tc.next, got, tc.want)
			}
		})
	}
}

func TestNormalizePhase(t *testing.T) {
	if got := NormalizePhase("needs-rebind"); got != PhaseRebind {
		t.Fatalf("NormalizePhase(needs-rebind) = %q, want %q", got, PhaseRebind)
	}
	if got := NormalizePhase(""); got != PhaseLocal {
		t.Fatalf("NormalizePhase('') = %q, want %q", got, PhaseLocal)
	}
	if got := NormalizePhase("locked"); got != PhaseLocked {
		t.Fatalf("NormalizePhase(locked) = %q, want %q", got, PhaseLocked)
	}
	if got := NormalizePhase("unknown-garbage"); got != PhaseLocal {
		t.Fatalf("NormalizePhase(unknown) = %q, want %q", got, PhaseLocal)
	}
}

func TestDecidePullVersion(t *testing.T) {
	cases := []struct {
		name       string
		localSeq   int64
		remoteHead int64
		dirty      bool
		want       PullDecision
	}{
		{"same head clean", 5, 5, false, PullNoop},
		{"same head dirty", 5, 5, true, PullNoop},
		{"ahead of server", 6, 5, false, PullNoop},
		{"behind clean", 3, 5, false, PullFastForward},
		{"behind dirty", 3, 5, true, PullReconcile},
		{"fresh device remote has data", 0, 1, false, PullFastForward},
		{"fresh device with local edits", 0, 1, true, PullReconcile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecidePullVersion(tc.localSeq, tc.remoteHead, tc.dirty); got != tc.want {
				t.Fatalf("DecidePullVersion(%d, %d, %v) = %v, want %v", tc.localSeq, tc.remoteHead, tc.dirty, got, tc.want)
			}
		})
	}
}
