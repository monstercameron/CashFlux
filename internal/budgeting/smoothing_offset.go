// SPDX-License-Identifier: MIT

package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smoothing"
)

// SmoothingLandingItems returns the offset and per-recurring breakdown so the
// budget row can explain the amount ("no black boxes"):
// each item names the landed bill and the amount its set-aside covered.
func SmoothingLandingItems(budget domain.Budget, recurrings []domain.Recurring, posted []domain.Transaction, start, end time.Time) (int64, []CommittedItem) {
	var offset int64
	var items []CommittedItem
	for _, r := range recurrings {
		if r.CategoryID == "" || !budget.TracksCategory(r.CategoryID) || !r.Smooths() {
			continue
		}
		mag := abs64(r.Amount.Amount)
		if mag == 0 || !smoothing.LandsIn(r, start, end) {
			continue
		}
		n := countPostedMatches(budget, posted, r, mag, start, end)
		if expected := len(smoothing.OccurrencesIn(r, start, end)); n > expected {
			n = expected
		}
		if n == 0 {
			continue
		}
		amt := mag * int64(n)
		offset += amt
		items = append(items, CommittedItem{
			RecurringID: r.ID, Label: r.Label,
			Amount: money.New(amt, r.Amount.Currency), Smoothed: true,
		})
	}
	return offset, items
}

// ApplySmoothingOffset returns the Status adjusted by a landing-period offset:
// Spent is reduced (never below zero), Remaining raised by the same amount, and
// Percent/State re-derived with the standard thresholds so the row's tone and
// meter agree with the adjusted figures. A zero or negative offset returns the
// Status unchanged.
func ApplySmoothingOffset(st Status, offset int64, nearThreshold float64) Status {
	if offset <= 0 {
		return st
	}
	if offset > st.Spent.Amount {
		offset = st.Spent.Amount
	}
	if offset <= 0 {
		return st
	}
	st.Spent = money.New(st.Spent.Amount-offset, st.Spent.Currency)
	st.Remaining = money.New(st.Remaining.Amount+offset, st.Remaining.Currency)
	limit := money.New(st.Spent.Amount+st.Remaining.Amount, st.Spent.Currency)
	st.Percent = percent(st.Spent, limit)
	st.State = classifyDirected(st.Budget.Direction, st.Spent, limit, nearThreshold)
	return st
}
