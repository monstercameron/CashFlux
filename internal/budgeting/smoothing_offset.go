// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smoothing"
)

// SmoothingLandingItemsWithRates returns checked landing offsets in the
// budget's currency. Missing FX and overflow remain visible to the caller.
func SmoothingLandingItemsWithRates(budget domain.Budget, recurrings []domain.Recurring, posted []domain.Transaction, start, end time.Time, rates currency.Rates) (int64, []CommittedItem, error) {
	var total int64
	var items []CommittedItem
	for _, r := range recurrings {
		if !r.Active() || !r.Amount.IsNegative() || !budget.TracksCategory(r.CategoryID) || !r.Smooths() || !smoothing.LandsIn(r, start, end) {
			continue
		}
		mag, err := money.Abs(r.Amount.Amount)
		if err != nil {
			return 0, nil, fmt.Errorf("smoothed recurring %q: %w", r.ID, err)
		}
		count := countPostedMatches(budget, posted, r, mag, start, end)
		if expected := len(smoothing.OccurrencesIn(r, start, end)); count > expected {
			count = expected
		}
		if count == 0 {
			continue
		}
		amount, err := money.Mul(mag, int64(count))
		if err != nil {
			return 0, nil, fmt.Errorf("scale smoothed recurring %q: %w", r.ID, err)
		}
		converted, err := rates.Convert(money.New(amount, r.Amount.Currency), budget.Limit.Currency)
		if err != nil {
			return 0, nil, fmt.Errorf("convert smoothed recurring %q: %w", r.ID, err)
		}
		total, err = money.Add(total, converted.Amount)
		if err != nil {
			return 0, nil, fmt.Errorf("sum smoothed recurring %q: %w", r.ID, err)
		}
		items = append(items, CommittedItem{RecurringID: r.ID, Label: r.Label, Amount: converted, Smoothed: true})
	}
	return total, items, nil
}

// ApplySmoothingOffsetChecked preserves the evaluated cap and returns an error
// if adjusting either spent or remaining would overflow.
func ApplySmoothingOffsetChecked(st Status, offset int64, nearThreshold float64) (Status, error) {
	if offset <= 0 {
		return st, nil
	}
	if offset > st.Spent.Amount {
		offset = st.Spent.Amount
	}
	if offset <= 0 {
		return st, nil
	}
	adjustment := money.New(offset, st.Spent.Currency)
	spent, err := st.Spent.Sub(adjustment)
	if err != nil {
		return Status{}, fmt.Errorf("offset smoothed spend: %w", err)
	}
	remaining, err := st.Remaining.Add(adjustment)
	if err != nil {
		return Status{}, fmt.Errorf("offset smoothed remainder: %w", err)
	}
	st.Spent, st.Remaining = spent, remaining
	st.Percent = percent(spent, st.Limit)
	st.State = classifyDirected(st.Budget.Direction, spent, st.Limit, nearThreshold)
	return st, nil
}

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
