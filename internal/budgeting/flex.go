// SPDX-License-Identifier: MIT

package budgeting

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smoothing"
)

// DefaultCategoryClass derives the seed classification for a category from the
// recurring cash flows that map to it (BG2's default heuristic). It is used to
// pre-fill the one-time assignment sheet; the stored domain.Category.CategoryClass
// (read via Category.ClassOf) is the source of truth once the user has chosen.
//
//   - A category with a SMOOTHED recurring (annual/quarterly, XC3) seeds as
//     non-monthly — the cost is irregular and set aside over time.
//   - A category with any other recurring seeds as fixed — an expected commitment.
//   - A category with no recurring mapping seeds as flex (day-to-day discretionary).
//
// Income categories always seed as flex (they are not spending to manage).
func DefaultCategoryClass(cat domain.Category, recurrings []domain.Recurring) domain.CategoryClass {
	if cat.Kind == domain.KindIncome {
		return domain.ClassFlex
	}
	fixed := false
	for _, r := range recurrings {
		if r.CategoryID != cat.ID {
			continue
		}
		if r.Smooths() {
			return domain.ClassNonMonthly
		}
		fixed = true
	}
	if fixed {
		return domain.ClassFixed
	}
	return domain.ClassFlex
}

// FixedRow is one fixed-commitment category's expected-vs-actual checkoff for a
// flex-budgeting period. It composes recurring occurrences (the expected charge)
// with posted spending (the actual), so the row can render "paid / unpaid".
type FixedRow struct {
	// CategoryID and CategoryName identify the fixed category.
	CategoryID   string
	CategoryName string
	// Expected is the total of the recurring charges due this period (positive).
	Expected money.Money
	// Actual is the total posted expense in this category this period (positive).
	Actual money.Money
	// Paid reports whether the expected charge appears settled — actual spending
	// reaches the expected amount within tolerance (or there was nothing expected
	// but money was spent). Drives the checkoff tick.
	Paid bool
}

// NonMonthlyRow is one irregular-cost category's smoothed set-aside for a flex-
// budgeting period. It shows the XC3 monthly accrual the household should reserve
// alongside what has actually been spent so far.
type NonMonthlyRow struct {
	// CategoryID and CategoryName identify the non-monthly category.
	CategoryID   string
	CategoryName string
	// Accrual is the recommended monthly set-aside for this category (positive),
	// summed over every smoothed recurring that maps to it (XC3).
	Accrual money.Money
	// Spent is the total posted expense in this category this period (positive).
	Spent money.Money
}

// FlexView is the fully-evaluated flex-budgeting read model for one period: the
// single pooled flex number the user manages, plus the fixed-commitment checkoffs
// and non-monthly set-asides that sit alongside it. It is pure data — the UI
// renders it directly. Amounts are minor units in the base currency.
type FlexView struct {
	// Target is the household's single flex budget number for the period.
	Target money.Money
	// Spent is the total posted expense across all flex categories this period.
	Spent money.Money
	// Remaining is Target minus Spent (may be negative when overspent).
	Remaining money.Money
	// Over reports whether flex spending exceeded the flex target.
	Over bool
	// Fixed lists each fixed-commitment category's checkoff, name-sorted.
	Fixed []FixedRow
	// NonMonthly lists each irregular-cost category's set-aside, name-sorted.
	NonMonthly []NonMonthlyRow
}

// EvaluateFlex builds the FlexView for [start, end) from the classified categories,
// the period's transactions, and the recurring schedule. Each expense category is
// bucketed by its stored class (domain.Category.ClassOf): flex spending pools into
// one number, fixed categories become expected-vs-actual checkoffs composed with
// recurring occurrences, and non-monthly categories show their smoothed accrual.
// flexTarget is the single flex number (minor units) in the base currency.
func EvaluateFlex(cats []domain.Category, txns []domain.Transaction, recurrings []domain.Recurring, flexTarget int64, base string, start, end time.Time) FlexView {
	// Index each expense category by its effective class.
	classByCat := map[string]domain.CategoryClass{}
	nameByCat := map[string]string{}
	for _, c := range cats {
		if c.Kind == domain.KindIncome {
			continue
		}
		classByCat[c.ID] = c.ClassOf()
		nameByCat[c.ID] = c.Name
	}

	// Posted expense per category over the window (splits counted per line).
	spentByCat := map[string]int64{}
	for _, t := range txns {
		if !t.IsExpense() {
			continue
		}
		if t.Date.Before(start) || !t.Date.Before(end) {
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				spentByCat[s.CategoryID] += abs64(s.Amount.Amount)
			}
			continue
		}
		spentByCat[t.CategoryID] += abs64(t.Amount.Amount)
	}

	// Flex spent = Σ posted expense in flex categories.
	var flexSpent int64
	for catID, spent := range spentByCat {
		if classByCat[catID] == domain.ClassFlex {
			flexSpent += spent
		}
	}

	// Expected recurring charges per fixed category, and accruals per non-monthly.
	expectedByCat := map[string]int64{}
	accrualByCat := map[string]int64{}
	for _, r := range recurrings {
		if r.CategoryID == "" {
			continue
		}
		class, ok := classByCat[r.CategoryID]
		if !ok {
			continue
		}
		mag := abs64(r.Amount.Amount)
		if mag == 0 {
			continue
		}
		switch class {
		case domain.ClassFixed:
			expectedByCat[r.CategoryID] += mag * int64(len(smoothing.OccurrencesIn(r, start, end)))
		case domain.ClassNonMonthly:
			if r.Smooths() {
				accrualByCat[r.CategoryID] += smoothing.MonthlyAccrual(r)
			}
		}
	}

	view := FlexView{
		Target:    money.New(flexTarget, base),
		Spent:     money.New(flexSpent, base),
		Remaining: money.New(flexTarget-flexSpent, base),
		Over:      flexSpent > flexTarget,
	}

	// Build fixed + non-monthly rows for every classified category (even ones with
	// no recurring/spending yet, so the checklist is complete and stable).
	for catID, class := range classByCat {
		switch class {
		case domain.ClassFixed:
			exp := expectedByCat[catID]
			act := spentByCat[catID]
			view.Fixed = append(view.Fixed, FixedRow{
				CategoryID:   catID,
				CategoryName: nameByCat[catID],
				Expected:     money.New(exp, base),
				Actual:       money.New(act, base),
				Paid:         fixedPaid(exp, act),
			})
		case domain.ClassNonMonthly:
			view.NonMonthly = append(view.NonMonthly, NonMonthlyRow{
				CategoryID:   catID,
				CategoryName: nameByCat[catID],
				Accrual:      money.New(accrualByCat[catID], base),
				Spent:        money.New(spentByCat[catID], base),
			})
		}
	}

	sort.Slice(view.Fixed, func(i, j int) bool { return view.Fixed[i].CategoryName < view.Fixed[j].CategoryName })
	sort.Slice(view.NonMonthly, func(i, j int) bool { return view.NonMonthly[i].CategoryName < view.NonMonthly[j].CategoryName })
	return view
}

// fixedPaid reports whether a fixed commitment reads as settled: actual spending
// reaches the expected charge within tolerance, or (when nothing was expected)
// any money was spent in the category.
func fixedPaid(expected, actual int64) bool {
	if expected <= 0 {
		return actual > 0
	}
	return actual+amountTolerance(expected) >= expected
}
