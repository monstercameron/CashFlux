// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pricecreep"
)

// priceCreepRecurringKey is the Custom-field key under which a price-creep task
// records the recurring it watches, so deleting that recurring auto-resolves it.
const priceCreepRecurringKey = "xc5_recurringId"

// PriceCreepBudgetImpact previews what accepting a new recurring price does to the
// affected budget (XC5): the before/after usage the accept-flow modal renders as
// two short lines, plus the limit bump that keeps the budget whole. newMinor is
// the new per-cadence amount as a positive magnitude in the recurring's currency.
func (a *App) PriceCreepBudgetImpact(recurringID string, newMinor int64) pricecreep.BudgetImpact {
	impact, err := a.PriceCreepBudgetImpactChecked(recurringID, newMinor)
	if err != nil {
		a.log.Error("price creep budget impact", "recurring", recurringID, "err", err)
		return pricecreep.BudgetImpact{}
	}
	return impact
}

// PriceCreepBudgetImpactChecked previews the price change using the same
// effective budget view as the Budgets screen. Currency conversion failures are
// returned so the caller can explain why a preview is unavailable.
func (a *App) PriceCreepBudgetImpactChecked(recurringID string, newMinor int64) (pricecreep.BudgetImpact, error) {
	r, ok := a.findRecurring(recurringID)
	if !ok {
		return pricecreep.BudgetImpact{}, fmt.Errorf("recurring %q not found", recurringID)
	}
	base := a.baseCurrency()
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}

	oldMonthly, err := recurringMonthlyEquivalentChecked(r)
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("calculate current monthly price: %w", err)
	}
	newAmount, err := money.New(newMinor, r.Amount.Currency).AbsChecked()
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("normalize new recurring price: %w", err)
	}
	newRecurring := r
	newRecurring.Amount = newAmount
	newMonthly, err := recurringMonthlyEquivalentChecked(newRecurring)
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("calculate new monthly price: %w", err)
	}
	delta, err := newMonthly.Sub(oldMonthly)
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("calculate monthly price change: %w", err)
	}
	delta, err = rates.Convert(delta, base)
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("convert monthly price change: %w", err)
	}

	b, found := a.budgetForRecurring(r)
	if !found {
		return pricecreep.Preview("", base, 0, 0, delta.Amount, false), nil
	}
	now := a.clock()
	in, err := a.BudgetViewInput(now)
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("build budget view input: %w", err)
	}
	v := budgeting.EvaluateViewBudget(b, in)
	if !v.Valid {
		if v.Error != nil {
			return pricecreep.BudgetImpact{}, fmt.Errorf("evaluate budget %q: %w", b.ID, v.Error)
		}
		return pricecreep.BudgetImpact{}, fmt.Errorf("evaluate budget %q: unavailable", b.ID)
	}
	limit, err := rates.Convert(v.Status.Limit, base)
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("convert effective budget limit: %w", err)
	}
	spent, err := rates.Convert(v.Status.Spent, base)
	if err != nil {
		return pricecreep.BudgetImpact{}, fmt.Errorf("convert budget spending: %w", err)
	}
	return pricecreep.Preview(b.Name, base, limit.Amount, spent.Amount, delta.Amount, true), nil
}

func recurringMonthlyEquivalentChecked(r domain.Recurring) (money.Money, error) {
	a, err := r.Amount.AbsChecked()
	if err != nil {
		return money.Money{}, err
	}
	switch r.Cadence {
	case domain.CadenceWeekly:
		return a.MulDiv(52, 12, money.RoundTowardZero)
	case domain.CadenceBiweekly:
		return a.MulDiv(26, 12, money.RoundTowardZero)
	case domain.CadenceSemimonthly:
		return a.Mul(2)
	case domain.CadenceQuarterly:
		return a.MulDiv(1, 3, money.RoundTowardZero)
	case domain.CadenceYearly:
		return a.MulDiv(1, 12, money.RoundTowardZero)
	default:
		return a, nil
	}
}

