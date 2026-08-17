package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNeeded(t *testing.T) {
	from := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		budget     domain.Budget
		remaining  money.Money
		linked     money.Money
		hasLinked  bool
		wantNeeded int64
		wantTarget int64
	}{
		{
			name:       "no target yields zero",
			budget:     domain.Budget{},
			remaining:  usd(5000),
			wantNeeded: 0,
			wantTarget: 0,
		},
		{
			name:       "refill up to with partial funding",
			budget:     domain.Budget{TargetKind: domain.TargetRefillUpTo, TargetAmount: usd(20000)},
			remaining:  usd(14000),
			wantNeeded: 6000,
			wantTarget: 20000,
		},
		{
			name:       "refill up to already met",
			budget:     domain.Budget{TargetKind: domain.TargetRefillUpTo, TargetAmount: usd(20000)},
			remaining:  usd(21000),
			wantNeeded: 0,
			wantTarget: 20000,
		},
		{
			name:       "refill up to with negative remaining funds full target",
			budget:     domain.Budget{TargetKind: domain.TargetRefillUpTo, TargetAmount: usd(20000)},
			remaining:  usd(-500),
			wantNeeded: 20000,
			wantTarget: 20000,
		},
		{
			name:       "set aside fixed regardless of balance",
			budget:     domain.Budget{TargetKind: domain.TargetSetAside, TargetAmount: usd(6000)},
			remaining:  usd(100000),
			wantNeeded: 6000,
			wantTarget: 6000,
		},
		{
			name:       "by date inline pace three months out",
			budget:     domain.Budget{TargetKind: domain.TargetByDate, TargetAmount: usd(30000), TargetDate: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)},
			remaining:  usd(0),
			wantNeeded: 10000, // 30000 over 3 months
			wantTarget: 30000,
		},
		{
			name:       "by date inline credits funded",
			budget:     domain.Budget{TargetKind: domain.TargetByDate, TargetAmount: usd(30000), TargetDate: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)},
			remaining:  usd(12000),
			wantNeeded: 6000, // (30000-12000)/3
			wantTarget: 30000,
		},
		{
			name:       "by date past deadline needs zero",
			budget:     domain.Budget{TargetKind: domain.TargetByDate, TargetAmount: usd(30000), TargetDate: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)},
			remaining:  usd(0),
			wantNeeded: 0,
			wantTarget: 30000,
		},
		{
			name:       "by date delegates to linked goal pace",
			budget:     domain.Budget{TargetKind: domain.TargetByDate, TargetAmount: usd(30000), TargetDate: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC), LinkedGoalID: "g1"},
			remaining:  usd(0),
			linked:     usd(4200),
			hasLinked:  true,
			wantNeeded: 4200, // goal pace used verbatim, not inline 10000
			wantTarget: 30000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := Status{Budget: tc.budget, Remaining: tc.remaining}
			got := Needed(tc.budget, status, from, tc.linked, tc.hasLinked)
			if got.Needed.Amount != tc.wantNeeded {
				t.Errorf("Needed = %d, want %d", got.Needed.Amount, tc.wantNeeded)
			}
			if got.Target.Amount != tc.wantTarget {
				t.Errorf("Target = %d, want %d", got.Target.Amount, tc.wantTarget)
			}
			if got.Needed.IsNegative() {
				t.Errorf("Needed must never be negative, got %d", got.Needed.Amount)
			}
		})
	}
}

// ─── WF17: snoozing a target ────────────────────────────────────────────────

func snoozeBudget(until time.Time) domain.Budget {
	return domain.Budget{
		ID: "b1", Period: domain.PeriodMonthly,
		Limit:              money.New(50_000, "USD"),
		TargetKind:         domain.TargetSetAside,
		TargetAmount:       money.New(30_000, "USD"),
		TargetSnoozedUntil: until,
	}
}

func TestASnoozedTargetDemandsNothingButIsStillThere(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	b := snoozeBudget(now.AddDate(0, 2, 0))
	st := Status{Remaining: money.New(0, "USD")}
	need := Needed(b, st, now, money.Zero("USD"), false)
	if !need.Snoozed {
		t.Fatal("a snoozed target must report itself as snoozed")
	}
	if need.Needed.Amount != 0 {
		t.Errorf("needed = %d, want 0 while paused", need.Needed.Amount)
	}
	// The LEVEL survives, or a snooze is indistinguishable from a deletion.
	if need.Target.Amount != 30_000 {
		t.Errorf("target = %d, want the level kept at 30000", need.Target.Amount)
	}
	if need.Kind != domain.TargetSetAside {
		t.Errorf("kind = %q, want the target kind preserved", need.Kind)
	}
}

func TestAnExpiredSnoozeResumesOnItsOwn(t *testing.T) {
	// Nothing has to be remembered: the target starts demanding funding again the
	// moment the date passes.
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	b := snoozeBudget(now.AddDate(0, -1, 0))
	need := Needed(b, Status{Remaining: money.New(0, "USD")}, now, money.Zero("USD"), false)
	if need.Snoozed {
		t.Error("a snooze in the past must not still be pausing the target")
	}
	if need.Needed.Amount != 30_000 {
		t.Errorf("needed = %d, want the full 30000 once the snooze expires", need.Needed.Amount)
	}
}

func TestSnoozeDoesNotHideThatATargetExists(t *testing.T) {
	// HasTarget stays true while paused — folding the snooze into it would make a
	// paused target vanish from every surface at once.
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	b := snoozeBudget(now.AddDate(0, 1, 0))
	if !b.HasTarget() {
		t.Error("a snoozed budget still HAS a target")
	}
	if b.TargetActive(now) {
		t.Error("a snoozed target is not active")
	}
	if !b.TargetSnoozed(now) {
		t.Error("TargetSnoozed must report the pause")
	}
}

func TestNoSnoozeSetIsNotSnoozed(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	b := snoozeBudget(time.Time{})
	if b.TargetSnoozed(now) {
		t.Error("a zero snooze date must not pause anything")
	}
	if !b.TargetActive(now) {
		t.Error("an un-snoozed target with a kind is active")
	}
}
