// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBR2MOBCompactFiguresAndPaceTick(t *testing.T) {
	guardTestApp(t)
	b := quickFillFixtureBudget()
	b.Limit = money.New(20000, "USD")
	props := budgetRowProps{
		Status: budgeting.Status{
			Budget: b, Limit: b.Limit, Spent: money.New(10000, "USD"),
			Remaining: money.New(10000, "USD"), Percent: 50, State: budgeting.StateOK,
		},
		Compact: true, PaceMarkerPct: 45, OnDelete: func(string) {},
	}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, props))

	figures := compactBudgetFiguresBR2MOB(f)
	if figures == nil {
		t.Fatal("compact row does not group its amount and remaining text")
	}
	if got := strings.Join(strings.Fields(figures.Text()), " "); !strings.Contains(got, "$100.00 of $200.00") || !strings.Contains(got, "$100.00 left") {
		t.Fatalf("compact figures = %q, want spent of limit and remaining amount", got)
	}
	if tick := byTestID(f, "div", "budget-pace-tick-"+b.ID); tick == nil || tick.Attr("aria-hidden") != "true" {
		t.Fatal("compact progress bar is missing its decorative pace tick")
	}

	props.Status.Spent = money.Zero("USD")
	props.Status.Remaining = b.Limit
	f.Rerender(ui.CreateElement(BudgetRow, props))
	if tick := byTestID(f, "div", "budget-pace-tick-"+b.ID); tick == nil || tick.Attr("aria-hidden") != "true" {
		t.Fatal("current-period pace tick should remain visible when nothing has been spent")
	}
}

func compactBudgetFiguresBR2MOB(f *render.Fixture) *render.QueryNode {
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "budget-crow-figures") {
			return node
		}
	}
	return nil
}
