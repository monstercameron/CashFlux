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

// CommittedItem is one recurring's contribution to a budget's committed total,
// kept for explainability (the "no black boxes" rule) — the row can name each
// commitment, e.g. "Netflix $16" or "set-aside for Insurance $50".
type CommittedItem struct {
	// RecurringID is the recurring cash flow this commitment comes from.
	RecurringID string
	// Label is the recurring's plain-English label.
	Label string
	// Amount is the still-to-come amount this period (positive magnitude).
	Amount money.Money
	// Smoothed is true when this is an off-period sinking-fund set-aside (XC3)
	// rather than a directly-expected recurring charge (XC4).
	Smoothed bool
}

// CommittedWithRates is the FX-aware committed calculation. Recurring amounts
// and smoothing accruals are converted into the budget's remaining currency;
// missing rates and arithmetic overflow are returned as errors.
func CommittedWithRates(budget domain.Budget, recurrings []domain.Recurring, posted []domain.Transaction, remaining money.Money, start, end time.Time, rates currency.Rates) (CommittedResult, error) {
	cur := remaining.Currency
	var committedRaw int64
	var items []CommittedItem
	convert := func(amount int64, from, id string) (int64, error) {
		mag, err := money.Abs(amount)
		if err != nil {
			return 0, fmt.Errorf("recurring %q amount: %w", id, err)
		}
		converted, err := rates.Convert(money.New(mag, from), cur)
		if err != nil {
			return 0, fmt.Errorf("convert recurring %q to %s: %w", id, cur, err)
		}
		return converted.Amount, nil
	}
	for _, r := range recurrings {
		if r.CategoryID == "" || !budget.TracksCategory(r.CategoryID) {
			continue
		}
		mag, err := money.Abs(r.Amount.Amount)
		if err != nil {
			return CommittedResult{}, fmt.Errorf("recurring %q magnitude: %w", r.ID, err)
		}
		if mag == 0 {
			continue
		}
		if r.Smooths() {
			if smoothing.LandsIn(r, start, end) {
				continue
			}
			accrual := smoothing.MonthlyAccrual(r)
			if accrual <= 0 {
				continue
			}
			converted, err := convert(accrual, r.Amount.Currency, r.ID)
			if err != nil {
				return CommittedResult{}, err
			}
			committedRaw, err = money.Add(committedRaw, converted)
			if err != nil {
				return CommittedResult{}, fmt.Errorf("sum committed recurring amounts: %w", err)
			}
			items = append(items, CommittedItem{RecurringID: r.ID, Label: r.Label, Amount: money.New(converted, cur), Smoothed: true})
			continue
		}
		expected := len(smoothing.OccurrencesIn(r, start, end))
		if expected == 0 {
			continue
		}
		postedCount := countPostedMatches(budget, posted, r, mag, start, end)
		notPosted := expected - postedCount
		if notPosted <= 0 {
			continue
		}
		converted, err := convert(mag, r.Amount.Currency, r.ID)
		if err != nil {
			return CommittedResult{}, err
		}
		amount, err := money.Mul(converted, int64(notPosted))
		if err != nil {
			return CommittedResult{}, fmt.Errorf("scale recurring %q commitment: %w", r.ID, err)
		}
		committedRaw, err = money.Add(committedRaw, amount)
		if err != nil {
			return CommittedResult{}, fmt.Errorf("sum committed recurring amounts: %w", err)
		}
		items = append(items, CommittedItem{RecurringID: r.ID, Label: r.Label, Amount: money.New(amount, cur)})
	}
	rem := remaining.Amount
	if rem <= 0 {
		return CommittedResult{Committed: money.Zero(cur), Free: remaining, Items: items}, nil
	}
	committed := committedRaw
	if committed > rem {
		committed = rem
	}
	return CommittedResult{Committed: money.New(committed, cur), Free: money.New(rem-committed, cur), Items: items}, nil
}

// CommittedResult splits a budget's remaining money into the part already
// spoken-for by recurring commitments this period (Committed) and the part
// genuinely free to spend (Free). Committed + Free always equals Remaining, so
// the meter's committed segment and the "free" figure reconcile exactly.
type CommittedResult struct {
	// Committed is remaining money already claimed by recurring items expected
	// this period but not yet posted, plus smoothed off-period set-asides.
	Committed money.Money
	// Free is Remaining minus Committed (never below zero unless Remaining is
	// itself negative, i.e. the budget is already over).
	Free money.Money
	// Items enumerates each commitment for the explainer caption.
	Items []CommittedItem
}

