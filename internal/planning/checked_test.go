// SPDX-License-Identifier: MIT
package planning

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
	"github.com/monstercameron/CashFlux/internal/money"
	"math"
	"reflect"
	"testing"
)

// frozenPlanProject preserves the prior flat/compounding split, monthly return
// order, anniversary growth and per-step rounding independently of the helpers.
func frozenPlanProject(p domain.Plan) []int64 {
	if p.HorizonMonths <= 0 {
		return nil
	}
	comp := p.AnnualReturnPct != 0
	for _, it := range p.Items {
		comp = comp || it.Kind == domain.PlanItemRecurring && it.AnnualGrowthPct != 0
	}
	curve := make([]int64, p.HorizonMonths)
	balance := p.StartBalance
	var net int64
	for _, it := range p.Items {
		if it.Kind == domain.PlanItemRecurring {
			net += it.Amount
		}
	}
	for month := 1; month <= p.HorizonMonths; month++ {
		if !comp {
			balance += net
			for _, it := range p.Items {
				if it.Kind == domain.PlanItemOneTime && it.Month == month {
					balance += it.Amount
				}
			}
		} else {
			if balance > 0 && p.AnnualReturnPct != 0 {
				balance += int64(math.Round(float64(balance) * (p.AnnualReturnPct / 100 / 12)))
			}
			for _, it := range p.Items {
				if it.Kind == domain.PlanItemRecurring {
					amount := it.Amount
					years := (month - 1) / 12
					if it.AnnualGrowthPct != 0 && years > 0 {
						amount = int64(math.Round(float64(amount) * math.Pow(1+it.AnnualGrowthPct/100, float64(years))))
					}
					balance += amount
				} else if it.Kind == domain.PlanItemOneTime && it.Month == month {
					balance += it.Amount
				}
			}
		}
		curve[month-1] = balance
	}
	return curve
}
func TestCheckedPlanParityWithFrozenModel(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan domain.Plan
	}{
		{"flat", domain.Plan{StartBalance: 100, HorizonMonths: 24, Items: []domain.PlanItem{recItem("in", 3000), recItem("out", -2000), oneItem("repair", 2, -600), oneItem("outside", 0, 100)}}},
		{"growth return", domain.Plan{StartBalance: 1200000, HorizonMonths: 36, AnnualReturnPct: 12, Items: []domain.PlanItem{growItem("salary", 100000, 10), growItem("cost", -50000, -20), oneItem("event", 13, -10000)}}},
		{"negative balance", domain.Plan{StartBalance: -1200000, HorizonMonths: 24, AnnualReturnPct: 12, Items: []domain.PlanItem{growItem("out", -1000, 5)}}},
		{"zero horizon", domain.Plan{StartBalance: 123, HorizonMonths: 0}},
		{"negative horizon", domain.Plan{StartBalance: 123, HorizonMonths: -1}},
		{"one time growth unused", domain.Plan{HorizonMonths: 24, Items: []domain.PlanItem{{Kind: domain.PlanItemOneTime, Month: 2, Amount: 50, AnnualGrowthPct: math.NaN()}}}},
		{"half rounding", domain.Plan{StartBalance: 50, HorizonMonths: 13, AnnualReturnPct: 12, Items: []domain.PlanItem{growItem("half", 5, 10)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			curve, err := ProjectChecked(tc.plan)
			want := frozenPlanProject(tc.plan)
			if err != nil || !reflect.DeepEqual(curve, want) || !reflect.DeepEqual(curve, Project(tc.plan)) {
				t.Fatalf("curve %v want%v err%v", curve, want, err)
			}
			summary, err := EvaluateChecked(tc.plan)
			if err != nil {
				t.Fatal(err)
			}
			end, err := EndBalanceChecked(tc.plan)
			if err != nil || end != EndBalance(tc.plan) || summary.EndBalance != end {
				t.Fatalf("end %d %+v %v", end, summary, err)
			}
			monthly, err := MonthlyNetChecked(tc.plan)
			if err != nil || monthly != MonthlyNet(tc.plan) || summary.MonthlyNet != monthly {
				t.Fatalf("monthly %d %+v %v", monthly, summary, err)
			}
			months, depletes, err := RunwayMonthsChecked(tc.plan)
			oldMonths, oldDepletes := RunwayMonths(tc.plan)
			if err != nil || months != oldMonths || depletes != oldDepletes || summary.RunwayMonths != months || summary.Depletes != depletes {
				t.Fatalf("runway %v %v %v", months, depletes, err)
			}
		})
	}
}
func TestCheckedPlanIntegerOverflowIsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan domain.Plan
	}{
		{"max plus one", domain.Plan{StartBalance: math.MaxInt64, HorizonMonths: 1, Items: []domain.PlanItem{recItem("in", 1)}}},
		{"min minus one", domain.Plan{StartBalance: math.MinInt64, HorizonMonths: 1, Items: []domain.PlanItem{oneItem("out", 1, -1)}}},
		{"monthly cancellation overflow", domain.Plan{HorizonMonths: 1, Items: []domain.PlanItem{recItem("max", math.MaxInt64), recItem("one", 1), recItem("cancel", -1)}}},
		{"compounding flow order", domain.Plan{StartBalance: math.MaxInt64, HorizonMonths: 1, AnnualReturnPct: -1e-20, Items: []domain.PlanItem{recItem("first", 1), recItem("later", -1)}}},
		{"return balance overflow", domain.Plan{StartBalance: math.MaxInt64, HorizonMonths: 1, AnnualReturnPct: 12}},
		{"return conversion overflow", domain.Plan{StartBalance: math.MaxInt64, HorizonMonths: 1, AnnualReturnPct: math.MaxFloat64}},
		{"growth conversion overflow", domain.Plan{HorizonMonths: 13, Items: []domain.PlanItem{growItem("growth", 1, math.MaxFloat64)}}},
		{"growth factor overflow", domain.Plan{HorizonMonths: 25, Items: []domain.PlanItem{growItem("growth", 1, math.MaxFloat64)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			curve, err := ProjectChecked(tc.plan)
			if curve != nil || !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow curve %v: %v", curve, err)
			}
			end, err := EndBalanceChecked(tc.plan)
			if end != 0 || !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow end%d: %v", end, err)
			}
			months, depletes, err := RunwayMonthsChecked(tc.plan)
			if months != 0 || depletes || !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow runway %v %v %v", months, depletes, err)
			}
			summary, err := EvaluateChecked(tc.plan)
			if summary != (Summary{}) || !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow summary %+v %v", summary, err)
			}
		})
	}
}
func TestCheckedPlanRatesOnlyRejectWhenUsed(t *testing.T) {
	for _, rate := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, p := range []domain.Plan{{StartBalance: 1, HorizonMonths: 1, AnnualReturnPct: rate}, {HorizonMonths: 13, Items: []domain.PlanItem{growItem("rate", 1, rate)}}} {
			curve, err := ProjectChecked(p)
			if curve != nil || !errors.Is(err, ErrInvalidAssumption) {
				t.Fatalf("invalid rate used: %v %v", curve, err)
			}
		}
		for _, p := range []domain.Plan{{StartBalance: -1, HorizonMonths: 2, AnnualReturnPct: rate}, {HorizonMonths: 12, Items: []domain.PlanItem{growItem("no anniversary", 1, rate)}}, {HorizonMonths: 13, Items: []domain.PlanItem{growItem("zero", 0, rate)}}, {HorizonMonths: 2, Items: []domain.PlanItem{{Kind: domain.PlanItemOneTime, Amount: 1, Month: 1, AnnualGrowthPct: rate}}}} {
			if _, err := ProjectChecked(p); err != nil {
				t.Fatalf("unused rate blocked known amounts: %v", err)
			}
		}
	}
}
func TestCheckedPlanBoundsKindsAndZeroAreDistinct(t *testing.T) {
	for _, p := range []domain.Plan{{HorizonMonths: forecast.MaxProjectionMonths + 1}, {HorizonMonths: 1200, Items: make([]domain.PlanItem, 1000)}} {
		curve, err := ProjectChecked(p)
		if curve != nil || !errors.Is(err, forecast.ErrProjectionBounds) {
			t.Fatalf("bounds curve %v: %v", curve, err)
		}
	}
	p := domain.Plan{HorizonMonths: 2, Items: []domain.PlanItem{{Kind: "future", Amount: 1}}}
	if _, err := ProjectChecked(p); !errors.Is(err, ErrInvalidAssumption) {
		t.Fatalf("unknown kind: %v", err)
	}
	if got := Project(p); !reflect.DeepEqual(got, []int64{0, 0}) {
		t.Fatalf("legacy unknown behavior changed: %v", got)
	}
	summary, err := EvaluateChecked(domain.Plan{HorizonMonths: 2})
	if err != nil || summary != (Summary{Depletes: true}) {
		t.Fatalf("valid zero unavailable: %+v %v", summary, err)
	}
}
func TestCheckedCompoundingKeepsActualFlowOrder(t *testing.T) {
	p := domain.Plan{StartBalance: math.MinInt64 + 10, HorizonMonths: 1, AnnualReturnPct: 1, Items: []domain.PlanItem{recItem("max", math.MaxInt64), recItem("hundred", 100), recItem("offset", -math.MaxInt64)}}
	curve, err := ProjectChecked(p)
	if err != nil || !reflect.DeepEqual(curve, frozenPlanProject(p)) {
		t.Fatalf("compounding regrouped cash flows: %v %v", curve, err)
	}
	// The independently requested monthly sum is unavailable even though this
	// balance path fits; Evaluate must not expose a partially valid summary.
	if _, err := MonthlyNetChecked(p); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("monthly overflow hidden: %v", err)
	}
	if summary, err := EvaluateChecked(p); summary != (Summary{}) || !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("partial summary %+v %v", summary, err)
	}
}

