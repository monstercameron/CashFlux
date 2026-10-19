// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/money"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// BudgetCopyDefaults are the monetary and directional fields copied into a new
// budget draft. Currency and cadence stay attached to the original limit.
type BudgetCopyDefaults struct {
	Limit     money.Money
	Period    domain.Period
	Direction domain.BudgetDirection
}

// AutoBudgetMatches returns one compatible household spending budget for each
// category. It includes every cadence and multi-category tracking, while excluding
// individual budgets so a household average never replaces one member's plan.
// The most specific matching budget wins; ties are resolved by ID.
func AutoBudgetMatches(budgets []domain.Budget) map[string]domain.Budget {
	byCategory := make(map[string]domain.Budget)
	for _, budget := range budgets {
		if budget.Direction.IsSaving() || budget.Scope == domain.ScopeIndividual || budget.OwnerID != "" && budget.OwnerID != domain.GroupOwnerID {
			continue
		}
		for _, categoryID := range budget.TrackedCategoryIDs() {
			current, exists := byCategory[categoryID]
			if !exists || autoBudgetMatchLess(budget, current, categoryID) {
				byCategory[categoryID] = budget
			}
		}
	}
	return byCategory
}

func autoBudgetMatchLess(candidate, current domain.Budget, categoryID string) bool {
	candidateExact := len(candidate.TrackedCategoryIDs()) == 1 && candidate.CategoryID == categoryID
	currentExact := len(current.TrackedCategoryIDs()) == 1 && current.CategoryID == categoryID
	if candidateExact != currentExact {
		return candidateExact
	}
	return candidate.ID < current.ID
}

// CopyBudgetDefaults returns the fields whose meaning must survive copying a
// budget into a new draft.
func CopyBudgetDefaults(budget domain.Budget) BudgetCopyDefaults {
	return BudgetCopyDefaults{Limit: budget.Limit, Period: budget.Period, Direction: budget.Direction}
}

// LimitForPeriod converts a limit to the target cadence and currency with
// checked period arithmetic and the selected FX table.
func LimitForPeriod(amount money.Money, from, to domain.Period, targetCurrency string, rates currency.Rates) (money.Money, error) {
	equivalent, err := PeriodEquivalent(amount, from, to)
	if err != nil {
		return money.Money{}, fmt.Errorf("convert budget limit cadence: %w", err)
	}
	converted, err := rates.Convert(equivalent, targetCurrency)
	if err != nil {
		return money.Money{}, fmt.Errorf("convert budget limit currency: %w", err)
	}
	return converted, nil
}

// SuggestMethod selects how a per-category monthly budget is learned from history.
type SuggestMethod string

const (
	// MethodRecent averages spend over the trailing window (SuggestLimit) — the quick
	// "budget my recent spending" take (the Smart, free tier).
	MethodRecent SuggestMethod = "recent"
	// MethodHealthy reviews a longer window and drops the single highest month before
	// averaging, so a one-off spike (a vacation, an annual bill) doesn't inflate the
	// target — a sustainable "healthy" average (the Smart+ tier).
	MethodHealthy SuggestMethod = "healthy"
)

// BudgetSuggestion is one category's proposed monthly budget, learned from spending
// history. MonthlyMinor is the suggested limit in base-currency minor units.
type BudgetSuggestion struct {
	CategoryID   string
	CategoryName string
	MonthlyMinor int64
}

// SuggestBudgets proposes a monthly budget for every EXPENSE category with learnable
// spend over the trailing `months` full months, using the chosen method. Categories
// with no spend to learn from are omitted. Results are sorted by suggested amount,
// largest first, so the biggest spending categories lead — the ones a budget matters
// most for.
//
// It is a pure function over the ledger: the UI layer decides which suggestions to
// pre-select and how the user tunes them before any budget is written.
func SuggestBudgets(categories []domain.Category, txns []domain.Transaction, now time.Time, months int, rates currency.Rates, method SuggestMethod) ([]BudgetSuggestion, error) {
	limit := SuggestLimit
	if method == MethodHealthy {
		limit = HealthyLimit
	}
	out := make([]BudgetSuggestion, 0, len(categories))
	for _, c := range categories {
		if c.Kind != domain.KindExpense {
			continue
		}
		minor, err := limit(c.ID, txns, now, months, rates)
		if err != nil {
			return nil, err
		}
		if minor <= 0 {
			continue
		}
		out = append(out, BudgetSuggestion{CategoryID: c.ID, CategoryName: c.Name, MonthlyMinor: minor})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].MonthlyMinor > out[j].MonthlyMinor })
	return out, nil
}

