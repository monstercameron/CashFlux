// SPDX-License-Identifier: MIT

package rules

import "testing"

// ─── C373: apply-once actions fill gaps, they never reverse a choice ─────────

func TestApplyOnceFillsOnlyWhatIsMissing(t *testing.T) {
	r := Rule{
		ID: "r1", SetMemberID: "m-priya", SetReviewed: true, SetExcludeFromReports: true,
	}

	// A transaction with nothing set: all three apply.
	got := r.ApplyOnce("", false, false)
	if got.MemberID != "m-priya" || !got.Reviewed || !got.ExcludeFromReports {
		t.Errorf("on an empty transaction = %+v, want all three applied", got)
	}
	if !got.Any() {
		t.Error("Any() = false when three actions applied")
	}

	// Already attributed to someone else: the rule must NOT reassign. A standing
	// instruction that silently changes whose spending something was is the one
	// behaviour these actions exist to avoid.
	if got := r.ApplyOnce("m-marcus", false, false); got.MemberID != "" {
		t.Errorf("reassigned an attributed transaction to %q", got.MemberID)
	}

	// Already reviewed / already excluded: nothing to do, and nothing to undo.
	// (Member is already someone else's too, so there is genuinely nothing left.)
	got = r.ApplyOnce("m-marcus", true, true)
	if got.Reviewed || got.ExcludeFromReports {
		t.Errorf("re-applied flags that were already set: %+v", got)
	}
	if got.Any() {
		t.Error("Any() = true when there was nothing left to change")
	}
}

// A rule with none of these actions changes nothing, whatever the transaction
// looks like — so a caller can evaluate unconditionally.
func TestApplyOnceIsInertWithoutActions(t *testing.T) {
	var r Rule
	for _, tc := range []struct {
		member            string
		reviewed, exclude bool
	}{{"", false, false}, {"m-1", true, true}, {"", true, false}} {
		if got := r.ApplyOnce(tc.member, tc.reviewed, tc.exclude); got.Any() {
			t.Errorf("a rule with no actions reported %+v", got)
		}
	}
}

// There is deliberately no way to express the reverse: the fields are one-way.
// This documents that as a decision rather than an oversight.
func TestApplyOnceHasNoUnsetDirection(t *testing.T) {
	// SetReviewed false on a reviewed transaction must not un-review it.
	r := Rule{ID: "r1"}
	if got := r.ApplyOnce("m-1", true, true); got.Any() {
		t.Error("a rule with SetReviewed=false reported a change against a reviewed " +
			"transaction — false means \"this rule does not touch it\", not \"un-review\"")
	}
}
