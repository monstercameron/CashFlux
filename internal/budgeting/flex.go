// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
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
// the period's transactions, and the recurring schedule. Each expense category keeps
// its effective stored class: flex spending pools into
// one number, fixed categories become expected-vs-actual checkoffs composed with
// recurring occurrences, and non-monthly categories show their smoothed accrual.
// flexTarget is the single flex number (minor units) in the base currency.
func EvaluateFlex(cats []domain.Category, ledger NettedLedger, recurrings []domain.Recurring, flexTarget int64, rates currency.Rates, start, end time.Time) (FlexView, error) {
	if rates.Base == "" {
		return FlexView{}, fmt.Errorf("evaluate flex: base currency is empty")
	}
	categories := indexFlexCategories(cats)
	spent, err := flexPoolSpending(categories, ledger, rates, start, end)
	if err != nil {
		return FlexView{}, err
	}
	spentByCat, err := flexCategorySpending(categories, ledger, rates, start, end)
	if err != nil {
		return FlexView{}, err
	}
	recurring, err := flexRecurringAmounts(categories, recurrings, rates, start, end)
	if err != nil {
		return FlexView{}, err
	}
	return assembleFlexView(categories, spent, spentByCat, recurring, flexTarget)
}

type flexCategoryIndex struct {
	classByCat map[string]domain.CategoryClass
	nameByCat  map[string]string
	ownerByCat map[string]flexCategoryOwner
	flexRoots  []string
}

func indexFlexCategories(cats []domain.Category) flexCategoryIndex {
	// Index each expense category by its effective class.
	classByCat := map[string]domain.CategoryClass{}
	nameByCat := map[string]string{}
	ownerByCat := flexCategoryOwners(cats)
	var flexRoots []string
	for _, c := range cats {
		if c.Kind == domain.KindIncome {
			continue
		}
		owner := ownerByCat[c.ID]
		classByCat[c.ID] = owner.class
		nameByCat[c.ID] = c.Name
		if owner.class == domain.ClassFlex && owner.id == c.ID {
			flexRoots = append(flexRoots, c.ID)
		}
	}

	return flexCategoryIndex{classByCat: classByCat, nameByCat: nameByCat, ownerByCat: ownerByCat, flexRoots: flexRoots}
}

// flexPoolSpending applies the common contribution rules to the entire flex pool.
func flexPoolSpending(categories flexCategoryIndex, ledger NettedLedger, rates currency.Rates, start, end time.Time) (money.Money, error) {
	base := rates.Base
	// Net refunds once, then ask the common contribution predicate to select flex
	// lines. This preserves report exclusions, signed split netting, and category
	// descendants before converting each contribution into the display currency.
	flexBudget := domain.Budget{CategoryIDs: categories.flexRoots}
	flexCovers := make(map[string]bool, len(categories.ownerByCat))
	for id, owner := range categories.ownerByCat {
		if owner.class == domain.ClassFlex {
			flexCovers[id] = true
		}
	}
	flexSpent := money.Zero(base)
	for _, t := range ledger {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		contribution, ok, err := ContributionChecked(flexBudget, t, func(id string) bool { return flexCovers[id] })
		if err != nil {
			return money.Money{}, fmt.Errorf("evaluate flex transaction %q: %w", t.ID, err)
		}
		if !ok {
			continue
		}
		converted, err := rates.Convert(contribution, base)
		if err != nil {
			return money.Money{}, fmt.Errorf("convert flex transaction %q: %w", t.ID, err)
		}
		flexSpent, err = flexSpent.Add(converted)
		if err != nil {
			return money.Money{}, fmt.Errorf("sum flex spending: %w", err)
		}
	}

	return flexSpent, nil
}

// flexCategorySpending keeps the checkoff ledgers separate from the pooled total.
func flexCategorySpending(categories flexCategoryIndex, ledger NettedLedger, rates currency.Rates, start, end time.Time) (map[string]int64, error) {
	base := rates.Base
	// Posted expense per category over the window for the checkoff ledgers.
	spentByCat := map[string]int64{}
	for catID := range categories.classByCat {
		if categories.ownerByCat[catID].id != catID {
			continue
		}
		budget := domain.Budget{CategoryID: catID}
		covers := make(map[string]bool)
		for id, owner := range categories.ownerByCat {
			if owner.id == catID {
				covers[id] = true
			}
		}
		for _, t := range ledger {
			if !dateutil.InRange(t.Date, start, end) {
				continue
			}
			contribution, ok, err := ContributionChecked(budget, t, func(id string) bool { return covers[id] })
			if err != nil {
				return nil, fmt.Errorf("evaluate flex category %q transaction %q: %w", catID, t.ID, err)
			}
			if !ok {
				continue
			}
			converted, err := rates.Convert(contribution, base)
			if err != nil {
				return nil, fmt.Errorf("convert flex category %q transaction %q: %w", catID, t.ID, err)
			}
			spentByCat[catID], err = money.Add(spentByCat[catID], converted.Amount)
			if err != nil {
				return nil, fmt.Errorf("sum flex category %q spending: %w", catID, err)
			}
		}
	}

	return spentByCat, nil
}

type flexRecurringTotals struct {
	expectedByCat map[string]int64
	accrualByCat  map[string]int64
}

