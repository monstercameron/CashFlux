// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBudgetBV7DToolsMenuDoesNotOfferMixedPeriodLastMonthOverlay(t *testing.T) {
	noop := ui.Handler{}
	f := render.New(t)
	f.Render(ui.CreateElement(budgetToolbarToolsMenu, budgetToolbarToolsMenuProps{
		HasBudgets: true,
		OnClose:    noop, OnAuto: noop, OnMonthClose: noop, OnSweep: noop, OnAdjust: noop,
	}))

	if got := byTestID(f, "button", "budgets-last-month"); got != nil {
		t.Fatalf("last-month overlay action is still offered: %q", got.Text())
	}
	if got := byTestID(f, "button", "budgets-autobudget"); got == nil {
		t.Fatal("removing the last-month view also removed the unrelated Auto Budget action")
	}
	if strings.Contains(strings.ToUpper(f.Container().Text()), "LAST MONTH") {
		t.Fatalf("Tools menu still labels a mixed-period view: %q", f.Container().Text())
	}

	status := budgeting.Status{
		Budget:  domain.Budget{ID: "groceries", Name: "Groceries", Limit: money.New(45_000, "USD")},
		Limit:   money.New(45_000, "USD"),
		Spent:   money.New(69_480, "USD"),
		Percent: 154,
	}
	values := budgetRowBarValues(status, budgetRowProps{
		LastMonthSpent: "$380.00", LastMonthPct: 84, LastMonthFill: 84,
	}, false)
	if values.barSpent != "$694.80" || values.barPct != 154 {
		t.Fatalf("current period row figures = %q / %d%%, want $694.80 / 154%%", values.barSpent, values.barPct)
	}
}
