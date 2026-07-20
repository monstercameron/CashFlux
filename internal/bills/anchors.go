// SPDX-License-Identifier: MIT

package bills

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// LiabilityAnchors reports which recurring flows are ANCHORED to a liability —
// that is, which ones settle a real debt the household owes (a mortgage, a car
// loan, a card statement) rather than merely moving money out of a funding
// account. The result maps a recurring flow's id to the liability account id its
// payments settle; flows with no such anchor are absent.
//
// It exists so the two surfaces that need this judgment cannot drift apart. The
// agenda already learns it as a side effect of DedupeObligations — collapsing a
// liability's own statement bill onto the recurring flow that pays it records
// the liability in Bill.AnchorAccountID — and the roster's "Bills" lens asks the
// same question of the same flows. Answering it twice, in two places, is how a
// page ends up claiming a flow is account-tied in one section and free-floating
// in the next. So the answer is computed ONCE, here, by running exactly the
// pipeline the agenda runs.
//
// Note this is deliberately NOT domain.Recurring.AccountID: that field names the
// FUNDING account an occurrence posts from, which nearly every flow carries, and
// reading it as the anchor makes "account-tied" mean "exists".
func LiabilityAnchors(accounts []domain.Account, recurring []domain.Recurring, now, until time.Time) map[string]string {
	out := map[string]string{}
	merged := DedupeObligations(OccurrencesWithin(accounts, recurring, now, until), recurring)
	for _, b := range merged {
		if b.AnchorAccountID == "" {
			continue
		}
		if rid, ok := RecurringIDFromAccount(b.AccountID); ok {
			out[rid] = b.AnchorAccountID
		}
	}
	return out
}
