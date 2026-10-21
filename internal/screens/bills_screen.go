// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/billfit"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
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
	Available  bool
	Fits       bool
	Amount     string // formatted room-left (fits) or amount-over (doesn't)
}

// prepareBillFitModel assembles one checked data snapshot for all agenda rows.
// A failed source read is passed through as unavailable rather than as an empty
// ledger or a zero-spend budget.
func prepareBillFitModel(app *appstate.App, candidates []bills.Bill, now time.Time) *billfit.Prepared {
	if app == nil || app.Store() == nil {
		return billfit.Prepare(billfit.Input{Candidates: candidates, Now: now, SourceError: errors.New("app data is unavailable")})
	}
	st := app.Store()
	settings, settingsErr := st.GetSettings()
	if err := st.CheckReady(); err != nil {
		settingsErr = errors.Join(settingsErr, err)
	}
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	accounts, accountsErr := app.AccountsChecked()
	categories, categoriesErr := app.CategoriesChecked()
	budgets, budgetsErr := app.BudgetsChecked()
	transactions, transactionsErr := app.TransactionsChecked()
	links, linksErr := app.TxnLinksChecked()
	recurring, recurringErr := st.ListRecurring()
	sourceErr := errors.Join(settingsErr, accountsErr, transactionsErr, linksErr)
	indexErr := errors.Join(categoriesErr, budgetsErr, recurringErr)
	return billfit.Prepare(billfit.Input{
		Candidates: candidates, Accounts: accounts, Recurring: recurring, Budgets: budgets, Categories: categories,
		Ledger: budgeting.NewNettedLedger(transactions, links), Rates: currency.Rates{Base: base, Rates: settings.FXRates},
		Now: now, WeekStart: uistate.LoadPrefs().WeekStartWeekday(), SourceError: sourceErr, IndexError: indexErr,
	})
}

// billFitFor converts a prepared native fact to the row chip presentation.
func billFitFor(prepared *billfit.Prepared, b bills.Bill) *billFitChip {
	fact := prepared.Fit(b)
	if !fact.HasBudget {
		return nil
	}
	chip := &billFitChip{BudgetID: fact.BudgetID, BudgetName: fact.BudgetName, Available: fact.Available, Fits: fact.Fits}
	if fact.Available {
		chip.Amount = fmtMoney(fact.Amount)
	}
	return chip
}

// billCategoryID preserves the shared bill-category lookup for other Bills
// interactions while delegating the identity parsing to the native model.
func billCategoryID(app *appstate.App, b bills.Bill) string {
	if app == nil {
		return ""
	}
	return billfit.CategoryID(b.AccountID, app.Recurring())
}

// sameDay reports whether two times fall on the same calendar date.
func sameDay(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }
