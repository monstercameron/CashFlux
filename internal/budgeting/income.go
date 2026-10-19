// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// IncomeForBudgetsChecked resolves actual or configured budget income and
// reports FX failures instead of treating an incomplete total as zero.
func IncomeForBudgetsChecked(configuredMinor int64, txns []domain.Transaction, start, end time.Time, base string, rates currency.Rates) (int64, error) {
	if configuredMinor > 0 {
		return configuredMinor, nil
	}

	var total int64
	for _, t := range txns {
		if !t.CountsInReports() || !t.IsIncome() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := currency.ConvertBetween(t.Amount.Amount, t.Amount.Currency, base, rates)
		if err != nil {
			return 0, fmt.Errorf("convert budget income transaction %q: %w", t.ID, err)
		}
		var addErr error
		total, addErr = money.Add(total, conv)
		if addErr != nil {
			return 0, fmt.Errorf("sum budget income: %w", addErr)
		}
	}
	return total, nil
}

// AveragedIncomeChecked resolves an averaged income basis and reports missing
// exchange rates and arithmetic overflow instead of treating incomplete income
// as a complete total.
func AveragedIncomeChecked(
	mode string,
	paycheckMinMinor, configuredMinor int64,
	sources IncomeSources,
	txns []domain.Transaction,
	monthStart time.Time,
	months int,
	base string,
	rates currency.Rates,
) (int64, error) {
	if months < 1 {
		months = 1
	}
	if mode == IncomeModeFixed {
		if configuredMinor > 0 {
			return configuredMinor, nil
		}
		return 0, nil
	}
	start := dateutil.AddMonths(monthStart, -months)
	total, err := ZeroBasedIncomeChecked(mode, paycheckMinMinor, configuredMinor, sources, txns, start, monthStart, base, rates)
	if err != nil {
		return 0, err
	}
	return total / int64(months), nil
}

// IncomeSources is the RESOLVED set of category ids a "by source" income basis
// counts: the categories the user ticked, plus every descendant of each.
//
// It is a type rather than a plain []string so the expansion cannot be skipped.
// It used to be one, and the raw ids were matched with an exact id test while
// every other tracked-category test in this package expanded descendants first
// (categorytree.DescendantsOfAll). The result was that ticking a parent income
// category — the obvious thing to do when your deposits are filed under
// "Salary > Employer A" — counted nothing at all, silently (C532).
type IncomeSources struct{ ids map[string]bool }

// NewIncomeSources resolves the chosen category ids against the category tree,
// including each chosen category's descendants. A nil or empty cats slice is
// allowed and yields the chosen ids alone, so callers that genuinely have no
// tree (tests, imports) still work.
func NewIncomeSources(cats []domain.Category, chosen []string) IncomeSources {
	if len(chosen) == 0 {
		return IncomeSources{}
	}
	ids := categorytree.DescendantsOfAll(cats, chosen)
	if len(ids) == 0 {
		// No tree to expand against — fall back to the chosen ids verbatim.
		ids = make(map[string]bool, len(chosen))
	}
	for _, id := range chosen {
		ids[id] = true
	}
	return IncomeSources{ids: ids}
}

// Empty reports whether no income source has been chosen yet.
func (s IncomeSources) Empty() bool { return len(s.ids) == 0 }

// Has reports whether income filed under categoryID counts toward the basis.
func (s IncomeSources) Has(categoryID string) bool { return s.ids[categoryID] }

// Zero-based income modes: how the "income to assign" figure is derived for a
// zero-based budget.
const (
	IncomeModeAll        = "all"        // every income deposit in the window (default)
	IncomeModePaychecks  = "paychecks"  // only deposits at/above the paycheck threshold (ignore side income)
	IncomeModeFixed      = "fixed"      // a configured monthly figure, regardless of actual deposits
	IncomeModeCategories = "categories" // only income in a chosen set of categories (pick sources by name)
)

