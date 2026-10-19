// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestBQ10A1FundsImpactLineFollowsSelectedReach(t *testing.T) {
	periodOnly := fundsImpactLine(budgeting.TopUpImpact(false, false), fundsImpactOption{
		reach: fundsReachThisPeriod,
	})
	if !strings.Contains(periodOnly, "this period only") || strings.Contains(periodOnly, "budgets you selected") {
		t.Fatalf("unfunded period-only impact = %q", periodOnly)
	}

	funded := fundsImpactLine(budgeting.TopUpImpact(false, true), fundsImpactOption{
		reach: fundsReachThisPeriod, movesFromSelectedBudgets: true,
	})
	if !strings.Contains(funded, "this period only") || !strings.Contains(funded, "budgets you selected") {
		t.Fatalf("funded period-only impact = %q", funded)
	}

	permanent := fundsImpactLine(budgeting.ImpactAdjustAll, fundsImpactOption{reach: fundsReachPermanent})
	if !strings.Contains(permanent, "permanently") || strings.Contains(permanent, "this period only") {
		t.Fatalf("permanent impact = %q", permanent)
	}
	if got := fundsImpactLine(budgeting.ImpactRemoveRecurringCover); !strings.Contains(got, "future periods only") {
		t.Fatalf("future-only impact = %q", got)
	}
	if got := fundsImpactLine(budgeting.ImpactDeleteBudget); !strings.Contains(got, "removes the budget") {
		t.Fatalf("delete impact = %q", got)
	}

	if got := uistate.T("budgets.impactReachThisAndLater"); !strings.Contains(got, "every later period") {
		t.Fatalf("this-and-later translation = %q", got)
	}
}
