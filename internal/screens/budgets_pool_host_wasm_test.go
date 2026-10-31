// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
)

func TestBudgetSummaryAllocationHidesOverflowedIncomeBasis(t *testing.T) {
	result := budgetSummaryAllocation(budgetSummaryAllocationProps{
		futurePeriod: true,
		v:            budgetView{Method: budgeting.MethodZeroBased, BannerIncome: math.MaxInt64, RolledOver: 1},
	})
	if result.hasAlloc {
		t.Fatal("overflowed income basis was treated as an available allocation")
	}
}