// ZeroBasedIncome resolves the income a zero-based budget assigns against, per the
// household's chosen basis so a user can be strict (paychecks only) or loose (all
// income, some months run over):
//   - IncomeModeFixed: the configured monthly take-home (configuredMinor); 0 when unset.
//   - IncomeModePaychecks: actual income in [start,end) but only deposits whose
//     base-currency amount is >= paycheckMinMinor, so regular paychecks count and
//     small side-hustle deposits are ignored (no threshold set → same as all).
//   - IncomeModeCategories: actual income in [start,end) but only from the chosen
//     sources, so a household picks its income by name (e.g. count Salary, hold
//     aside Freelance). The set is already descendant-expanded (see IncomeSources),
//     so ticking a parent counts the deposits filed under its children. An empty
//     set means nothing is chosen yet, so the figure is 0 until a source is picked.
//   - IncomeModeAll (default / unknown): every income deposit in [start,end).
//
// Actual-income modes sum inline (IsIncome + ConvertBetween over the same window
// IncomeForBudgets uses) — budgeting must not import ledger. Unlike IncomeForBudgets,
// an FX failure on one deposit skips just that deposit rather than zeroing the whole
// figure, so a single missing rate can't blank the budget.
//
// Deposits the user excluded from reports never count, in any mode. In the
// by-source mode a SPLIT deposit is attributed per line, so a paycheck split
// into salary + bonus feeds only the sources actually chosen (C533); the other
// modes have no per-category question to answer and count the whole deposit.
func ZeroBasedIncome(
	mode string,
	paycheckMinMinor, configuredMinor int64,
	sources IncomeSources,
	txns []domain.Transaction,
	start, end time.Time,
	base string,
	rates currency.Rates,
) int64 {
	if mode == IncomeModeFixed {
		if configuredMinor > 0 {
			return configuredMinor
		}
		return 0
	}
	if mode == IncomeModeCategories && sources.Empty() {
		return 0 // no income source chosen yet
	}
	convert := func(amt money.Money) (int64, bool) {
		conv, err := currency.ConvertBetween(amt.Amount, amt.Currency, base, rates)
		if err != nil {
			return 0, false // skip an unconvertible deposit, don't blank the whole figure
		}
		return conv, true
	}
	var total int64
	for _, t := range txns {
		if !t.IsIncome() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		// C532/C533 companion fix: a deposit the user excluded from reports (a
		// reimbursement, a transfer-in they tagged as such) must not inflate the
		// income they budget against either. reports.IncomeByCategory already
		// skipped these, so the modal's per-source rows and this total used to
		// disagree by exactly those deposits.
		if !t.CountsInReports() {
			continue
		}
		// C533: in the by-source basis a split deposit is attributed per LINE, so
		// a paycheck split into salary + bonus feeds only the sources actually
		// chosen. The other modes have no per-category question to answer, so
		// they keep counting the whole deposit.
		if mode == IncomeModeCategories && t.HasSplits() {
			var allocated int64
			for _, s := range t.Splits {
				conv, ok := convert(s.Amount)
				if !ok {
					continue
				}
				allocated += conv
				if sources.Has(s.CategoryID) {
					total += conv
				}
			}
			// Whatever the lines did not cover stays on the transaction's own
			// category, so this total keeps agreeing with the per-source rows
			// reports.IncomeByCategory renders in the picker.
			if whole, ok := convert(t.Amount); ok && sources.Has(t.CategoryID) {
				if rest := whole - allocated; rest > 0 {
					total += rest
				}
			}
			continue
		}
		if mode == IncomeModeCategories && !sources.Has(t.CategoryID) {
			continue // this income source is held aside
		}
		conv, ok := convert(t.Amount)
		if !ok {
			continue
		}
		if mode == IncomeModePaychecks && paycheckMinMinor > 0 && conv < paycheckMinMinor {
			continue // below the paycheck threshold — treat as side income
		}
		total += conv
	}
	return total
}

// ZeroBasedIncomeChecked sums the selected income basis and fails if a relevant
// amount cannot be converted or if the total overflows.
func ZeroBasedIncomeChecked(
	mode string,
	paycheckMinMinor, configuredMinor int64,
	sources IncomeSources,
	txns []domain.Transaction,
	start, end time.Time,
	base string,
	rates currency.Rates,
) (int64, error) {
	if mode == IncomeModeFixed {
		if configuredMinor > 0 {
			return configuredMinor, nil
		}
		return 0, nil
	}
	if mode == IncomeModeCategories && sources.Empty() {
		return 0, nil
	}
	convert := func(amt money.Money) (int64, error) {
		converted, err := currency.ConvertBetween(amt.Amount, amt.Currency, base, rates)
		if err != nil {
			return 0, fmt.Errorf("convert income amount from %s to %s: %w", amt.Currency, base, err)
		}
		return converted, nil
	}
	add := func(total, amount int64) (int64, error) {
		value, err := money.Add(total, amount)
		if err != nil {
			return 0, fmt.Errorf("sum selected income: %w", err)
		}
		return value, nil
	}
	var total int64
	for _, txn := range txns {
		if !txn.IsIncome() || !txn.CountsInReports() || !dateutil.InRange(txn.Date, start, end) {
			continue
		}
		if mode == IncomeModeCategories && txn.HasSplits() {
			var allocated int64
			for _, split := range txn.Splits {
				converted, err := convert(split.Amount)
				if err != nil {
					return 0, fmt.Errorf("transaction %q split: %w", txn.ID, err)
				}
				allocated, err = add(allocated, converted)
				if err != nil {
					return 0, fmt.Errorf("transaction %q: %w", txn.ID, err)
				}
				if sources.Has(split.CategoryID) {
					total, err = add(total, converted)
					if err != nil {
						return 0, fmt.Errorf("transaction %q: %w", txn.ID, err)
					}
				}
			}
			whole, err := convert(txn.Amount)
			if err != nil {
				return 0, fmt.Errorf("transaction %q: %w", txn.ID, err)
			}
			remainder, err := money.Sub(whole, allocated)
			if err != nil {
				return 0, fmt.Errorf("subtract allocated income for transaction %q: %w", txn.ID, err)
			}
			if rest := remainder; rest > 0 && sources.Has(txn.CategoryID) {
				total, err = add(total, rest)
				if err != nil {
					return 0, fmt.Errorf("transaction %q: %w", txn.ID, err)
				}
			}
			continue
		}
		if mode == IncomeModeCategories && !sources.Has(txn.CategoryID) {
			continue
		}
		converted, err := convert(txn.Amount)
		if err != nil {
			return 0, fmt.Errorf("transaction %q: %w", txn.ID, err)
		}
		if mode == IncomeModePaychecks && paycheckMinMinor > 0 && converted < paycheckMinMinor {
			continue
		}
		total, err = add(total, converted)
		if err != nil {
			return 0, fmt.Errorf("transaction %q: %w", txn.ID, err)
		}
	}
	return total, nil
}
