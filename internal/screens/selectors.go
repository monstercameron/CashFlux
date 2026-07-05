// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// This file holds the dashboard's render-time *derived selectors* — net worth,
// period totals, budget health — wrapped in ui.UseMemo so each recomputes
// only when the underlying data actually changes, not on every re-render (§1.6).
// (state.UseComputed until GWC v4 soft-deprecated it in favor of UseMemo, which
// returns the value directly instead of wrapping it in an atom.)
//
// Correctness hinges on the memo key: app.Rev() is an O(1) monotonic counter the
// store advances on every entity write/delete AND on settings (base currency / FX)
// writes, so it changes exactly when — and only when — a derived value could
// change. UI-only inputs that also affect a result (the active period, the
// active-member filter) are added as extra deps so switching them recomputes too.

// useNetWorth returns a memoized net-worth breakdown. Net worth spans all accounts
// and all time, so the data/FX revision (app.Rev()) is a complete key.
func useNetWorth(app *appstate.App, accounts []domain.Account, txns []domain.Transaction, rates currency.Rates) ledger.NetWorthResult {
	return ui.UseMemo(func() ledger.NetWorthResult {
		nw, _ := ledger.NetWorthExplained(accounts, txns, rates)
		return nw
	}, app.Rev())
}

// usePeriodTotals returns memoized income/expense for the period over the given
// (already member-filtered) transactions. Period bounds and a member signature are
// extra deps so changing the active period or member recomputes without a data
// change.
func usePeriodTotals(app *appstate.App, txns []domain.Transaction, start, end time.Time, rates currency.Rates, memberSig string) (money.Money, money.Money) {
	type totals struct{ income, expense money.Money }
	v := ui.UseMemo(func() totals {
		i, e, _ := ledger.PeriodTotals(txns, start, end, rates)
		return totals{i, e}
	}, app.Rev(), start.Unix(), end.Unix(), memberSig)
	return v.income, v.expense
}

// useBudgetHealth returns the memoized per-budget status (health) for the period.
func useBudgetHealth(app *appstate.App, budgets []domain.Budget, txns []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64) []budgeting.Status {
	return ui.UseMemo(func() []budgeting.Status {
		st, _ := budgeting.EvaluateAll(budgets, txns, start, end, rates, nearThreshold)
		return st
	}, app.Rev(), start.Unix(), end.Unix())
}