// AcceptNewPrice updates a recurring's expected amount to the accepted new price
// (XC5). newMinor is a positive magnitude; the stored amount keeps the recurring's
// currency and its expense (negative) sign.
func (a *App) AcceptNewPrice(recurringID string, newMinor int64) error {
	r, ok := a.findRecurring(recurringID)
	if !ok {
		return fmt.Errorf("appstate: recurring %q not found", recurringID)
	}
	r.Amount = money.New(-abs64(newMinor), r.Amount.Currency)
	return a.PutRecurring(r)
}

// RaiseBudgetForCreep raises the budget covering the recurring's category by the
// price delta converted to that budget's currency and cadence (the "also
// raise the budget?" option in the accept flow). No-op when there is no budget.
func (a *App) RaiseBudgetForCreep(recurringID string, newMinor int64) error {
	r, ok := a.findRecurring(recurringID)
	if !ok {
		return fmt.Errorf("appstate: recurring %q not found", recurringID)
	}
	// Raising the limit is a write path with an established category-budget
	// contract (BC4S1). Keep its selector independent from the stricter,
	// account-scoped selector used by the impact preview.
	b, found := a.budgetForCategory(r.CategoryID)
	if !found {
		return nil
	}
	newR := r
	newAmount, err := money.New(newMinor, r.Amount.Currency).AbsChecked()
	if err != nil {
		return fmt.Errorf("normalize new recurring price: %w", err)
	}
	newR.Amount = newAmount
	newMonthly, err := recurringMonthlyEquivalentChecked(newR)
	if err != nil {
		return fmt.Errorf("calculate new monthly price: %w", err)
	}
	oldMonthly, err := recurringMonthlyEquivalentChecked(r)
	if err != nil {
		return fmt.Errorf("calculate current monthly price: %w", err)
	}
	delta, err := newMonthly.Sub(oldMonthly)
	if err != nil {
		return fmt.Errorf("price increase delta: %w", err)
	}
	b = b.WithBaseCurrency(a.baseCurrency())
	rates := currency.Rates{Base: a.baseCurrency(), Rates: a.Settings().FXRates}
	delta, err = rates.Convert(delta, b.Limit.Currency)
	if err != nil {
		return fmt.Errorf("convert price increase: %w", err)
	}
	delta, err = budgeting.PeriodEquivalent(delta, domain.PeriodMonthly, b.Period)
	if err != nil {
		return fmt.Errorf("price increase cadence: %w", err)
	}
	b.Limit, err = b.Limit.Add(delta)
	if err != nil {
		return fmt.Errorf("raise budget for price increase: %w", err)
	}
	return a.PutBudget(b)
}

// CreatePriceCreepTask creates a self-resolving "cancel or downgrade" task for a
// crept recurring (XC5 → XC8). It auto-resolves when a later cycle posts at the
// old price again (a charge matching the old amount) or when the recurring is
// deleted (see resolvePriceCreepTasksForDeletedRecurring). oldMinor is the prior
// expected amount as a positive magnitude in the recurring's currency.
func (a *App) CreatePriceCreepTask(recurringID string, oldMinor int64) error {
	r, ok := a.findRecurring(recurringID)
	if !ok {
		return fmt.Errorf("appstate: recurring %q not found", recurringID)
	}
	tol := oldMinor / 50 // 2%
	if tol < 1 {
		tol = 1
	}
	task := domain.Task{
		ID:       id.New(),
		Title:    "Cancel or downgrade " + r.Label,
		Notes:    r.Label + " has crept above its old price. Cancel, downgrade, or accept the increase.",
		Status:   domain.StatusOpen,
		Priority: domain.PriorityMedium,
		Source:   domain.SourceNudge,
		Custom:   map[string]any{priceCreepRecurringKey: recurringID},
		Resolve: &domain.TaskResolve{
			MatchPayee:          r.Label,
			MatchAmountMinor:    abs64(oldMinor),
			MatchCurrency:       r.Amount.Currency,
			MatchToleranceMinor: tol,
		},
	}
	return a.PutTask(task)
}

