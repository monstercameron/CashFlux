// SPDX-License-Identifier: MIT

package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ContributingCount returns how many TRANSACTIONS contributed to a budget over
// [start, end). It exists for the "why did this go over" read (SM-4), where the
// interesting question is whether the household shopped more often or merely
// spent more each time — and answering that needs a count that agrees with the
// Spent figure beside it.
//
// The matching mirrors spentCovered branch for branch: charges excluded from
// reports don't count, a tracked TAG counts the charge in full and once (taking
// priority over category matching, so nothing is counted twice), a split
// transaction counts when any covered line is owned in scope, and an unsplit one
// counts when its own category is covered and it is owned in scope.
//
// A split transaction counts ONCE however many of its lines the budget covers.
// The count answers "how many times did you shop", not "how many ledger lines
// exist", and a receipt broken into four categories was still one trip.
//
// covers is the category predicate — pass budget.TracksCategory for a plain
// budget, or the descendant-aware predicate EvaluateRollup builds for a parent.
// A nil covers falls back to the budget's own category.
func ContributingCount(budget domain.Budget, all []domain.Transaction, start, end time.Time, covers func(string) bool) int {
	if covers == nil {
		covers = budget.TracksCategory
	}
	// The same refund-pair netting Spent applies (XC2). Without it a refunded
	// charge would still be counted as a trip while contributing no money, and the
	// count would quietly disagree with the figure it is shown beside.
	all = nettedForSpending(all)
	n := 0
	for _, t := range all {
		if !t.CountsInReports() || !matchesScope(budget, t, start, end) {
			continue
		}
		if budget.TracksAnyTag(t.Tags) {
			if ownsScope(budget, t.MemberID) {
				n++
			}
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				if covers(s.CategoryID) && ownsScope(budget, s.LineOwner(t.MemberID)) {
					n++
					break // one trip, however many lines it was broken into
				}
			}
			continue
		}
		if ownsScope(budget, t.MemberID) && covers(t.CategoryID) {
			n++
		}
	}
	return n
}
