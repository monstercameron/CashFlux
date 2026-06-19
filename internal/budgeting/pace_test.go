package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// status builds a Status with the given spent/limit in USD (Remaining = limit − spent).
func status(spent, limit int64) Status {
	return Status{
		Spent:     money.New(spent, "USD"),
		Remaining: money.New(limit-spent, "USD"),
		Budget:    domain.Budget{Limit: money.New(limit, "USD")},
	}
}

func TestElapsedFraction(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 30)
	tests := []struct {
		name string
		now  time.Time
		want float64
	}{
		{"before start clamps to 0", start.AddDate(0, 0, -5), 0},
		{"at start", start, 0},
		{"halfway", start.AddDate(0, 0, 15), 0.5},
		{"after end clamps to 1", end.AddDate(0, 0, 5), 1},
		{"at end", end, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := elapsedFraction(start, end, tc.now)
			if diff := got - tc.want; diff < -0.001 || diff > 0.001 {
				t.Errorf("elapsedFraction = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestElapsedFractionDegenerate(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if got := elapsedFraction(now, now, now); got != 1 {
		t.Errorf("zero-span fraction = %v, want 1", got)
	}
}

func TestProjectPace(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 30)
	half := start.AddDate(0, 0, 15)

	t.Run("on track at half period", func(t *testing.T) {
		// Spent $300 of a $1000 limit halfway → projected $600, on track.
		p := ProjectPace(status(30000, 100000), start, end, half)
		if p.Projected.Amount != 60000 {
			t.Errorf("Projected = %d, want 60000", p.Projected.Amount)
		}
		if !p.OnTrack || !p.OverBy.IsZero() {
			t.Errorf("expected on track, got OnTrack=%v OverBy=%v", p.OnTrack, p.OverBy)
		}
	})

	t.Run("projected over at half period", func(t *testing.T) {
		// Spent $700 of $1000 halfway → projected $1400, over by $400.
		p := ProjectPace(status(70000, 100000), start, end, half)
		if p.Projected.Amount != 140000 {
			t.Errorf("Projected = %d, want 140000", p.Projected.Amount)
		}
		if p.OnTrack {
			t.Error("expected not on track")
		}
		if p.OverBy.Amount != 40000 {
			t.Errorf("OverBy = %d, want 40000", p.OverBy.Amount)
		}
	})

	t.Run("before any time elapsed cannot extrapolate", func(t *testing.T) {
		// At the start, projection = spend so far (no rate to extrapolate).
		p := ProjectPace(status(5000, 100000), start, end, start)
		if p.Projected.Amount != 5000 {
			t.Errorf("Projected = %d, want 5000 (spend so far)", p.Projected.Amount)
		}
		if !p.OnTrack {
			t.Error("5000 < 100000 limit → on track")
		}
	})

	t.Run("currency follows Spent", func(t *testing.T) {
		s := Status{Spent: money.New(100, "EUR"), Remaining: money.New(900, "EUR")}
		p := ProjectPace(s, start, end, half)
		if p.Projected.Currency != "EUR" || p.OverBy.Currency != "EUR" {
			t.Errorf("currency = %q/%q, want EUR", p.Projected.Currency, p.OverBy.Currency)
		}
	})

	t.Run("full period projects actual spend", func(t *testing.T) {
		p := ProjectPace(status(120000, 100000), start, end, end)
		if p.Projected.Amount != 120000 {
			t.Errorf("Projected = %d, want 120000", p.Projected.Amount)
		}
		if p.OverBy.Amount != 20000 {
			t.Errorf("OverBy = %d, want 20000", p.OverBy.Amount)
		}
	})
}
