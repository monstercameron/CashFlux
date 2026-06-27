// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// sinkGoal is a convenience constructor for sinking-fund tests.
func sinkGoal(current, target int64, targetDate time.Time) domain.Goal {
	return domain.Goal{
		TargetAmount:  money.New(target, "USD"),
		CurrentAmount: money.New(current, "USD"),
		TargetDate:    targetDate,
	}
}

// ─── DrawDownFund ────────────────────────────────────────────────────────────

func TestDrawDownFund(t *testing.T) {
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	far := now.AddDate(0, 6, 0)

	tests := []struct {
		name       string
		current    int64
		target     int64
		spend      int64
		wantAmount int64
		wantErr    bool
	}{
		{
			name:       "normal reduce",
			current:    50000,
			target:     100000,
			spend:      20000,
			wantAmount: 30000,
		},
		{
			name:       "draw-down to zero exactly",
			current:    20000,
			target:     100000,
			spend:      20000,
			wantAmount: 0,
		},
		{
			name:       "draw-down past zero clamps to 0",
			current:    10000,
			target:     100000,
			spend:      99999,
			wantAmount: 0,
		},
		{
			name:       "zero spend leaves fund unchanged",
			current:    50000,
			target:     100000,
			spend:      0,
			wantAmount: 50000,
		},
		{
			name:    "negative spend returns error",
			current: 50000,
			target:  100000,
			spend:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := sinkGoal(tt.current, tt.target, far)
			got, err := DrawDownFund(g, tt.spend)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.CurrentAmount.Amount != tt.wantAmount {
				t.Errorf("CurrentAmount = %d, want %d", got.CurrentAmount.Amount, tt.wantAmount)
			}
			if got.CurrentAmount.Currency != "USD" {
				t.Errorf("currency = %q, want USD", got.CurrentAmount.Currency)
			}
			// Input must not be mutated.
			if g.CurrentAmount.Amount != tt.current {
				t.Errorf("input Goal was mutated: CurrentAmount = %d, want %d", g.CurrentAmount.Amount, tt.current)
			}
		})
	}
}

func TestDrawDownFundDoesNotMutateInput(t *testing.T) {
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	g := sinkGoal(50000, 100000, now.AddDate(0, 6, 0))
	original := g.CurrentAmount.Amount

	_, err := DrawDownFund(g, 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.CurrentAmount.Amount != original {
		t.Errorf("input mutated: CurrentAmount = %d, want %d", g.CurrentAmount.Amount, original)
	}
}

// ─── FundSetAsideMinor ───────────────────────────────────────────────────────

func TestFundSetAsideMinor(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		current    int64
		target     int64
		targetDate time.Time
		wantMinor  int64
	}{
		{
			// $1200 needed, 12 whole months (Jan 1 → Jan 1 next year, same day-of-month).
			// months = (2027-2026)*12 + 1 - 1 = 12; day 1 == day 1, no extra month.
			// SinkingFundContribution(120000, 12) = ceil(120000/12) = 10000.
			name:       "1200 over 12 months → 100/mo",
			current:    0,
			target:     120000,
			targetDate: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantMinor:  10000,
		},
		{
			// $100 needed, 3 months (Jan 1 → Apr 1, same day-of-month).
			// months = 3; ceil(10000/3) = 3334.
			name:       "100 over 3 months → ceil",
			current:    0,
			target:     10000,
			targetDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			wantMinor:  3334,
		},
		{
			// Partial month: now=Jan 1, target=Jan 15 of same year. Day 15 > day 1 → months++.
			// months = (2026-2026)*12 + 1 - 1 = 0, then +1 (day 15 > day 1) → 1; min 1.
			// SinkingFundContribution(50000, 1) = 50000.
			name:       "partial month rounds up to 1",
			current:    0,
			target:     50000,
			targetDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			wantMinor:  50000,
		},
		{
			// Partially funded: target=120000, current=60000, remaining=60000.
			// 12 months → ceil(60000/12) = 5000.
			name:       "partially funded reduces set-aside",
			current:    60000,
			target:     120000,
			targetDate: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantMinor:  5000,
		},
		{
			// Already fully funded → 0.
			name:       "fully funded → 0",
			current:    120000,
			target:     120000,
			targetDate: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantMinor:  0,
		},
		{
			// No target date → 0.
			name:       "no target date → 0",
			current:    0,
			target:     120000,
			targetDate: time.Time{},
			wantMinor:  0,
		},
		{
			// Target in the past → 0.
			name:       "past deadline → 0",
			current:    0,
			target:     120000,
			targetDate: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			wantMinor:  0,
		},
		{
			// Target is exactly now (not after now) → 0.
			name:       "target is now → 0",
			current:    0,
			target:     120000,
			targetDate: now,
			wantMinor:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := sinkGoal(tt.current, tt.target, tt.targetDate)
			got := FundSetAsideMinor(g, now)
			if got != tt.wantMinor {
				t.Errorf("FundSetAsideMinor = %d, want %d", got, tt.wantMinor)
			}
		})
	}
}

// ─── FundAccrualDue ──────────────────────────────────────────────────────────

