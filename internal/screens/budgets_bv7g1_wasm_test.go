// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBV7G1BudgetEditShowsRolloverAdjustedCap(t *testing.T) {
	b := quickFillFixtureBudget()
	b.Rollover = true
	f := render.New(t)
	status := budgeting.Status{
		Budget: b, Base: money.New(130000, "USD"), Limit: money.New(105520, "USD"),
		Rollover: budgeting.RolloverView{Current: budgeting.RolloverPeriodState{
			Base: money.New(130000, "USD"), CarryIn: money.New(-24480, "USD"), EffectiveCap: money.New(105520, "USD"),
		}},
	}
	f.Render(ui.CreateElement(bv7g1BudgetEditBasicsHarness, bv7g1BudgetEditBasicsHarnessProps{
		budget: b, status: status,
	}))

	if byTestID(f, "input", "budget-edit-limit") == nil {
		t.Fatal("Limit field did not render")
	}
	note := byTestID(f, "p", "budget-edit-effective-cap")
	want := uistate.T("budgets.effectiveCapMath", "$1,055.20", "$1,300.00 limit − $244.80 carried debt")
	if note == nil || note.Text() != want {
		t.Fatalf("effective-cap note = %v, want the card's rollover-adjusted cap %q", note, want)
	}
}

type bv7g1BudgetEditBasicsHarnessProps struct {
	budget domain.Budget
	status budgeting.Status
}

func bv7g1BudgetEditBasicsHarness(p bv7g1BudgetEditBasicsHarnessProps) ui.Node {
	fields := budgetEditFieldsProps{
		periodS: ui.UseState(string(domain.PeriodMonthly)),
		b:       p.budget, limitS: ui.UseState("1300.00"), dec: 2,
		nameS: ui.UseState(p.budget.Name), selfFound: true, selfStatus: p.status,
	}
	return ui.CreateElement(budgetEditBasics, budgetEditBasicsProps{budgetEditFieldsProps: fields})
}
