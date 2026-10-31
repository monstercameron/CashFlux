// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type br3dlgRaiseHarnessProps struct {
	amountText string
	permanent  bool
}

func br3dlgRaiseHarness(props br3dlgRaiseHarnessProps) ui.Node {
	permanent := ui.UseState(props.permanent)
	selected := ui.UseState(map[string]bool{})
	coverOpen := ui.UseState(false)
	amount := ui.UseState("")
	amount.Set(props.amountText)
	submit := ui.UseEvent(Prevent(func() {}))
	input := ui.UseEvent(func(ui.Event) {})
	toggle := ui.UseEvent(Prevent(func() {}))
	return budgetEditRaiseLimit(budgetEditRaiseLimitProps{
		topupPermanentS: permanent, b: domain.Budget{ID: "groceries", Name: "Groceries", Limit: money.New(45000, "USD")},
		coverSelS: selected, coverSrcs: nil, topupCoverOpenS: coverOpen, topupAmt: amount, dec: 2,
		vw: period.Window{}, submitTopup: submit, onTopupAmt: input, onToggleTopupCover: toggle,
		cur: "USD", cancel: submit,
	})
}

func TestBR3DLGRaiseBodyDoesNotRepeatDialogTitleAndDisablesEmptySubmit(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(br3dlgRaiseHarness, br3dlgRaiseHarnessProps{}))
	title := byTestID(f, "h3", "budget-raise-title")
	field := byTestID(f, "input", "budget-topup-amt")
	if title != nil {
		t.Fatalf("raise body repeats the outer dialog title: %q", title.Text())
	}
	if field == nil || field.Attr("placeholder") == uistate.T("budgets.raiseAmount") {
		t.Fatalf("raise amount placeholder repeats its label: %v", field)
	}
	button := byTestID(f, "button", "budget-raise-submit")
	if button == nil || !isDisabled(button) {
		t.Fatalf("raise action should be disabled for an empty amount: %v", button)
	}
	capMath := byTestID(f, "p", "topup-cap-math")
	if capMath == nil {
		t.Fatal("raise dialog has no cap explanation")
	}
	monthCopy := capMath.Text()
	f.Render(ui.CreateElement(br3dlgRaiseHarness, br3dlgRaiseHarnessProps{amountText: "37.30", permanent: true}))
	button = byTestID(f, "button", "budget-raise-submit")
	if button == nil || isDisabled(button) || button.Text() != "Raise by $37.30" {
		t.Fatalf("valid raise action = %v, want enabled Raise by $37.30", button)
	}
	capMath = byTestID(f, "p", "topup-cap-math")
	if capMath == nil || capMath.Text() != monthCopy {
		t.Fatalf("cap explanation changed by duration: month %q, permanent %v", monthCopy, capMath)
	}
}

func TestBR3DLGQuickPicksUseRowOverspendAndProjection(t *testing.T) {
	status := budgeting.Status{
		Budget:     domain.Budget{ID: "groceries", Name: "Groceries"},
		Limit:      money.New(45000, "USD"),
		Spent:      money.New(48730, "USD"),
		Remaining:  money.New(-3730, "USD"),
		State:      budgeting.StateOver,
		Projection: budgeting.Pace{OverBy: money.New(21100, "USD")},
	}
	cover, projected := budgetRaiseQuickPickAmounts(status)
	if cover != 3730 || projected != 21100 {
		t.Fatalf("quick-pick amounts = %d/%d, want 3730/21100 minor units", cover, projected)
	}
	status.Projection.Error = errors.New("projection unavailable")
	_, projected = budgetRaiseQuickPickAmounts(status)
	if projected != 0 {
		t.Fatalf("unavailable projection offered %d minor units", projected)
	}
}

func TestBR3DLGCoverRequiresExplicitDonorAndNormalizesYearlyRoom(t *testing.T) {
	if coverAllHasDonor(map[string]string{"groceries": ""}) {
		t.Fatal("Cover should stay disabled before a donor is selected")
	}
	if !coverAllHasDonor(map[string]string{"groceries": "dining"}) {
		t.Fatal("Cover should enable after choosing a donor")
	}
	room, err := coverAllMonthlyRoom(money.New(330000, "USD"), domain.PeriodYearly)
	if err != nil {
		t.Fatalf("monthly equivalent room: %v", err)
	}
	if room.Amount != 27500 {
		t.Fatalf("yearly room shown as %d, want monthly equivalent 27500", room.Amount)
	}
}
