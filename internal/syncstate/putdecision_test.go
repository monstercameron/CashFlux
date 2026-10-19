// SPDX-License-Identifier: MIT

package syncstate

import (
	"testing"
	"time"
)

var (
	serverT = time.Date(2026, 8, 31, 12, 0, 30, 0, time.UTC)
	// A browser 30 seconds behind the server. Ordinary drift; no NTP required to
	// produce it, and nothing on the device tells the user it is happening.
	skewedClientT = time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
)

// THE BUG THIS FILE EXISTS FOR.
//
// A device whose clock trails the server's used to lose every push forever: the
// comparison was client-wall-clock vs server-stamp, two different clocks
// measuring different events. With a version the decision no longer involves a
// clock at all.
func TestASlowClockNoLongerBlocksEveryPush(t *testing.T) {
	in := PutInput{
		Exists: true, ClientBaseVersion: 7, CurrentVersion: 7,
		ClientUpdatedAt: skewedClientT, CurrentUpdatedAt: serverT,
	}
	got := DecidePut(in)
	if !got.Accept {
		t.Fatalf("a current edit was rejected for clock skew alone: %+v", got)
	}
	if got.Reason != "version-match" {
		t.Errorf("Reason = %q, want version-match (the clock must not decide this)", got.Reason)
	}
	// And prove the skew really would have rejected it on the old rule.
	if !in.ClientUpdatedAt.Before(in.CurrentUpdatedAt) {
		t.Fatal("fixture no longer reproduces clock skew; the test above proves nothing")
	}
}

func TestAGenuinelyStaleEditIsStillRejected(t *testing.T) {
	// The protection that must survive the fix: another device landed a write
	// (version 8) after this client read version 7, so this push would clobber it.
	got := DecidePut(PutInput{
		Exists: true, ClientBaseVersion: 7, CurrentVersion: 8,
		// Clock says the client is NEWER — and it must still lose, because the
		// version says it never saw version 8.
		ClientUpdatedAt: serverT.Add(time.Hour), CurrentUpdatedAt: serverT,
	})
	if got.Accept {
		t.Error("a push based on a superseded version was accepted; that clobbers the other device")
	}
	if got.Reason != "version-stale" {
		t.Errorf("Reason = %q, want version-stale", got.Reason)
	}
}

func TestAFastClockCannotStealTheDecision(t *testing.T) {
	// The mirror of the skew bug: a device whose clock runs AHEAD used to win
	// every race regardless of what it had actually seen.
	got := DecidePut(PutInput{
		Exists: true, ClientBaseVersion: 3, CurrentVersion: 9,
		ClientUpdatedAt: serverT.Add(48 * time.Hour), CurrentUpdatedAt: serverT,
	})
	if got.Accept {
		t.Error("a client two days ahead overwrote six versions it never saw")
	}
}

func TestAFirstWriteIsAlwaysAccepted(t *testing.T) {
	// Nothing to race. This must not depend on clocks or versions at all.
	for _, in := range []PutInput{
		{Exists: false},
		{Exists: false, ClientUpdatedAt: skewedClientT, CurrentUpdatedAt: serverT},
		{Exists: false, ClientBaseVersion: 0, CurrentVersion: 99},
	} {
		if got := DecidePut(in); !got.Accept || got.Reason != "new" {
			t.Errorf("first write rejected: %+v → %+v", in, got)
		}
	}
}

func TestForceOverridesEverything(t *testing.T) {
	// The user was shown what they would overwrite and said yes. Nothing below
	// may second-guess that.
	got := DecidePut(PutInput{
		Exists: true, Force: true, ClientBaseVersion: 1, CurrentVersion: 500,
		ClientUpdatedAt: skewedClientT, CurrentUpdatedAt: serverT,
	})
	if !got.Accept || got.Reason != "forced" {
		t.Errorf("force did not win: %+v", got)
	}
}

