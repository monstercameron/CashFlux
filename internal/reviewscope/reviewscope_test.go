// SPDX-License-Identifier: MIT

package reviewscope

import (
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   Scope
		want Scope
	}{
		{ScopeCurrentView, ScopeCurrentView},
		{ScopeEntireQueue, ScopeEntireQueue},
		{"", ScopeEntireQueue},
		{"whatever-a-stale-build-persisted", ScopeEntireQueue},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultFollowsTheLedger(t *testing.T) {
	if got := Default(true); got != ScopeCurrentView {
		t.Errorf("Default(filtered) = %q, want %q — a user who filtered and then pressed Review has already said what they want", got, ScopeCurrentView)
	}
	if got := Default(false); got != ScopeEntireQueue {
		t.Errorf("Default(unfiltered) = %q, want %q", got, ScopeEntireQueue)
	}
}

func TestNormalizeCommitFailsSmall(t *testing.T) {
	if got := NormalizeCommit(""); got != CommitCharge {
		t.Errorf("NormalizeCommit(\"\") = %q, want %q — an unreadable commit scope must touch less, not more", got, CommitCharge)
	}
	if got := NormalizeCommit("group"); got != CommitCharge {
		t.Errorf("NormalizeCommit(unknown) = %q, want %q", got, CommitCharge)
	}
	if got := NormalizeCommit(CommitMerchant); got != CommitMerchant {
		t.Errorf("NormalizeCommit(merchant) = %q, want %q", got, CommitMerchant)
	}
}

// TestTargetsChargeScopeIgnoresTheGroup is the C653 guard in its smallest form:
// the card depicts one charge, so the default commit writes one charge, however
// many siblings the merchant has.
func TestTargetsChargeScopeIgnoresTheGroup(t *testing.T) {
	got := Targets(CommitCharge, "t1", []string{"t1", "t2", "t3"})
	if want := []string{"t1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Targets(charge) = %v, want %v", got, want)
	}
}

func TestTargetsMerchantScopeCoversTheGroupExactlyOnce(t *testing.T) {
	got := Targets(CommitMerchant, "t2", []string{"t1", "t2", "t3", "t2"})
	want := []string{"t2", "t1", "t3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Targets(merchant) = %v, want %v (current charge first, no duplicates)", got, want)
	}
}

// TestTargetsMerchantScopeIncludesACurrentChargeMissingFromTheGroup covers the
// race where the group list was built a render before the click: the charge the
// user is looking at must still be written.
func TestTargetsMerchantScopeIncludesACurrentChargeMissingFromTheGroup(t *testing.T) {
	got := Targets(CommitMerchant, "t9", []string{"t1", "t2"})
	want := []string{"t9", "t1", "t2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Targets = %v, want %v", got, want)
	}
}

func TestTargetsWithoutACurrentChargeWritesNothing(t *testing.T) {
	for _, s := range []CommitScope{CommitCharge, CommitMerchant} {
		if got := Targets(s, "", []string{"t1", "t2"}); len(got) != 0 {
			t.Errorf("Targets(%q, \"\") = %v, want none — there is no card to act on", s, got)
		}
	}
}

// TestTargetsSkipsBlankGroupIDs keeps a malformed group from producing a write
// target that matches no transaction and silently lowers the confirmed count.
func TestTargetsSkipsBlankGroupIDs(t *testing.T) {
	got := Targets(CommitMerchant, "t1", []string{"", "t2", ""})
	if want := []string{"t1", "t2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Targets = %v, want %v", got, want)
	}
}

func TestRestrictIntersectsInQueueOrder(t *testing.T) {
	got := Restrict([]string{"c", "a", "b", "d"}, []string{"b", "d", "a", "z"})
	want := []string{"a", "b", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Restrict = %v, want %v (queue order preserved)", got, want)
	}
}

func TestRestrictEmptyInputs(t *testing.T) {
	if got := Restrict(nil, []string{"a"}); len(got) != 0 {
		t.Errorf("Restrict(nil, …) = %v, want none", got)
	}
	// An empty visible set means the ledger's filters match nothing, which must
	// review nothing — not everything.
	if got := Restrict([]string{"a", "b"}, nil); len(got) != 0 {
		t.Errorf("Restrict(…, nil) = %v, want none — a filter that matches nothing must not fall back to the whole queue", got)
	}
}

// TestCardIndexHoldsTheMerchantThroughAReorder is the C653 follow-up guard.
//
// reviewqueue.GroupByMerchant ranks by (confidence, group size, key), so
// confirming ONE charge shrinks its group and can push it behind a
// same-confidence neighbour. A cursor that was only an index would then be
// pointing at a merchant the user had never been shown — one click, an
// unpredictable card.
func TestCardIndexHoldsTheMerchantThroughAReorder(t *testing.T) {
	before := []string{"b-shop", "c-shop", "d-shop"} // b(3) c(3) d(2)
	if got := CardIndex(before, 0, "b-shop"); got != 0 {
		t.Fatalf("setup: CardIndex = %d, want 0", got)
	}
	// b drops to 2 items and re-ranks behind c.
	after := []string{"c-shop", "b-shop", "d-shop"}
	if got := CardIndex(after, 0, "b-shop"); got != 1 {
		t.Errorf("CardIndex = %d, want 1 — the card jumped to a merchant the user "+
			"has not been through (C653)", got)
	}
}

// TestCardIndexFallsBackToThePositionWhenTheMerchantIsGone: a merchant-wide
// commit clears the group entirely, and then the SLOT is the right answer —
// whatever moved into it is the next piece of work.
func TestCardIndexFallsBackToThePositionWhenTheMerchantIsGone(t *testing.T) {
	if got := CardIndex([]string{"c-shop", "d-shop"}, 0, "b-shop"); got != 0 {
		t.Errorf("CardIndex = %d, want 0", got)
	}
	if got := CardIndex([]string{"c-shop", "d-shop", "e-shop"}, 2, "b-shop"); got != 2 {
		t.Errorf("CardIndex = %d, want 2", got)
	}
}

func TestCardIndexClampsAndHandlesAnEmptyQueue(t *testing.T) {
	if got := CardIndex(nil, 5, "b-shop"); got != 0 {
		t.Errorf("CardIndex(empty) = %d, want 0", got)
	}
	if got := CardIndex([]string{"a", "b"}, 9, ""); got != 0 {
		t.Errorf("CardIndex(out of range) = %d, want 0 (wrap to the top)", got)
	}
	if got := CardIndex([]string{"a", "b"}, -1, ""); got != 0 {
		t.Errorf("CardIndex(negative) = %d, want 0", got)
	}
}