func TestFundAccrualDue(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	far := now.AddDate(0, 6, 0) // deadline 6 months out — set-aside will be > 0

	// sinkFundGoal builds a sinking-fund goal with optional Custom fields.
	sinkFundGoal := func(current, target int64, targetDate time.Time, custom map[string]any) domain.Goal {
		g := domain.Goal{
			IsSinkingFund: true,
			TargetAmount:  money.New(target, "USD"),
			CurrentAmount: money.New(current, "USD"),
			TargetDate:    targetDate,
			Custom:        custom,
		}
		return g
	}

	t.Run("not a sinking fund → not due", func(t *testing.T) {
		g := sinkGoal(0, 120000, far) // IsSinkingFund == false
		due, amt := FundAccrualDue(g, now)
		if due {
			t.Errorf("expected due=false for non-sinking-fund goal, got due=true amt=%d", amt)
		}
	})

	t.Run("archived → not due", func(t *testing.T) {
		g := sinkFundGoal(0, 120000, far, nil)
		g.Archived = true
		due, amt := FundAccrualDue(g, now)
		if due {
			t.Errorf("expected due=false for archived goal, got due=true amt=%d", amt)
		}
	})

	t.Run("already at target → not due", func(t *testing.T) {
		g := sinkFundGoal(120000, 120000, far, nil)
		due, amt := FundAccrualDue(g, now)
		if due {
			t.Errorf("expected due=false when fully funded, got due=true amt=%d", amt)
		}
	})

	t.Run("already accrued this month → not due", func(t *testing.T) {
		periodKey := now.UTC().Format("2006-01") // "2026-06"
		g := sinkFundGoal(0, 120000, far, map[string]any{"fundAccrualPeriod": periodKey})
		due, amt := FundAccrualDue(g, now)
		if due {
			t.Errorf("expected due=false when already accrued this month, got due=true amt=%d", amt)
		}
	})

	t.Run("no target date → not due (set-aside=0)", func(t *testing.T) {
		g := sinkFundGoal(0, 120000, time.Time{}, nil)
		due, amt := FundAccrualDue(g, now)
		if due {
			t.Errorf("expected due=false with no target date, got due=true amt=%d", amt)
		}
	})

	t.Run("fresh month, under target → due with correct amount", func(t *testing.T) {
		// $1200 goal, $0 saved, 6 months to deadline.
		// FundSetAsideMinor will be ceil(120000/6) = 20000.
		g := sinkFundGoal(0, 120000, far, nil)
		due, amt := FundAccrualDue(g, now)
		if !due {
			t.Fatal("expected due=true for fresh sinking fund, got false")
		}
		wantSetAside := FundSetAsideMinor(g, now)
		if amt != wantSetAside {
			t.Errorf("amountMinor = %d, want %d (FundSetAsideMinor)", amt, wantSetAside)
		}
		if amt <= 0 {
			t.Errorf("amountMinor must be > 0, got %d", amt)
		}
	})

	t.Run("accrual from prior month is not a block", func(t *testing.T) {
		// Last accrual was in May; June is a fresh month.
		g := sinkFundGoal(20000, 120000, far, map[string]any{"fundAccrualPeriod": "2026-05"})
		due, amt := FundAccrualDue(g, now)
		if !due {
			t.Fatal("expected due=true when prior month's marker doesn't match current month")
		}
		if amt <= 0 {
			t.Errorf("amountMinor must be > 0, got %d", amt)
		}
	})

	t.Run("set-aside capped at remaining to target", func(t *testing.T) {
		// Only $500 left to target; monthly set-aside would exceed that.
		// FundSetAsideMinor returns full monthly contribution; FundAccrualDue
		// caps at the remaining ($500 = 50000 minor units).
		remaining := int64(50000)
		target := int64(120000)
		current := target - remaining
		g := sinkFundGoal(current, target, far, nil)
		setAside := FundSetAsideMinor(g, now)
		due, amt := FundAccrualDue(g, now)
		if !due {
			t.Fatal("expected due=true")
		}
		if setAside > remaining {
			// Cap must have fired.
			if amt != remaining {
				t.Errorf("expected amountMinor capped at remaining=%d, got %d", remaining, amt)
			}
		} else {
			if amt != setAside {
				t.Errorf("expected amountMinor=%d (no cap needed), got %d", setAside, amt)
			}
		}
	})
}

// TestFundSetAsideMinorAgreesWithMonthlyNeeded verifies that FundSetAsideMinor
// and MonthlyNeeded return the same per-month figure for the same goal — they
// must use identical months arithmetic.
func TestFundSetAsideMinorAgreesWithMonthlyNeeded(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		current    int64
		target     int64
		targetDate time.Time
	}{
		{"12-month exact", 0, 120000, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"3-month ceil", 0, 10000, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
		{"partial-month", 0, 50000, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"partly funded", 60000, 120000, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			g := sinkGoal(tt.current, tt.target, tt.targetDate)

			fromSetAside := FundSetAsideMinor(g, now)

			mnResult, ok, err := MonthlyNeeded(g, now)
			if err != nil {
				t.Fatalf("MonthlyNeeded error: %v", err)
			}
			if !ok {
				t.Fatal("MonthlyNeeded returned ok=false; goal should have a valid projection")
			}

			if fromSetAside != mnResult.Amount {
				t.Errorf("FundSetAsideMinor=%d, MonthlyNeeded=%d — mismatch", fromSetAside, mnResult.Amount)
			}
		})
	}
}
