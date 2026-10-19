// SPDX-License-Identifier: MIT

package monthclose

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBCS2CopyNeverReplaysMovesOrOpaqueLegacyBoosts(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	periods := func(domain.Budget) (time.Time, time.Time) { return start, start.AddDate(0, 1, 0) }
	from, _ := (domain.Budget{ID: "from"}).WithPeriodMoveChecked(start, -5000)
	to, _ := (domain.Budget{ID: "to"}).WithPeriodMoveChecked(start, 5000)
	to, _ = to.WithPeriodBoostChecked(start, 700)
	legacy := domain.Budget{ID: "legacy", PeriodBoosts: map[string]int64{"2026-10-01": 5000}}
	release, _ := (domain.Budget{ID: "release"}).WithPeriodBoostChecked(start, -1000)
	for _, excluded := range []map[string]bool{nil, {"from": true}, {"to": true}, {"legacy": true}} {
		want := map[string]int64{"to": 700}
		if excluded["to"] {
			delete(want, "to")
		}
		got := CopyBoosts([]domain.Budget{from, to, legacy, release}, periods, excluded)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("exclude=%v plan=%v want=%v", excluded, got, want)
		}
	}
	writes, err := ApplyCopyBoosts([]domain.Budget{from, to, legacy}, periods, nil)
	if err != nil || len(writes) != 1 || writes[0].PeriodBoost(start.AddDate(0, 1, 0)) != 700 {
		t.Fatalf("writes=%+v err=%v", writes, err)
	}
	if again := CopyBoosts(writes, periods, nil); len(again) != 0 {
		t.Fatal("copy stacked")
	}
	if got := legacy.PeriodBoost(start); got != 5000 {
		t.Fatal("legacy amount changed")
	}
	// A prior top-up can be exactly offset by a move in the target period.
	// Its aggregate being zero must not make the copy button stack funding.
	target := start.AddDate(0, 1, 0)
	to, _ = to.WithPeriodBoostChecked(target, 700)
	to, _ = to.WithPeriodMoveChecked(target, -700)
	if plan := CopyBoosts([]domain.Budget{to}, periods, nil); len(plan) != 0 {
		t.Fatal("net-zero target allowed funding to be copied twice")
	}
}

func TestBCS2MonthCloseOverflowAndSavingTargets(t *testing.T) {
	for _, amounts := range [][]int64{{math.MinInt64}, {math.MaxInt64, 1}, {-math.MaxInt64, -1}} {
		var statuses []budgeting.Status
		for _, amount := range amounts {
			statuses = append(statuses, budgeting.Status{Remaining: money.New(amount, "USD")})
		}
		if summary := Build(statuses, nil, 0, 0, 0, false); summary.Error == nil || summary.Clean() {
			t.Fatalf("overflow hidden: %+v", summary)
		}
	}
	s := Build([]budgeting.Status{{Budget: domain.Budget{Direction: domain.DirectionSave}, Remaining: money.New(-40000, "USD")}}, nil, 0, 0, 0, false)
	if len(s.Overspends) != 0 {
		t.Fatal("saving shortfall offered as spending cover")
	}
	if _, err := (Summary{ExpectedIncomeMinor: math.MinInt64, ActualIncomeMinor: 1}).IncomeDeltaChecked(); err == nil {
		t.Fatal("income difference overflow hidden")
	}
}
