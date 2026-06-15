package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func goal(target, current int64) domain.Goal {
	return domain.Goal{TargetAmount: usd(target), CurrentAmount: usd(current)}
}

func TestRemaining(t *testing.T) {
	rem, err := Remaining(goal(100000, 30000))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !rem.Equal(usd(70000)) {
		t.Errorf("Remaining = %v, want 70000 USD", rem)
	}
	over, _ := Remaining(goal(100000, 120000))
	if !over.Equal(usd(0)) {
		t.Errorf("Remaining(over) = %v, want 0", over)
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		target, current int64
		want            int
	}{
		{100000, 30000, 30},
		{100000, 0, 0},
		{100000, 120000, 100}, // clamped
		{0, 5000, 100},        // zero target with savings -> complete
		{0, 0, 0},
	}
	for _, tt := range tests {
		if got := Percent(goal(tt.target, tt.current)); got != tt.want {
			t.Errorf("Percent(%d,%d) = %d, want %d", tt.target, tt.current, got, tt.want)
		}
	}
}

func TestIsComplete(t *testing.T) {
	if c, _ := IsComplete(goal(100000, 100000)); !c {
		t.Error("exactly met should be complete")
	}
	if c, _ := IsComplete(goal(100000, 99999)); c {
		t.Error("under target should not be complete")
	}
}

func TestProject(t *testing.T) {
	from := mustDate("2026-06-15")

	// remaining 60000, monthly 20000 -> 3 months -> 2026-09-15
	date, ok, err := Project(goal(100000, 40000), usd(20000), from)
	if err != nil || !ok {
		t.Fatalf("Project ok=%v err=%v", ok, err)
	}
	if dateutil.FormatDate(date) != "2026-09-15" {
		t.Errorf("projected = %s, want 2026-09-15", dateutil.FormatDate(date))
	}

	// remaining 65000, monthly 20000 -> ceil(3.25) = 4 months
	date2, _, _ := Project(goal(100000, 35000), usd(20000), from)
	if dateutil.FormatDate(date2) != "2026-10-15" {
		t.Errorf("projected (ceil) = %s, want 2026-10-15", dateutil.FormatDate(date2))
	}

	// already complete -> from, ok
	dc, okc, _ := Project(goal(100000, 100000), usd(20000), from)
	if !okc || !dc.Equal(from) {
		t.Errorf("complete projection = %v ok=%v, want from/true", dc, okc)
	}

	// non-positive contribution -> no projection
	if _, ok, _ := Project(goal(100000, 0), usd(0), from); ok {
		t.Error("zero contribution should yield no projection")
	}

	// currency mismatch -> error
	if _, _, err := Project(goal(100000, 0), money.New(20000, "EUR"), from); err == nil {
		t.Error("expected currency mismatch error")
	}
}

func TestEvaluate(t *testing.T) {
	from := mustDate("2026-06-15")
	s, err := Evaluate(goal(100000, 40000), usd(20000), from)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if s.Percent != 40 || !s.Remaining.Equal(usd(60000)) || s.Complete {
		t.Errorf("status = %+v", s)
	}
	if !s.HasProjection || dateutil.FormatDate(s.Projected) != "2026-09-15" {
		t.Errorf("projection = %v has=%v", s.Projected, s.HasProjection)
	}
}
