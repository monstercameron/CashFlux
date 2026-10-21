// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/agemoney"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func budgetAgeMoneyOverflowFixture(result agemoney.Result) ui.Node {
	_, _, chip := budgetSummaryChips(budgetSummaryChipsProps{v: budgetView{AgeMoney: result}})
	return chip
}

func TestBudgetAgeOfMoneyOverflowShowsUnavailableReason(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(budgetAgeMoneyOverflowFixture, agemoney.Result{Overflow: true}))
	f.Stabilize()
	chip := byTestID(f, "span", "budgets-agemoney-unavailable")
	if chip == nil || !strings.Contains(chip.Text(), uistate.T("budgets.ageMoneyUnavailable")) || !strings.Contains(chip.Text(), uistate.T("budgets.ageMoneyTotalsTooLarge")) {
		t.Fatalf("overflow did not show its unavailable reason: %q", f.Text())
	}
	if strings.Contains(f.Text(), "0 days") || strings.Contains(f.Text(), uistate.T("budgets.ageMoneyNotReady")) {
		t.Fatalf("overflow was presented as zero days or insufficient history: %q", f.Text())
	}
}
