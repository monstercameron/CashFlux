// SPDX-License-Identifier: MIT

package planning

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
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

// ─── FP-T3a: growth and rate-of-return ───────────────────────────────────────

func growItem(label string, amt int64, growthPct float64) domain.PlanItem {
	return domain.PlanItem{
		ID: label, Label: label, Kind: domain.PlanItemRecurring,
		Amount: amt, AnnualGrowthPct: growthPct,
	}
}

// A plan with neither growth nor return must go through the forecast engine
// exactly as before, so the linear case cannot drift from the rest of the app.
func TestFlatPlanIsUnchangedByTheCompoundingPath(t *testing.T) {
	p := domain.Plan{HorizonMonths: 12, StartBalance: 100_000, Items: []domain.PlanItem{
		recItem("salary", 5_000), oneItem("bonus", 6, 20_000),
	}}
	got := Project(p)
	rec, one := toForecastInputs(p)
	want := forecast.Project(p.StartBalance, rec, one, p.HorizonMonths)
	if len(got) != len(want) {
		t.Fatalf("length %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("month %d = %d, want the forecast engine's %d", i+1, got[i], want[i])
		}
	}
}

// A raise arrives in one month; the eleven months before it are unaffected.
// Smoothing would misstate every intermediate month to make the end come out.
func TestARaiseLandsOnItsAnniversaryNotSmoothly(t *testing.T) {
	p := domain.Plan{HorizonMonths: 24, Items: []domain.PlanItem{growItem("salary", 100_000, 10)}}
	curve := Project(p)
	// Months 1–12 are year 0: a flat 100,000 a month.
	if curve[0] != 100_000 {
		t.Errorf("month 1 = %d, want 100000", curve[0])
	}
	if curve[11] != 1_200_000 {
		t.Errorf("month 12 = %d, want 12 months at the un-raised amount", curve[11])
	}
	// Month 13 opens year 1 and adds the raised figure.
	if got := curve[12] - curve[11]; got != 110_000 {
		t.Errorf("month 13 added %d, want the raised 110000", got)
	}
}

func TestGrowthCompoundsOnTheAlreadyRaisedAmount(t *testing.T) {
	// A 10% raise in year two is 10% of 110, not of 100.
	p := domain.Plan{HorizonMonths: 36, Items: []domain.PlanItem{growItem("salary", 100_000, 10)}}
	curve := Project(p)
	year2 := curve[12] - curve[11]
	year3 := curve[24] - curve[23]
	if year2 != 110_000 {
		t.Errorf("year 2 monthly = %d, want 110000", year2)
	}
	if year3 != 121_000 {
		t.Errorf("year 3 monthly = %d, want 121000 (compounded, not 120000)", year3)
	}
}

func TestAShrinkingItemIsAllowed(t *testing.T) {
	// Negative growth is real — a subscription being wound down, a loan payment
	// falling — and clamping it would silently model something else.
	p := domain.Plan{HorizonMonths: 24, Items: []domain.PlanItem{growItem("cost", -50_000, -20)}}
	curve := Project(p)
	y1 := curve[0]
	y2 := curve[12] - curve[11]
	if y2 <= y1 {
		t.Errorf("a -20%% growth on a -50000 cost should shrink the outflow: %d then %d", y1, y2)
	}
	if y2 != -40_000 {
		t.Errorf("year 2 = %d, want -40000", y2)
	}
}

func TestReturnAppliesToTheOpeningBalanceNotTheClosingOne(t *testing.T) {
	// Crediting a return on money that arrives during the month would pay a full
	// month's return on a deposit made on the last day.
	p := domain.Plan{HorizonMonths: 1, StartBalance: 1_200_000, AnnualReturnPct: 12,
		Items: []domain.PlanItem{recItem("deposit", 1_000_000)}}
	curve := Project(p)
	// 1% of the opening 1,200,000 is 12,000, then the deposit lands.
	if curve[0] != 1_200_000+12_000+1_000_000 {
		t.Errorf("month 1 = %d, want the return on the OPENING balance only", curve[0])
	}
}

// A positive rate on a negative balance would make debt shrink by itself — the
// most flattering possible bug.
func TestANegativeBalanceEarnsNothing(t *testing.T) {
	p := domain.Plan{HorizonMonths: 3, StartBalance: -500_000, AnnualReturnPct: 12}
	curve := Project(p)
	for i, v := range curve {
		if v != -500_000 {
			t.Fatalf("month %d = %d, want the debt untouched at -500000", i+1, v)
		}
	}
}

func TestReturnCompoundsAcrossMonths(t *testing.T) {
	p := domain.Plan{HorizonMonths: 2, StartBalance: 1_000_000, AnnualReturnPct: 12}
	curve := Project(p)
	if curve[0] != 1_010_000 {
		t.Errorf("month 1 = %d, want 1010000", curve[0])
	}
	// The second month earns on the first month's result, not on the start.
	if curve[1] != 1_020_100 {
		t.Errorf("month 2 = %d, want 1020100 (compounded)", curve[1])
	}
}

func TestGrowthAndReturnComposeWithoutHorizonSurprises(t *testing.T) {
	p := domain.Plan{HorizonMonths: 0, StartBalance: 500_000, AnnualReturnPct: 8,
		Items: []domain.PlanItem{growItem("salary", 100_000, 3)}}
	if got := Project(p); len(got) != 0 {
		t.Errorf("a zero horizon projected %d months, want none", len(got))
	}
	if got := EndBalance(p); got != 500_000 {
		t.Errorf("end balance = %d, want the start balance when nothing is projected", got)
	}
}

func TestOneTimeItemsIgnoreGrowth(t *testing.T) {
	// A single payment in month 7 has no year over which to grow.
	it := oneItem("bonus", 7, 50_000)
	it.AnnualGrowthPct = 50
	p := domain.Plan{HorizonMonths: 24, AnnualReturnPct: 0, Items: []domain.PlanItem{it}}
	curve := Project(p)
	if curve[6]-curve[5] != 50_000 {
		t.Errorf("month 7 added %d, want the un-grown 50000", curve[6]-curve[5])
	}
	// And it happens once.
	if curve[23] != 50_000 {
		t.Errorf("end = %d, want a single 50000", curve[23])
	}
}
