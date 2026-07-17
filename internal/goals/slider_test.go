// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func sliderGoal() domain.Goal {
	return domain.Goal{
		ID:            "g1",
		Name:          "Vacation",
		TargetAmount:  money.New(1200000, "USD"), // $12,000
		CurrentAmount: money.New(0, "USD"),
		TargetDate:    time.Date(2027, 8, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestSliderPointAt(t *testing.T) {
	g := sliderGoal()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// $500/mo → 24 payments starting this month (Jan '26..Dec '27) → Dec 2027.
	pt := SliderPointAt(g, 50000, from)
	if !pt.HasFinish {
		t.Fatal("expected a finish at $500/mo")
	}
	if pt.Finish.Year() != 2027 || pt.Finish.Month() != time.December {
		t.Fatalf("finish = %v, want Dec 2027", pt.Finish)
	}
	if pt.OnTrack {
		t.Fatal("Jan 2028 is after Aug 2027 target — should not be on track")
	}

	// $1000/mo → 12 months → Jan 2027, before the Aug 2027 target.
	fast := SliderPointAt(g, 100000, from)
	if !fast.OnTrack {
		t.Fatal("$1000/mo should be on track for Aug 2027")
	}

	// Zero contribution → no finish.
	if zero := SliderPointAt(g, 0, from); zero.HasFinish {
		t.Fatal("zero contribution should not project a finish")
	}
}

func TestSliderPointAtComplete(t *testing.T) {
	g := sliderGoal()
	g.CurrentAmount = money.New(1200000, "USD")
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pt := SliderPointAt(g, 0, from)
	if !pt.HasFinish {
		t.Fatal("a complete goal projects to `from`")
	}
}

func TestSliderTicks(t *testing.T) {
	g := sliderGoal()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pts := SliderTicks(g, []int64{50000, 100000}, from)
	if len(pts) != 2 {
		t.Fatalf("want 2 points, got %d", len(pts))
	}
	if !pts[0].HasFinish || !pts[1].HasFinish {
		t.Fatal("both ticks should project a finish")
	}
	// More per month finishes no later.
	if pts[1].Finish.After(pts[0].Finish) {
		t.Fatal("higher contribution should finish sooner or equal")
	}
}

func TestSliderRange(t *testing.T) {
	g := sliderGoal()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	min, max, step, ok := SliderRange(g, from)
	if !ok {
		t.Fatal("expected a range for an open goal")
	}
	if min.Amount <= 0 || max.Amount <= min.Amount || step.Amount <= 0 {
		t.Fatalf("bad range min=%d max=%d step=%d", min.Amount, max.Amount, step.Amount)
	}
	if max.Amount > g.TargetAmount.Amount {
		t.Fatalf("max %d should not exceed remaining %d", max.Amount, g.TargetAmount.Amount)
	}
	if min.Currency != "USD" || max.Currency != "USD" || step.Currency != "USD" {
		t.Fatal("range should be in goal currency")
	}
}

func TestSliderRangeComplete(t *testing.T) {
	g := sliderGoal()
	g.CurrentAmount = money.New(1200000, "USD")
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, _, _, ok := SliderRange(g, from); ok {
		t.Fatal("a complete goal has no remaining balance to size a range")
	}
}