func TestAClientAheadOfTheServerIsRejected(t *testing.T) {
	// A restore from backup, or a rolled-back server, leaves a client holding a
	// version the server no longer reports. The earlier rule accepted these
	// (>=), which let a stale or foreign queue overwrite the restored data —
	// the exact silent overwrite the version check exists to prevent. The
	// client is not stranded: the rejection carries current server state, it
	// pulls, and a person who wants its copy to win can Force.
	got := DecidePut(PutInput{Exists: true, ClientBaseVersion: 12, CurrentVersion: 4})
	if got.Accept {
		t.Error("a client ahead of the server overwrote data it never based an edit on")
	}
	if got.Reason != "version-ahead" {
		t.Errorf("Reason = %q, want version-ahead (distinct from stale: the fix is pull, not rebase)", got.Reason)
	}
	// …and Force remains the deliberate way through.
	forced := DecidePut(PutInput{Exists: true, Force: true, ClientBaseVersion: 12, CurrentVersion: 4})
	if !forced.Accept || forced.Reason != "forced" {
		t.Errorf("forced ahead push = %+v, want accepted/forced", forced)
	}
}

func TestAPushToADeletedWorkspaceIsRejected(t *testing.T) {
	// A queued push from before the delete must not resurrect the workspace.
	// This holds on both the version path and the legacy clock path — the
	// tombstone refuses before either is consulted.
	for _, in := range []PutInput{
		{Exists: true, Deleted: true, ClientBaseVersion: 4, CurrentVersion: 4},
		{Exists: true, Deleted: true, ClientUpdatedAt: serverT.Add(time.Hour), CurrentUpdatedAt: serverT},
	} {
		got := DecidePut(in)
		if got.Accept || got.Reason != "deleted" {
			t.Errorf("%+v → %+v, want rejected/deleted", in, got)
		}
	}
	// Force is still the explicit resurrect-on-purpose path.
	got := DecidePut(PutInput{Exists: true, Deleted: true, Force: true, ClientBaseVersion: 1, CurrentVersion: 9})
	if !got.Accept || got.Reason != "forced" {
		t.Errorf("forced push to tombstone = %+v, want accepted/forced", got)
	}
}

// TestDecidePutTable is the whole decision table in one place — every branch
// DecidePut can take, with the reason it must report.
func TestDecidePutTable(t *testing.T) {
	cases := []struct {
		name       string
		in         PutInput
		wantAccept bool
		wantReason string
	}{
		{"new workspace", PutInput{Exists: false}, true, "new"},
		{"new workspace ignores versions", PutInput{Exists: false, ClientBaseVersion: 9, CurrentVersion: 3}, true, "new"},
		{"forced over stale version", PutInput{Exists: true, Force: true, ClientBaseVersion: 1, CurrentVersion: 9}, true, "forced"},
		{"forced over tombstone", PutInput{Exists: true, Force: true, Deleted: true, ClientBaseVersion: 9, CurrentVersion: 9}, true, "forced"},
		{"tombstone rejects matching version", PutInput{Exists: true, Deleted: true, ClientBaseVersion: 9, CurrentVersion: 9}, false, "deleted"},
		{"tombstone rejects legacy newer clock", PutInput{Exists: true, Deleted: true, ClientUpdatedAt: serverT.Add(time.Hour), CurrentUpdatedAt: serverT}, false, "deleted"},
		{"version match", PutInput{Exists: true, ClientBaseVersion: 5, CurrentVersion: 5}, true, "version-match"},
		{"explicit zero version is not a legacy peer", PutInput{Exists: true, HasBaseVersion: true, ClientBaseVersion: 0, CurrentVersion: 1}, false, "version-stale"},
		{"version stale by one", PutInput{Exists: true, ClientBaseVersion: 4, CurrentVersion: 5}, false, "version-stale"},
		{"version ahead by one", PutInput{Exists: true, ClientBaseVersion: 6, CurrentVersion: 5}, false, "version-ahead"},
		{"legacy newer clock", PutInput{Exists: true, ClientUpdatedAt: serverT.Add(time.Second), CurrentUpdatedAt: serverT}, true, "clock-newer"},
		{"legacy tie", PutInput{Exists: true, ClientUpdatedAt: serverT, CurrentUpdatedAt: serverT}, true, "clock-newer"},
		{"legacy older clock", PutInput{Exists: true, ClientUpdatedAt: skewedClientT, CurrentUpdatedAt: serverT}, false, "clock-older"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecidePut(tc.in)
			if got.Accept != tc.wantAccept || got.Reason != tc.wantReason {
				t.Fatalf("DecidePut(%+v) = %+v, want accept=%v reason=%q", tc.in, got, tc.wantAccept, tc.wantReason)
			}
		})
	}
}