func TestCheckedRoundedAmountsRejectUnrepresentableValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value float64
		want  int64
		fail  bool
	}{
		{"positive half", 0.5, 1, false}, {"negative half", -0.5, -1, false},
		{"upper excluded", 0x1p63, 0, true}, {"nearest below upper", math.Nextafter(0x1p63, 0), math.MaxInt64 - 1023, false},
		{"minimum included", -0x1p63, math.MinInt64, false}, {"below minimum", math.Nextafter(-0x1p63, math.Inf(-1)), 0, true},
		{"NaN", math.NaN(), 0, true}, {"infinity", math.Inf(1), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := roundedAmount(tc.value, true)
			if got != tc.want || errors.Is(err, money.ErrOverflow) != tc.fail {
				t.Fatalf("rounded %d %v", got, err)
			}
		})
	}
	if got, err := grownAmountUsing(growItem("identity", math.MaxInt64, 1e-20), 1, true); err != nil || got != math.MaxInt64 {
		t.Fatalf("identity factor damaged boundary: %d %v", got, err)
	}
	if _, err := grownAmountUsing(growItem("factor", 1, math.MaxFloat64), 2, true); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("infinite growth factor accepted: %v", err)
	}
}

func TestCheckedPlanParityAcrossRateAndAnniversaryCombinations(t *testing.T) {
	for _, rate := range []float64{-24, 0, 3.5, 8.1, 12} {
		for _, growth := range []float64{-25, 0, 2.5, 10} {
			for _, horizon := range []int{1, 12, 13, 24, 25, 36} {
				p := domain.Plan{StartBalance: 200000, HorizonMonths: horizon, AnnualReturnPct: rate, Items: []domain.PlanItem{growItem("out", -5001, growth), oneItem("boundary", 13, -19999), growItem("income", 10001, growth), oneItem("first", 1, 500)}}
				curve, err := ProjectChecked(p)
				if err != nil || !reflect.DeepEqual(curve, frozenPlanProject(p)) {
					t.Fatalf("rate%v growth%v months%d: %v %v", rate, growth, horizon, curve, err)
				}
			}
		}
	}
}
