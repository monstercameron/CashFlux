package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// maxEnvelopePeriods caps how far back the envelope accumulation looks, so a
// budget with very old (or bad) transaction dates can't loop unboundedly. 240
// covers 20 years of monthly periods.
const maxEnvelopePeriods = 240

// EnvelopeAvailable returns the envelope balance available for the period
// containing curRef under envelope budgeting (D6): every period from the first
// covered transaction through the current period is funded by the budget's
// limit, and all covered spend in that span draws it down. The result is the
// running balance carried forward — positive means money still in the envelope,
// negative means it's overdrawn. covers is the set of categories the budget
// counts (its own plus descendants, from categorytree.Descendants); an empty
// covers falls back to the budget's own category. Individual-scope budgets only
// count the owner's spend.
//
// With no covered spend yet, the envelope holds exactly one period's limit.
func EnvelopeAvailable(budget domain.Budget, all []domain.Transaction, curRef time.Time, weekStart time.Weekday, rates currency.Rates, covers map[string]bool) (money.Money, error) {
	limit := normalizedLimit(budget, rates)
	pred := func(id string) bool { return id == budget.CategoryID || covers[id] }

	// Earliest covered, in-scope expense — the period funding starts from here.
	var earliest time.Time
	found := false
	for _, t := range all {
		if !t.IsExpense() || !pred(t.CategoryID) {
			continue
		}
		if budget.Scope == domain.ScopeIndividual && t.MemberID != budget.OwnerID {
			continue
		}
		if !found || t.Date.Before(earliest) {
			earliest, found = t.Date, true
		}
	}
	if !found {
		return limit, nil // funded once, nothing spent
	}

	curStart, _ := PeriodRange(budget.Period, curRef, weekStart)
	available := money.Zero(limit.Currency)
	ref := earliest
	for i := 0; i < maxEnvelopePeriods; i++ {
		ps, pe := PeriodRange(budget.Period, ref, weekStart)
		spent, err := spentCovered(budget, all, ps, pe, rates, pred)
		if err != nil {
			return money.Money{}, err
		}
		leftover, err := limit.Sub(spent)
		if err != nil {
			return money.Money{}, err
		}
		if available, err = available.Add(leftover); err != nil {
			return money.Money{}, err
		}
		if !ps.Before(curStart) {
			break // reached the current period (or beyond) — stop after funding it
		}
		ref = pe // step to the next period
	}
	return available, nil
}