// amountTolerance is the minor-unit slack allowed when matching a posted
// transaction to an expected recurring charge (v1: exact category + amount within
// 1% of the recurring's magnitude, minimum 1 unit).
func amountTolerance(magnitude int64) int64 {
	tol := magnitude / 100
	if tol < 1 {
		tol = 1
	}
	return tol
}

// abs64 returns the absolute value of a signed minor-unit amount.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// Committed derives the committed-vs-free split for a budget over its own period
// [start, end). It maps each recurring whose category the budget tracks to the
// amount still expected this period:
//
//   - A non-smoothed recurring contributes its expected occurrences in the period
//     minus those already posted (matched by tracked category + amount within
//     tolerance), each at the recurring's magnitude.
//   - A smoothed annual/quarterly recurring (XC3) contributes its virtual monthly
//     set-aside as committed in OFF periods; in its landing period it contributes
//     nothing (the accrued fund offsets the large posted charge elsewhere).
//
// remaining is the budget's remaining money for the period (limit minus spent).
// Committed is capped so it never exceeds a positive remaining; Free is the
// balance. When remaining is zero or negative (budget met or over), Committed is
// zero and Free carries the remaining as-is.
func Committed(budget domain.Budget, recurrings []domain.Recurring, posted []domain.Transaction, remaining money.Money, start, end time.Time) CommittedResult {
	cur := remaining.Currency

	var committedRaw int64
	var items []CommittedItem
	for _, r := range recurrings {
		if r.CategoryID == "" || !budget.TracksCategory(r.CategoryID) {
			continue
		}
		mag := abs64(r.Amount.Amount)
		if mag == 0 {
			continue
		}

		if r.Smooths() {
			if smoothing.LandsIn(r, start, end) {
				continue // landing period — fund covers it, no committed set-aside
			}
			accrual := smoothing.MonthlyAccrual(r)
			if accrual <= 0 {
				continue
			}
			committedRaw += accrual
			items = append(items, CommittedItem{
				RecurringID: r.ID, Label: r.Label,
				Amount: money.New(accrual, cur), Smoothed: true,
			})
			continue
		}

		expected := len(smoothing.OccurrencesIn(r, start, end))
		if expected == 0 {
			continue
		}
		postedCount := countPostedMatches(budget, posted, r, mag, start, end)
		notPosted := expected - postedCount
		if notPosted <= 0 {
			continue
		}
		amt := mag * int64(notPosted)
		committedRaw += amt
		items = append(items, CommittedItem{
			RecurringID: r.ID, Label: r.Label,
			Amount: money.New(amt, cur), Smoothed: false,
		})
	}

	rem := remaining.Amount
	if rem <= 0 {
		// Budget met or over: nothing is "free to commit"; carry remaining as free.
		return CommittedResult{
			Committed: money.Zero(cur),
			Free:      remaining,
			Items:     items,
		}
	}
	committed := committedRaw
	if committed > rem {
		committed = rem
	}
	return CommittedResult{
		Committed: money.New(committed, cur),
		Free:      money.New(rem-committed, cur),
		Items:     items,
	}
}

// countPostedMatches counts posted expense transactions in [start, end) that plausibly
// settle recurring r: a tracked-category expense whose magnitude is within tolerance of
// the recurring's amount. Split lines are matched per-line against the tracked category.
func countPostedMatches(budget domain.Budget, posted []domain.Transaction, r domain.Recurring, mag int64, start, end time.Time) int {
	tol := amountTolerance(mag)
	n := 0
	for _, t := range posted {
		if !t.IsExpense() {
			continue
		}
		if t.Date.Before(start) || !t.Date.Before(end) {
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				if budget.TracksCategory(s.CategoryID) && abs64(abs64(s.Amount.Amount)-mag) <= tol {
					n++
				}
			}
			continue
		}
		if budget.TracksCategory(t.CategoryID) && abs64(abs64(t.Amount.Amount)-mag) <= tol {
			n++
		}
	}
	return n
}