// flexRecurringAmounts reserves fixed occurrences and smoothed monthly accruals.
func flexRecurringAmounts(categories flexCategoryIndex, recurrings []domain.Recurring, rates currency.Rates, start, end time.Time) (flexRecurringTotals, error) {
	base := rates.Base
	// Expected recurring charges per fixed category, and accruals per non-monthly.
	expectedByCat := map[string]int64{}
	accrualByCat := map[string]int64{}
	for _, r := range recurrings {
		if r.CategoryID == "" {
			continue
		}
		if _, ok := categories.classByCat[r.CategoryID]; !ok {
			continue
		}
		owner := categories.ownerByCat[r.CategoryID]
		class := owner.class
		magnitude, err := r.Amount.AbsChecked()
		if err != nil {
			return flexRecurringTotals{}, fmt.Errorf("recurring %q amount: %w", r.ID, err)
		}
		mag, err := rates.Convert(magnitude, base)
		if err != nil {
			return flexRecurringTotals{}, fmt.Errorf("convert recurring %q: %w", r.ID, err)
		}
		if mag.Amount == 0 {
			continue
		}
		switch class {
		case domain.ClassFixed:
			amount, err := money.Mul(mag.Amount, int64(len(smoothing.OccurrencesIn(r, start, end))))
			if err != nil {
				return flexRecurringTotals{}, fmt.Errorf("calculate recurring %q occurrences: %w", r.ID, err)
			}
			expectedByCat[owner.id], err = money.Add(expectedByCat[owner.id], amount)
			if err != nil {
				return flexRecurringTotals{}, fmt.Errorf("sum expected recurring for %q: %w", owner.id, err)
			}
		case domain.ClassNonMonthly:
			if r.Smooths() {
				accrual := money.New(smoothing.MonthlyAccrual(r), r.Amount.Currency)
				accrual, err = rates.Convert(accrual, base)
				if err != nil {
					return flexRecurringTotals{}, fmt.Errorf("convert recurring accrual %q: %w", r.ID, err)
				}
				accrualByCat[owner.id], err = money.Add(accrualByCat[owner.id], accrual.Amount)
				if err != nil {
					return flexRecurringTotals{}, fmt.Errorf("sum recurring accrual for %q: %w", owner.id, err)
				}
			}
		}
	}

	return flexRecurringTotals{expectedByCat: expectedByCat, accrualByCat: accrualByCat}, nil
}

// assembleFlexView derives the remaining pool and name-sorted checkoff rows.
func assembleFlexView(categories flexCategoryIndex, flexSpent money.Money, spentByCat map[string]int64, recurring flexRecurringTotals, flexTarget int64) (FlexView, error) {
	base := flexSpent.Currency
	view := FlexView{
		Target:    money.New(flexTarget, base),
		Spent:     flexSpent,
		Remaining: money.New(0, base),
	}
	remaining, err := money.Sub(flexTarget, flexSpent.Amount)
	if err != nil {
		return FlexView{}, fmt.Errorf("calculate flex remaining: %w", err)
	}
	view.Remaining = money.New(remaining, base)
	view.Over = flexSpent.Amount > flexTarget

	// Build fixed + non-monthly rows for each class owner, even with no recurring
	// or spending yet, so the checklist is complete and stable without duplicates.
	for catID, class := range categories.classByCat {
		if categories.ownerByCat[catID].id != catID {
			continue
		}
		switch class {
		case domain.ClassFixed:
			exp := recurring.expectedByCat[catID]
			act := spentByCat[catID]
			view.Fixed = append(view.Fixed, FixedRow{
				CategoryID:   catID,
				CategoryName: categories.nameByCat[catID],
				Expected:     money.New(exp, base),
				Actual:       money.New(act, base),
				Paid:         fixedPaid(exp, act),
			})
		case domain.ClassNonMonthly:
			view.NonMonthly = append(view.NonMonthly, NonMonthlyRow{
				CategoryID:   catID,
				CategoryName: categories.nameByCat[catID],
				Accrual:      money.New(recurring.accrualByCat[catID], base),
				Spent:        money.New(spentByCat[catID], base),
			})
		}
	}

	sort.Slice(view.Fixed, func(i, j int) bool { return view.Fixed[i].CategoryName < view.Fixed[j].CategoryName })
	sort.Slice(view.NonMonthly, func(i, j int) bool { return view.NonMonthly[i].CategoryName < view.NonMonthly[j].CategoryName })
	return view, nil
}

type flexCategoryOwner struct {
	id    string
	class domain.CategoryClass
}

// flexCategoryOwners records each expense category's effective stored class.
// ClassOf's legacy default keeps categories without a stored class in the flex pool.
func flexCategoryOwners(cats []domain.Category) map[string]flexCategoryOwner {
	owners := make(map[string]flexCategoryOwner, len(cats))
	for _, c := range cats {
		if c.Kind != domain.KindExpense {
			continue
		}
		owners[c.ID] = flexCategoryOwner{id: c.ID, class: c.ClassOf()}
	}
	return owners
}

// fixedPaid reports whether a fixed commitment reads as settled: actual spending
// reaches the expected charge within tolerance, or (when nothing was expected)
// any money was spent in the category.
func fixedPaid(expected, actual int64) bool {
	if expected <= 0 {
		return actual > 0
	}
	return actual >= expected-amountTolerance(expected)
}
