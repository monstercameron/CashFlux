// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBudgetGoalFundingRowPreservesUnavailableErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ccy   string
		rates map[string]float64
		spend []int64
		want  string
	}{
		{name: "missing FX", ccy: "EUR", spend: []int64{-100}, want: "unavailable"},
		{name: "overflow", ccy: "USD", spend: []int64{-math.MaxInt64, -math.MaxInt64}, want: "unavailable"},
		{name: "valid", ccy: "EUR", rates: map[string]float64{"EUR": 1}, spend: []int64{-100}, want: "known"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := appstate.New(&bytes.Buffer{}, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := app.PutAccount(domain.Account{ID: "row-checking", Name: "Checking", Currency: tc.ccy, Type: domain.TypeChecking,
				Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.Zero(tc.ccy)}); err != nil {
				t.Fatal(err)
			}
			if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: tc.rates}); err != nil {
				t.Fatal(err)
			}
			if err := app.PutCategory(domain.Category{ID: "row-food", Name: "Food", Kind: domain.KindExpense}); err != nil {
				t.Fatal(err)
			}
			if err := app.PutGoal(domain.Goal{ID: "row-goal", Name: "Food reserve", CategoryID: "row-food", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
				TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(1000, "USD")}); err != nil {
				t.Fatal(err)
			}
			anchor := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
			for i, amount := range tc.spend {
				if err := app.PutTransaction(domain.Transaction{ID: "row-spend-" + strconv.Itoa(i), AccountID: "row-checking", Desc: "Food purchase", Date: anchor,
					CategoryID: "row-food", Amount: money.New(amount, tc.ccy), GoalID: "row-goal"}); err != nil {
					t.Fatal(err)
				}
			}
			budget := domain.Budget{ID: "row-goal-budget", Name: "Food", CategoryID: "row-food", Period: domain.PeriodMonthly,
				Limit: money.New(5000, "USD"), Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID}
			if err := app.PutBudget(budget); err != nil {
				t.Fatal(err)
			}

			previous := appstate.Default
			appstate.Default = app
			f := render.New(t)
			finished := false
			defer func() {
				if !finished {
					f.Render(ui.Fragment())
					f.Stabilize()
					f.Cleanup()
					appstate.Default = previous
					_ = app.Store().Close()
				}
			}()
			line, chip := budgetRowGoalFunding(false, budgeting.Status{Budget: budget, Limit: budget.Limit}, budgetRowProps{Anchor: anchor}, int(app.Rev()))
			f.Render(line)
			unavailable := byTestID(f, "p", "budget-goalfunded-unavailable-row-goal-budget")
			if tc.want == "unavailable" {
				if unavailable == nil || unavailable.Text() != uistate.T("budgets.fundingUnavailable") {
					t.Fatalf("row did not render a labeled unavailable state: %v", unavailable)
				}
				if chip != "" {
					t.Fatalf("unknown goal funding produced a chip: %q", chip)
				}
			} else if unavailable != nil || byTestID(f, "span", "budget-goalfunded-row-goal-budget") == nil || chip == "" {
				t.Fatalf("valid goal funding was not preserved: unavailable=%v chip=%q", unavailable, chip)
			}
			f.Render(ui.Fragment())
			f.Stabilize()
			f.Cleanup()
			finished = true
			appstate.Default = previous
			if err := app.Store().Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