// resolvePriceCreepTasksForDeletedRecurring completes any open price-creep task
// watching the given recurring, since deleting it satisfies the "cancel" intent.
func (a *App) resolvePriceCreepTasksForDeletedRecurring(recurringID string) {
	for _, tk := range a.Tasks() {
		if tk.Status != domain.StatusOpen || tk.Custom == nil {
			continue
		}
		if v, _ := tk.Custom[priceCreepRecurringKey].(string); v == recurringID {
			if err := a.CompleteTask(tk.ID, "", a.clock()); err != nil {
				a.logErr("priceCreepTaskResolveOnDelete", err)
				continue
			}
			if a.Notifier != nil {
				a.Notifier("Done for you: " + tk.Title)
			}
		}
	}
}

// --- small helpers -------------------------------------------------------

func (a *App) findRecurring(idStr string) (domain.Recurring, bool) {
	for _, r := range a.Recurring() {
		if r.ID == idStr {
			return r, true
		}
	}
	return domain.Recurring{}, false
}

func (a *App) recurringCategory(idStr string) string {
	if r, ok := a.findRecurring(idStr); ok {
		return r.CategoryID
	}
	return ""
}

// budgetForCategory selects the first budget whose tracked categories, including
// descendants, cover categoryID. The price-creep raise path preserves this
// historical category-based selection contract.
func (a *App) budgetForCategory(categoryID string) (domain.Budget, bool) {
	if categoryID == "" {
		return domain.Budget{}, false
	}
	cats := a.Categories()
	for _, b := range a.Budgets() {
		if b.IsSaving() {
			continue
		}
		covers := categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())
		if covers[categoryID] {
			return b, true
		}
	}
	return domain.Budget{}, false
}

// budgetForRecurring selects the most specific spending budget in the recurring
// account's scope. Exact category matches outrank ancestor rollups; ties are
// resolved by stable budget ID rather than store order.
func (a *App) budgetForRecurring(r domain.Recurring) (domain.Budget, bool) {
	if r.CategoryID == "" || r.AccountID == "" {
		return domain.Budget{}, false
	}
	var account domain.Account
	foundAccount := false
	for _, candidate := range a.Accounts() {
		if candidate.ID == r.AccountID {
			account, foundAccount = candidate, true
			break
		}
	}
	if !foundAccount {
		return domain.Budget{}, false
	}
	cats := a.Categories()
	type candidate struct {
		budget domain.Budget
		exact  bool
	}
	matches := make([]candidate, 0)
	for _, b := range a.Budgets() {
		if b.IsSaving() || b.Scope != account.Scope || b.OwnerID != account.OwnerID {
			continue
		}
		covers := categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())
		if covers[r.CategoryID] {
			exact := false
			for _, tracked := range b.TrackedCategoryIDs() {
				if tracked == r.CategoryID {
					exact = true
					break
				}
			}
			matches = append(matches, candidate{budget: b, exact: exact})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].exact != matches[j].exact {
			return matches[i].exact
		}
		return matches[i].budget.ID < matches[j].budget.ID
	})
	if len(matches) == 0 {
		return domain.Budget{}, false
	}
	return matches[0].budget, true
}

func (a *App) baseCurrency() string {
	if base := a.Settings().BaseCurrency; base != "" {
		return base
	}
	return "USD"
}

// convertMinor is retained for legacy appstate callers. New code should use
// Rates.Convert directly and handle the conversion error.
func convertMinor(minor int64, from, to string, rates currency.Rates) int64 {
	if from == "" || from == to {
		return minor
	}
	converted, err := currency.ConvertBetween(minor, from, to, rates)
	if err != nil {
		return minor
	}
	return converted
}
