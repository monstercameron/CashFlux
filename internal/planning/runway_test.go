// SPDX-License-Identifier: MIT

package planning

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// approxEqual returns true when a and b differ by less than eps.
func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func TestRunwayMonthsSteadyDrawdown(t *testing.T) {
	// Dev & Priya scenario: $22,500 start, -$4,000/mo, 12-month horizon.
	// 22500 / 4000 = 5.625 months.
	p := domain.Plan{
		StartBalance:  2250000, // $22,500.00 in minor units (cents)
		HorizonMonths: 12,
		Items:         []domain.PlanItem{recItem("Expenses", -400000)}, // -$4,000.00/mo
	}
	months, depletes := RunwayMonths(p)
	if !depletes {
		t.Fatal("expected depletes=true")
	}
	// month 6 end: 2250000 - 6*400000 = -150000 (crosses zero between month 5 and 6)
	// prev (end month 5) = 2250000 - 5*400000 = 250000
	// cur  (end month 6) = -150000
	// frac = 250000 / (250000 + 150000) = 250000/400000 = 0.625
	// months = 5 + 0.625 = 5.625
	want := 5.625
	if !approxEqual(months, want, 1e-9) {
		t.Errorf("RunwayMonths = %.6f, want %.6f", months, want)
	}
}

func TestRunwayMonthsNeverDepletes(t *testing.T) {
	// Positive net — balance rises every month; should never deplete.
	p := domain.Plan{
		StartBalance:  500000,
		HorizonMonths: 6,
		Items:         []domain.PlanItem{recItem("Salary", 100000)},
	}
	months, depletes := RunwayMonths(p)
	if depletes {
		t.Errorf("expected depletes=false, got true (months=%.2f)", months)
	}
	if months != 0 {
		t.Errorf("months = %.2f, want 0 when not depleting", months)
	}
}

func TestRunwayMonthsAlreadyNegativeNonIncreasing(t *testing.T) {
	// Start below zero with spending — depletes immediately at month 0.
	p := domain.Plan{
		StartBalance:  -10000,
		HorizonMonths: 3,
		Items:         []domain.PlanItem{recItem("Expenses", -5000)},
	}
	months, depletes := RunwayMonths(p)
	if !depletes {
		t.Fatal("expected depletes=true for already-negative start")
	}
	if months != 0 {
		t.Errorf("months = %.2f, want 0 (already negative)", months)
	}
}

func TestRunwayMonthsExactlyHitsZero(t *testing.T) {
	// Balance lands exactly on zero — by spec, zero is NOT depleted.
	p := domain.Plan{
		StartBalance:  300000,
		HorizonMonths: 3,
		Items:         []domain.PlanItem{recItem("Expenses", -100000)}, // 3 × 100000 = 300000
	}
	months, depletes := RunwayMonths(p)
	if depletes {
		t.Errorf("expected depletes=false when balance hits exactly zero (months=%.2f)", months)
	}
	if months != 0 {
		t.Errorf("months = %.2f, want 0 when not depleting", months)
	}
}

func TestRunwayMonthsOneTimeDip(t *testing.T) {
	// $5,000 start, +$0/mo, one-time -$6,000 in month 3 → dips to -$1,000 in month 3.
	// prev (end month 2) = 5000, cur (end month 3) = 5000 - 6000 = -1000
	// frac = 5000 / (5000 + 1000) = 5/6 ≈ 0.8333...
	// months = 2 + 5/6 ≈ 2.8333...
	p := domain.Plan{
		StartBalance:  500000,
		HorizonMonths: 6,
		Items:         []domain.PlanItem{oneItem("Big expense", 3, -600000)},
	}
	months, depletes := RunwayMonths(p)
	if !depletes {
		t.Fatal("expected depletes=true")
	}
	want := 2.0 + 5.0/6.0
	if !approxEqual(months, want, 1e-9) {
		t.Errorf("RunwayMonths = %.9f, want %.9f", months, want)
	}
}

func TestRunwayMonthsHorizonShorterThanDepletion(t *testing.T) {
	// Would deplete at month 10, but horizon is only 4 — so depletes=false.
	p := domain.Plan{
		StartBalance:  4000000,
		HorizonMonths: 4,
		Items:         []domain.PlanItem{recItem("Draw", -400000)}, // depletes at month 10
	}
	months, depletes := RunwayMonths(p)
	if depletes {
		t.Errorf("expected depletes=false when horizon ends before depletion (months=%.2f)", months)
	}
	if months != 0 {
		t.Errorf("months = %.2f, want 0", months)
	}
}

func TestRunwayMonthsZeroHorizon(t *testing.T) {
	p := domain.Plan{StartBalance: 100000, HorizonMonths: 0, Items: []domain.PlanItem{recItem("Draw", -10000)}}
	months, depletes := RunwayMonths(p)
	if depletes || months != 0 {
		t.Errorf("zero horizon: got (%.2f, %v), want (0, false)", months, depletes)
	}
}

func TestRunwayMonthsFirstMonthCrossing(t *testing.T) {
	// Start $500, lose $2000 in month 1 → crosses immediately.
	// prev = 500, cur = 500 - 2000 = -1500
	// frac = 500 / (500 + 1500) = 500/2000 = 0.25
	// months = 0 + 0.25 = 0.25
	p := domain.Plan{
		StartBalance:  50000,
		HorizonMonths: 3,
		Items:         []domain.PlanItem{recItem("Burn", -200000)},
	}
	months, depletes := RunwayMonths(p)
	if !depletes {
		t.Fatal("expected depletes=true")
	}
	want := 0.25
	if !approxEqual(months, want, 1e-9) {
		t.Errorf("RunwayMonths = %.9f, want %.9f", months, want)
	}
}
