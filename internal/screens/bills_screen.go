// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// Bills is the /bills route — a thin shell over the unified Bills & Recurring
// surface (RhythmSurface), landing on the up-next agenda.
//
// What remains in this file is the budget-fit chip the recurring agenda draws on.
// The old BillsPanel and its calendar, row and projection helpers were deleted
// (WF9-b): nothing had mounted them since /bills became a shell, and their doc
// comment still claimed they were routed — which is how a feature came to be
// built into dead code. Their behaviour lives on the rhythm surface.
func Bills() ui.Node {
	return rhythmSurfaceFocused(focusAgenda)
}

// monthLabel renders a month/year heading like "June 2026".
func monthLabel(t time.Time) string { return t.Format("January 2006") }

// billFitChip is the "does this bill fit its budget?" verdict shown on a bill row —
// the analytical link from Bills to Budgets. It's computed for recurring-derived
// bills that map to a category a budget tracks, comparing the (FX-converted) charge
// against what's left in that budget for the period the bill lands in.
type billFitChip struct {
	BudgetID   string
	BudgetName string
	Fits       bool
	Amount     string // formatted room-left (fits) or amount-over (doesn't)
}

// billCategoryID resolves the spending category a bill maps to, or "" when it has
// none to budget against. Only recurring-derived bills carry a category (their
// AccountID is "recurring:<id>"); account statement bills (a liability's minimum
// payment) aren't category spend, so they get no fit chip.
func billCategoryID(app *appstate.App, b bills.Bill) string {
	const prefix = "recurring:"
	if !strings.HasPrefix(b.AccountID, prefix) {
		return ""
	}
	rid := strings.TrimPrefix(b.AccountID, prefix)
	for _, r := range app.Recurring() {
		if r.ID == rid {
			return r.CategoryID
		}
	}
	return ""
}

// billFitFor is the pointer-returning wrapper the row builder uses: the fit chip
// when the bill maps to a tracked budget, or nil (no chip).
func billFitFor(b bills.Bill) *billFitChip {
	if c, ok := billBudgetFit(b); ok {
		return &c
	}
	return nil
}

// billBudgetFit computes whether a bill fits the budget that tracks its category,
// for the period the bill's due date falls in. Returns ok=false when the bill has
// no category, no budget tracks it, or the budget carries no positive limit — in
// all those cases the row simply shows no chip. It mirrors the engine's own spend
// (budgeting.Spent) so the chip reconciles with the Budgets page.
func billBudgetFit(b bills.Bill) (billFitChip, bool) {
	app := appstate.Default
	if app == nil {
		return billFitChip{}, false
	}
	catID := billCategoryID(app, b)
	if catID == "" {
		return billFitChip{}, false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	cats := app.Categories()
	var matched domain.Budget
	var matchedDesc map[string]bool
	found := false
	for _, bd := range app.Budgets() {
		desc := categorytree.DescendantsOfAll(cats, bd.TrackedCategoryIDs())
		if desc[catID] {
			matched = bd
			matchedDesc = desc
			found = true
			break
		}
	}
	if !found {
		return billFitChip{}, false
	}
	limit := matched.Limit
	if limit.Currency == "" {
		limit = money.New(limit.Amount, base)
	}
	if limit.Amount <= 0 {
		return billFitChip{}, false
	}
	start, end := budgeting.PeriodRange(matched.Period, b.DueDate, uistate.LoadPrefs().WeekStartWeekday())
	spent, err := budgeting.Spent(matched, app.Transactions(), start, end, rates)
	if err != nil {
		return billFitChip{}, false
	}
	// Reconcile against the OTHER upcoming bills that also land in this budget and
	// period: without this, three subscriptions each due this month would every one
	// report "fits" against the same posted-only baseline, then together blow the
	// budget. Folding the siblings into the baseline makes each chip honest — it's
	// the verdict for paying this bill on top of everything else already committed.
	now := time.Now()
	var pending int64
	for _, ob := range bills.UpcomingAll(app.Accounts(), app.Recurring(), now) {
		if ob.AccountID == b.AccountID && ob.DueDate.Equal(b.DueDate) {
			continue // this bill itself
		}
		if ob.DueDate.Before(start) || !ob.DueDate.Before(end) {
			continue // a different period
		}
		ocat := billCategoryID(app, ob)
		if ocat == "" || !matchedDesc[ocat] {
			continue // not this budget
		}
		if conv, err := rates.Convert(ob.Amount, limit.Currency); err == nil {
			pending += conv.Amount
		}
	}
	billConv, err := rates.Convert(b.Amount, limit.Currency)
	if err != nil {
		return billFitChip{}, false
	}
	fit := budgeting.FitBill(limit.Amount, spent.Amount+pending, billConv.Amount)
	chip := billFitChip{BudgetID: matched.ID, BudgetName: matched.Name, Fits: fit.Fits}
	if fit.Fits {
		chip.Amount = fmtMoney(money.New(fit.LeftAfter, limit.Currency))
	} else {
		chip.Amount = fmtMoney(money.New(fit.OverBy, limit.Currency))
	}
	return chip, true
}

// sameDay reports whether two times fall on the same calendar date.
func sameDay(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }
