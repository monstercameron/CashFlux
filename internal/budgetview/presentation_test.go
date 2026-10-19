package budgetview

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/untracked"
	"math"
	"strings"
	"testing"
)

func TestTrackingPresentations(t *testing.T) {
	for _, tc := range []struct {
		cats, tags int
		disabled   bool
		part       string
	}{{0, 0, true, "categoryCount.many"}, {1, 0, false, "categoryCount.one"}, {2, 1, false, "tagCount.one"}} {
		got := PresentTrackingFooter(tc.cats, tc.tags, tr)
		if got.Disabled != tc.disabled || !strings.Contains(got.Count, tc.part) {
			t.Fatal(got)
		}
	}
	for _, on := range []bool{true, false} {
		for _, hint := range []bool{true, false} {
			r := TrackRow{Cand: untracked.Candidate{SpentMinor: 100, FromHint: hint}, Include: on, Amount: 123}
			if on {
				r.BudgetID = "b"
			}
			got := PresentTrackRow(r, "USD", tr)
			if got.Amount != "1.23" || got.ShowRaise != on || strings.Contains(got.Class, "is-off") == on || strings.Contains(got.Subtitle, "track.fromHistory") != hint {
				t.Fatal(got)
			}
		}
	}
	for _, method := range []budgeting.Methodology{budgeting.MethodSimple, budgeting.MethodZeroBased} {
		for _, risk := range []bool{false, true} {
			p := TrackPreview{Effect: untracked.Effect{Categories: 1, ToAssignBeforeMinor: 100, ToAssignAfterMinor: -50}}
			if risk {
				p.RiskNames = []string{"Food", "Travel"}
			}
			got := PresentTrackFooter(p, "USD", method, tr)
			if (got.ToAssign != "") != (method == budgeting.MethodZeroBased) || (got.Risk != "") != risk {
				t.Fatal(got)
			}
		}
	}
	if !ApplyDisabled(untracked.Effect{}) || ApplyDisabled(untracked.Effect{Categories: 1}) {
		t.Fatal("disabled")
	}
}

func TestAnnualPresentations(t *testing.T) {
	for _, on := range []bool{false, true} {
		class, key := AnnualDisclosure(on)
		if strings.Contains(class, "is-open") != on || (key == "budgets.planYearHideAria") != on {
			t.Fatal(class, key)
		}
	}
	for _, tc := range []struct {
		on    bool
		delta int64
		under int
		want  string
	}{{false, 0, 0, "+$0.00"}, {true, 123, 0, "+$1.23"}, {true, -123, 2, "\u2212$1.23"}, {true, math.MinInt64, 1, "\u2212$92233720368547758.08"}} {
		got := PresentAnnualScenario(tc.on, tc.delta, tc.under, "USD", tr)
		if got.Delta != tc.want || strings.Contains(got.Class, "is-on") != tc.on || strings.Contains(got.StatusClass, "is-under") != (tc.under > 0) {
			t.Fatal(got)
		}
	}
	if AnnualWindowClass("h1", "h1") != "budget-annualgrid-win is-on" || AnnualWindowClass("h1", "h2") != "budget-annualgrid-win" {
		t.Fatal("window tone")
	}
	grid := budgeting.AnnualGrid{Currency: "USD", CurrentMonth: 9}
	grid.MonthActualTotals[10] = money.New(123, "USD")
	for _, tc := range []struct {
		from      int
		projected int64
		want      bool
		amount    string
	}{{10, 0, false, "$1.23"}, {10, 300, true, "$3.00"}, {11, 300, false, "$1.23"}, {10, -300, false, "$1.23"}} {
		totals := budgetplan.MonthAmounts{}
		totals[10] = tc.projected
		got := PresentAnnualFooter(grid, totals, 10, tc.from)
		if got.Projected != tc.want || got.Amount != tc.amount {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct{ first, last, want int }{{0, 11, 13}, {0, 5, 7}, {6, 11, 7}} {
		if ErrorColumnSpan(tc.first, tc.last) != tc.want {
			t.Fatal(tc)
		}
	}
}