// ── the legacy path: clients that cannot send a version ──────────────────────

func TestALegacyClientKeepsTheOldTimestampRule(t *testing.T) {
	// A server upgrade must not stop older builds syncing, and their behaviour
	// must not change either — they cannot be fixed from here.
	older := DecidePut(PutInput{
		Exists: true, ClientBaseVersion: 0,
		ClientUpdatedAt: skewedClientT, CurrentUpdatedAt: serverT,
	})
	if older.Accept {
		t.Error("legacy older-timestamp push was accepted; the old rule rejected it")
	}
	newer := DecidePut(PutInput{
		Exists: true, ClientBaseVersion: 0,
		ClientUpdatedAt: serverT.Add(time.Second), CurrentUpdatedAt: serverT,
	})
	if !newer.Accept {
		t.Error("legacy newer-timestamp push was rejected; the old rule accepted it")
	}
}

func TestALegacyTieIsAcceptedExactlyAsBefore(t *testing.T) {
	// Deliberately preserved, not fixed. The client used to copy the server's own
	// timestamp forward so its next push was "contemporaneous"; rejecting a tie
	// would break that flow for builds that cannot be changed. Version-carrying
	// clients never reach this branch.
	got := DecidePut(PutInput{
		Exists: true, ClientBaseVersion: 0,
		ClientUpdatedAt: serverT, CurrentUpdatedAt: serverT,
	})
	if !got.Accept {
		t.Error("a legacy tie was rejected, changing behaviour for clients that cannot be updated")
	}
}

func TestZeroBaseVersionSelectsLegacyRatherThanMeaningVersionZero(t *testing.T) {
	// The distinction the whole compatibility story rests on: 0 means "did not
	// send one", NOT "based on version zero". Treating it as a version would
	// reject every legacy push against any existing row.
	got := DecidePut(PutInput{
		Exists: true, ClientBaseVersion: 0, CurrentVersion: 5,
		ClientUpdatedAt: serverT.Add(time.Minute), CurrentUpdatedAt: serverT,
	})
	if !got.Accept {
		t.Error("a legacy client was rejected because 0 was read as a stale version")
	}
	if got.Reason != "clock-newer" {
		t.Errorf("Reason = %q, want the legacy clock path", got.Reason)
	}
}

func TestTheDecisionIsDeterministic(t *testing.T) {
	// Same input, same answer — a sync rule that varies run to run is unfixable.
	in := PutInput{Exists: true, ClientBaseVersion: 4, CurrentVersion: 6}
	first := DecidePut(in)
	for i := 0; i < 50; i++ {
		if got := DecidePut(in); got != first {
			t.Fatalf("run %d differed: %+v vs %+v", i, got, first)
		}
	}
}

func TestEveryReasonIsDistinctAndNonEmpty(t *testing.T) {
	// The reason is logged and drives the client's retry, so two different
	// outcomes must never share a tag.
	seen := map[string]string{}
	cases := map[string]PutInput{
		"new":           {Exists: false},
		"forced":        {Exists: true, Force: true},
		"deleted":       {Exists: true, Deleted: true, ClientBaseVersion: 2, CurrentVersion: 2},
		"version-match": {Exists: true, ClientBaseVersion: 2, CurrentVersion: 2},
		"version-stale": {Exists: true, ClientBaseVersion: 1, CurrentVersion: 2},
		"version-ahead": {Exists: true, ClientBaseVersion: 3, CurrentVersion: 2},
		"clock-newer":   {Exists: true, ClientUpdatedAt: serverT.Add(time.Second), CurrentUpdatedAt: serverT},
		"clock-older":   {Exists: true, ClientUpdatedAt: skewedClientT, CurrentUpdatedAt: serverT},
	}
	for want, in := range cases {
		got := DecidePut(in)
		if got.Reason != want {
			t.Errorf("%+v → Reason %q, want %q", in, got.Reason, want)
		}
		if prev, dup := seen[got.Reason]; dup {
			t.Errorf("reason %q produced by two different cases (%s)", got.Reason, prev)
		}
		seen[got.Reason] = want
	}
}
