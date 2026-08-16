// SPDX-License-Identifier: MIT

package rules

// Applied reports which of a rule's apply-once actions would change a
// transaction's current state, and what to (C373).
//
// It exists so the three seams that fire rules — entry-time auto-categorization,
// the full backfill and a single-rule backfill — cannot drift apart on the
// question of when an action applies. Each of them previously spelled out the
// bill-link's "only when empty" rule inline, and each new action would have
// needed the same care in three places.
//
// The rules encoded here are deliberate and identical for all four actions: a
// standing instruction may FILL something in, and may never reverse a person's
// explicit choice. So a member is assigned only where there is none, and the two
// flags only ever go false → true.
type Applied struct {
	MemberID           string // non-empty when the member should be set
	Reviewed           bool   // true when the transaction should be marked reviewed
	ExcludeFromReports bool   // true when it should be excluded from reports
}

// Any reports whether the rule would change anything through these actions.
func (a Applied) Any() bool {
	return a.MemberID != "" || a.Reviewed || a.ExcludeFromReports
}

// ApplyOnce evaluates the apply-once actions against a transaction's current
// state. curMember is the transaction's existing MemberID, curReviewed and
// curExcluded its current flags.
func (r Rule) ApplyOnce(curMember string, curReviewed, curExcluded bool) Applied {
	var out Applied
	if r.SetMemberID != "" && curMember == "" {
		out.MemberID = r.SetMemberID
	}
	if r.SetReviewed && !curReviewed {
		out.Reviewed = true
	}
	if r.SetExcludeFromReports && !curExcluded {
		out.ExcludeFromReports = true
	}
	return out
}
