// SPDX-License-Identifier: MIT

package planning

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func recItem(label string, amt int64) domain.PlanItem {
	return domain.PlanItem{Label: label, Kind: domain.PlanItemRecurring, Amount: amt}
}

func oneItem(label string, month int, amt int64) domain.PlanItem {
	return domain.PlanItem{Label: label, Kind: domain.PlanItemOneTime, Month: month, Amount: amt}
}

func TestProjectRecurringAndOneTime(t *testing.T) {
	p := domain.Plan{
		StartBalance:  100000, // $1000.00
		HorizonMonths: 3,
		Items: []domain.PlanItem{
			recItem("Savings", 50000),        // +$500/mo
			recItem("Subscriptions", -10000), // -$100/mo  => net +400/mo
			oneItem("Bonus", 2, 200000),      // +$2000 in month 2
		},
	}
	got := Project(p)
	// net +40000/mo; month 2 also gets the +200000 bonus.
	want := []int64{140000, 380000, 420000}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("month %d = %d, want %d (full %v)", i+1, got[i], want[i], got)
		}
	}
}

func TestMonthlyNetExcludesOneTime(t *testing.T) {
	p := domain.Plan{Items: []domain.PlanItem{
		recItem("In", 30000),
		recItem("Out", -12000),
		oneItem("Windfall", 1, 999999), // must not count toward monthly net
	}}
	if got := MonthlyNet(p); got != 18000 {
		t.Errorf("MonthlyNet = %d, want 18000", got)
	}
}

func TestEndBalance(t *testing.T) {
	p := domain.Plan{StartBalance: 5000, HorizonMonths: 4, Items: []domain.PlanItem{recItem("Save", 1000)}}
	if got := EndBalance(p); got != 9000 {
		t.Errorf("EndBalance = %d, want 9000", got)
	}
}

func TestEndBalanceIncludesOneTimeItems(t *testing.T) {
	p := domain.Plan{
		StartBalance:  5000,
		HorizonMonths: 3,
		Items: []domain.PlanItem{
			recItem("Save", 1000),
			oneItem("Repair", 2, -2500),
		},
	}
	if got := EndBalance(p); got != 5500 {
		t.Errorf("EndBalance with one-time item = %d, want 5500", got)
	}
}

func TestEndBalanceNoHorizonReturnsStart(t *testing.T) {
	p := domain.Plan{StartBalance: 7777, HorizonMonths: 0, Items: []domain.PlanItem{recItem("Save", 1000)}}
	if got := EndBalance(p); got != 7777 {
		t.Errorf("EndBalance with no horizon = %d, want 7777 (start)", got)
	}
}

func TestProjectEmptyHorizon(t *testing.T) {
	if got := Project(domain.Plan{StartBalance: 100, HorizonMonths: 0}); got != nil {
		t.Errorf("empty horizon = %v, want nil", got)
	}
}

func TestProjectUnknownKindIgnored(t *testing.T) {
	// An item with an unrecognized kind contributes nothing (defensive against
	// future/garbage data) — the balance simply stays flat.
	p := domain.Plan{
		StartBalance:  1000,
		HorizonMonths: 2,
		Items:         []domain.PlanItem{{Label: "mystery", Kind: "bogus", Amount: 500}},
	}
	got := Project(p)
	want := []int64{1000, 1000}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("month %d = %d, want %d", i+1, got[i], want[i])
		}
	}
}
