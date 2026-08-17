// SPDX-License-Identifier: MIT

package uistate_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// ─── C409: an alert's id is a durable reference to what it is about ──────────

func TestResolutionForBillDue(t *testing.T) {
	got := uistate.ResolutionFor("default-bill-due@acct-card@2026-08-22")
	if !got.Resolvable() || got.Kind != uistate.ResolveBillPaid {
		t.Fatalf("got %+v, want a bill-paid resolution", got)
	}
	if got.EntityID != "acct-card" {
		t.Errorf("EntityID = %q, want acct-card", got.EntityID)
	}
	if want := time.Date(2026, time.August, 22, 0, 0, 0, 0, time.UTC); !got.Occurrence.Equal(want) {
		t.Errorf("Occurrence = %s, want %s", got.Occurrence, want)
	}
}

// A merged obligation's bill id is "recurring:<id>", which contains its own
// separator — splitting from the left would cut it in half and mark the wrong
// thing paid.
func TestResolutionForBillWithASeparatorInItsID(t *testing.T) {
	got := uistate.ResolutionFor("default-bill-due@recurring:rec-studentloan@2026-09-05")
	if got.EntityID != "recurring:rec-studentloan" {
		t.Errorf("EntityID = %q, want the whole recurring id", got.EntityID)
	}
	if got.Occurrence.Day() != 5 || got.Occurrence.Month() != time.September {
		t.Errorf("Occurrence = %s, want 2026-09-05", got.Occurrence)
	}
}

func TestResolutionForStaleBalance(t *testing.T) {
	got := uistate.ResolutionFor("default-stale@acct-hysa@2026-W33")
	if got.Kind != uistate.ResolveBalanceConfirmed || got.EntityID != "acct-hysa" {
		t.Errorf("got %+v, want a balance-confirmed resolution for acct-hysa", got)
	}
	if !got.Occurrence.IsZero() {
		t.Error("a balance confirmation is about now, not about an occurrence date")
	}
}

// Anything unrecognised resolves to NOTHING rather than to a guess: a wrong
// resolution writes to the wrong entity, which is worse than offering no button.
func TestResolutionForRefusesToGuess(t *testing.T) {
	for _, id := range []string{
		"",
		"default-digest@2026-W33",
		"default-budget@bud-food:over@2026-08",
		"default-large@lorem",
		"default-bill-due@acct-card@not-a-date",
		"default-bill-due@nodate",
		"default-stale@nokey",
		"no-at-sign",
		"default-bill-due@",
	} {
		if got := uistate.ResolutionFor(id); got.Resolvable() {
			t.Errorf("ResolutionFor(%q) = %+v, want no resolution", id, got)
		}
	}
}

// C403: a to-do reminder resolves by completing the to-do.
func TestResolutionForTaskReminder(t *testing.T) {
	r := uistate.ResolutionFor("default-task-reminder@t-42@2026-08-18")
	if r.Kind != uistate.ResolveTaskDone {
		t.Fatalf("Kind = %q, want %q", r.Kind, uistate.ResolveTaskDone)
	}
	if r.EntityID != "t-42" {
		t.Errorf("EntityID = %q, want %q", r.EntityID, "t-42")
	}
	if !r.Resolvable() {
		t.Error("a task reminder should offer a direct action")
	}
	// A malformed key resolves to nothing rather than to a task id that would
	// complete the wrong to-do.
	if got := uistate.ResolutionFor("default-task-reminder@2026-08-18"); got.Resolvable() {
		t.Errorf("a key with no task id resolved to %+v", got)
	}
}

// ─── C407: the member lens ───────────────────────────────────────────────────

// A member lens must NOT hide household-wide alerts. "What needs me" has to
// include the things that need somebody and were never assigned, or an
// unassigned overdue bill goes permanently unseen in a household where everyone
// uses their own view.
func TestMemberFilterKeepsHouseholdWideAlerts(t *testing.T) {
	mine := uistate.FeedItem{ID: "a", MemberID: "m-cam"}
	theirs := uistate.FeedItem{ID: "b", MemberID: "m-alex"}
	everyone := uistate.FeedItem{ID: "c"}

	if !mine.MatchesMemberFilter("m-cam") {
		t.Error("my own alert was filtered out of my lens")
	}
	if theirs.MatchesMemberFilter("m-cam") {
		t.Error("another member's alert passed my lens")
	}
	if !everyone.MatchesMemberFilter("m-cam") {
		t.Error("an unassigned alert was hidden — this is how work nobody claimed goes unseen")
	}
	// No lens shows everything.
	for _, it := range []uistate.FeedItem{mine, theirs, everyone} {
		if !it.MatchesMemberFilter("") {
			t.Errorf("%s was hidden with no lens set", it.ID)
		}
	}
}