// FillAutoBudgetPercentages seeds suggestions that arrived after the form's
// first render while preserving every slider value the user already changed.
func FillAutoBudgetPercentages(current map[string]int, suggestions []BudgetSuggestion) map[string]int {
	out := make(map[string]int, len(current)+len(suggestions))
	for id, value := range current {
		out[id] = value
	}
	for _, suggestion := range suggestions {
		if _, exists := out[suggestion.CategoryID]; !exists {
			out[suggestion.CategoryID] = 100
		}
	}
	return out
}

// FillAutoBudgetSelection seeds newly available categories as selected only
// when they have no compatible household budget, preserving existing choices.
func FillAutoBudgetSelection(current map[string]bool, suggestions []BudgetSuggestion, existing map[string]domain.Budget) map[string]bool {
	out := make(map[string]bool, len(current)+len(suggestions))
	for id, value := range current {
		out[id] = value
	}
	for _, suggestion := range suggestions {
		if _, exists := out[suggestion.CategoryID]; !exists {
			_, hasBudget := existing[suggestion.CategoryID]
			out[suggestion.CategoryID] = !hasBudget
		}
	}
	return out
}

// SelectedBudgetCategories returns the selected category IDs that are valid
// for the budget direction. Saving budgets can track income categories;
// spending budgets may track only expenses. IDs follow the category input order.
func SelectedBudgetCategories(categories []domain.Category, selected map[string]bool, saving bool) []string {
	var out []string
	for _, category := range categories {
		if !selected[category.ID] || !saving && category.Kind != domain.KindExpense {
			continue
		}
		out = append(out, category.ID)
	}
	return out
}

// HealthyLimit proposes a monthly budget like SuggestLimit but drops the single highest
// month within the learned span before averaging, so one blowout month (a holiday, an
// annual renewal) doesn't inflate an everyday target. With a one-month span it returns
// that month's spend (nothing to trim). Same currency/expense/partial-month rules as
// SuggestLimit. It is a func value so SuggestBudgets can dispatch between the two.
//
// Like SuggestLimit it attributes SPLIT LINES to their own category (the split
// contract in domain/category_split.go). Reading only the parent's CategoryID
// made the two methods disagree on the same household: a family that always
// splits its Costco receipt across Groceries and Household got a Groceries
// suggestion under "recent" and none at all under "healthy", because this
// function saw no transaction booked to Groceries.
func HealthyLimit(categoryID string, txns []domain.Transaction, now time.Time, months int, rates currency.Rates) (int64, error) {
	if categoryID == "" || months <= 0 {
		return 0, nil
	}
	covers := map[string]bool{categoryID: true}
	txns = nettedForSpending(txns)
	curStart := dateutil.MonthStart(now)

	monthSums := make([]int64, months+1)
	for k := 1; k <= months; k++ {
		ms := dateutil.AddMonths(curStart, -k)
		me := dateutil.AddMonths(curStart, -(k - 1))
		for _, t := range txns {
			if !t.IsExpense() || !dateutil.InRange(t.Date, ms, me) {
				continue
			}
			amt, ok, err := scopedSpend(t, covers, rates)
			if err != nil {
				return 0, err
			}
			if ok {
				monthSums[k], err = money.Add(monthSums[k], amt)
				if err != nil {
					return 0, err
				}
			}
		}
	}

	// Span = oldest month with spend .. most recent (k=1). Mirrors SuggestLimit so a
	// brand-new category isn't diluted by empty leading months.
	oldest := 0
	for k := 1; k <= months; k++ {
		if monthSums[k] > 0 {
			oldest = k
		}
	}
	if oldest == 0 {
		return 0, nil
	}
	span := monthSums[1 : oldest+1]
	if len(span) == 1 {
		return span[0], nil
	}
	// Drop the single highest month, then average the rest — the "healthy" trim.
	var total, hi int64
	for _, v := range span {
		var err error
		total, err = money.Add(total, v)
		if err != nil {
			return 0, err
		}
		if v > hi {
			hi = v
		}
	}
	return (total - hi) / int64(len(span)-1), nil
}
