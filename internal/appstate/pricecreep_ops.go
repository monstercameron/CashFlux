// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"time"

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
	r, ok := a.findRecurring(recurringID)
	if !ok {
		return pricecreep.BudgetImpact{}
	}
	base := a.baseCurrency()
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}

	oldMonthly := abs64(r.MonthlyEquivalent())
	newR := r
	newR.Amount = money.New(-abs64(newMinor), r.Amount.Currency)
	newMonthly := abs64(newR.MonthlyEquivalent())
	deltaMonthly := convertMinor(newMonthly-oldMonthly, r.Amount.Currency, base, rates)

	b, found := a.budgetForCategory(r.CategoryID)
	if !found {
		return pricecreep.Preview("", base, 0, 0, deltaMonthly, false)
	}
	now := a.clock()
	bs, be := budgeting.PeriodRange(b.Period, now, a.weekStart())
	cats := a.Categories()
	st, err := budgeting.EvaluateRollup(b, a.Transactions(), bs, be, rates,
		budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs()))
	if err != nil {
		return pricecreep.Preview("", base, 0, 0, deltaMonthly, false)
	}
	limit := convertMinor(b.Limit.Amount, b.Limit.Currency, base, rates)
	spent := convertMinor(st.Spent.Amount, st.Spent.Currency, base, rates)
	return pricecreep.Preview(b.Name, base, limit, spent, deltaMonthly, true)
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
// monthly-equivalent delta so the higher price doesn't blow the budget (the "also
// raise the budget?" option in the accept flow). No-op when there is no budget.
func (a *App) RaiseBudgetForCreep(recurringID string, newMinor int64) error {
	imp := a.PriceCreepBudgetImpact(recurringID, newMinor)
	if !imp.HasBudget {
		return nil
	}
	b, found := a.budgetForCategory(a.recurringCategory(recurringID))
	if !found {
		return nil
	}
	b.Limit = money.New(imp.SuggestedLimitMinor, imp.Currency)
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

// budgetForCategory returns the first budget whose tracked categories (including
// descendants) cover the given category id.
func (a *App) budgetForCategory(categoryID string) (domain.Budget, bool) {
	if categoryID == "" {
		return domain.Budget{}, false
	}
	cats := a.Categories()
	for _, b := range a.Budgets() {
		covers := categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())
		if covers[categoryID] {
			return b, true
		}
	}
	return domain.Budget{}, false
}

func (a *App) baseCurrency() string {
	if base := a.Settings().BaseCurrency; base != "" {
		return base
	}
	return "USD"
}

func (a *App) weekStart() time.Weekday { return time.Monday }

// convertMinor converts minor units between currencies via the rate table,
// returning the input unchanged when currencies match or a rate is missing.
func convertMinor(minor int64, from, to string, rates currency.Rates) int64 {
	if from == "" || from == to {
		return minor
	}
	v, err := currency.ConvertBetween(minor, from, to, rates)
	if err != nil {
		return minor
	}
	return v
}
