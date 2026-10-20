// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type annualProjectionMemoFixtureProps struct {
	Props    budgetAnnualGridProps
	Build    *budgetview.AnnualBuild
	Open     bool
	Revision int
	Scenario int64
	Window   string
	Capture  func(*budgetview.AnnualProjection)
}

func annualProjectionMemoFixture(p annualProjectionMemoFixtureProps) ui.Node {
	projection := useBudgetAnnualProjection(p.Props, p.Build, p.Open, p.Revision)
	p.Capture(projection)
	return P(p.Window)
}

func TestAnnualProjectionMemoReusesScenarioAndWindowUpdates(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	budgets := []domain.Budget{{ID: "food", Name: "Food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(1000, "USD")}}
	categories := []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	build := budgetview.BuildAnnual(budgets, nil, categories, 2026, rates, time.Monday, now)
	recurrings := []domain.Recurring{
		{ID: "first", CategoryID: "food", Amount: money.New(-100, "EUR"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "second", CategoryID: "food", Amount: money.New(-50, "EUR"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
	}
	var got *budgetview.AnnualProjection
	props := annualProjectionMemoFixtureProps{Props: budgetAnnualGridProps{Now: now, Rates: rates, Recurrings: recurrings[:1]},
		Build: &build, Open: true, Window: "all", Capture: func(value *budgetview.AnnualProjection) { got = value }}
	f := render.New(t)
	defer f.Cleanup()
	renderValue := func() {
		f.Render(ui.CreateElement(annualProjectionMemoFixture, props))
		f.Stabilize()
	}
	renderValue()
	if got.Err != nil || got.Totals[10] != 200 {
		t.Fatalf("initial annual projection = %+v", got)
	}
	first := got
	props.Scenario, props.Window = -10000, "h2"
	renderValue()
	if got != first {
		t.Fatal("presentation controls repeated recurring/goal projection")
	}
	props.Props.Recurrings = recurrings[:2]
	renderValue()
	if got == first || got.Totals[10] != 300 {
		t.Fatalf("same-pointer recurring reslice retained old projection: %+v", got)
	}
	first = got
	props.Props.Rates = currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 3}}
	renderValue()
	if got == first || got.Totals[10] != 450 {
		t.Fatalf("FX change retained old annual projection: %+v", got)
	}
	props.Open = false
	renderValue()
	if len(got.PerBudget) != 0 || got.Totals[10] != 0 {
		t.Fatal("closed grid retained computed projection")
	}
	props.Open = true
	props.Revision++
	renderValue()
	if got.Err != nil || got.Totals[10] != 450 {
		t.Fatalf("reopened grid did not refresh projection: %+v", got)
	}
}
