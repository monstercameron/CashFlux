// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func mkGoalWithAlloc(id string, targetMinor, currentMinor int64, allocs ...domain.GoalAllocation) domain.Goal {
	return domain.Goal{
		ID:            id,
		TargetAmount:  money.New(targetMinor, "USD"),
		CurrentAmount: money.New(currentMinor, "USD"),
		Allocations:   allocs,
	}
}

func alloc(acct string, minor int64) domain.GoalAllocation {
	return domain.GoalAllocation{AccountID: acct, Amount: money.New(minor, "USD")}
}

func TestAllocatedAndCoverage(t *testing.T) {
	g := mkGoalWithAlloc("g1", 2_000_000, 500_00, alloc("a", 300_00), alloc("b", 200_00))
	if got := AllocatedTotal(g); got.Amount != 500_00 {
		t.Fatalf("AllocatedTotal = %d, want 50000", got.Amount)
	}
	if got := AllocatedTotal(g).Currency; got != "USD" {
		t.Fatalf("AllocatedTotal currency = %q, want USD", got)
	}
	// Coverage = committed 500.00 + earmarked 500.00 = 1000.00 of 20000.00 target = 5%.
	if got := CoverageMinor(g); got != 1_000_00 {
		t.Fatalf("CoverageMinor = %d, want 100000", got)
	}
	if got := CoveragePercent(g); got != 5 {
		t.Fatalf("CoveragePercent = %d, want 5", got)
	}
}

func TestCoveragePercentClampsAndGuardsTarget(t *testing.T) {
	// Over-covered clamps to 100.
	over := mkGoalWithAlloc("g", 100_00, 80_00, alloc("a", 50_00))
	if got := CoveragePercent(over); got != 100 {
		t.Fatalf("over CoveragePercent = %d, want 100", got)
	}
	// No positive target → 0 (no divide-by-zero).
	none := mkGoalWithAlloc("g", 0, 0)
	if got := CoveragePercent(none); got != 0 {
		t.Fatalf("zero-target CoveragePercent = %d, want 0", got)
	}
}

func TestAccountEarmarkedAndAvailable(t *testing.T) {
	g1 := mkGoalWithAlloc("g1", 1_000_00, 0, alloc("checking", 300_00), alloc("savings", 100_00))
	g2 := mkGoalWithAlloc("g2", 1_000_00, 0, alloc("checking", 150_00))
	goals := []domain.Goal{g1, g2}

	// Across both goals, "checking" has 450.00 earmarked.
	if got := AccountEarmarkedMinor(goals, "checking", ""); got != 450_00 {
		t.Fatalf("AccountEarmarkedMinor(checking) = %d, want 45000", got)
	}
	// Excluding g1, only g2's 150.00 counts.
	if got := AccountEarmarkedMinor(goals, "checking", "g1"); got != 150_00 {
		t.Fatalf("AccountEarmarkedMinor(checking, excl g1) = %d, want 15000", got)
	}
	// Available for g1 to earmark from a 1000.00 "checking" balance = 1000 − 150 (g2) = 850.
	if got := AvailableToEarmarkMinor(goals, "checking", 1_000_00, "g1"); got != 850_00 {
		t.Fatalf("AvailableToEarmarkMinor = %d, want 85000", got)
	}
	// Never negative: a tiny balance already over-earmarked by others yields 0.
	if got := AvailableToEarmarkMinor(goals, "checking", 100_00, "g1"); got != 0 {
		t.Fatalf("AvailableToEarmarkMinor (over) = %d, want 0", got)
	}
}

func TestEarmarkStatus(t *testing.T) {
	// No earmark → none.
	if got := EarmarkOf(mkGoalWithAlloc("g", 1_000_00, 200_00)); got != EarmarkNone {
		t.Fatalf("no-alloc EarmarkOf = %q, want none", got)
	}
	// Earmarked but committed+earmarked < target → partial.
	partial := mkGoalWithAlloc("g", 1_000_00, 200_00, alloc("a", 300_00)) // 200+300=500 < 1000
	if got := EarmarkOf(partial); got != EarmarkPartial {
		t.Fatalf("partial EarmarkOf = %q, want partial", got)
	}
	// Committed + earmarked ≥ target → full (money need not have moved).
	full := mkGoalWithAlloc("g", 1_000_00, 200_00, alloc("a", 800_00)) // 200+800=1000 ≥ 1000
	if got := EarmarkOf(full); got != EarmarkFull {
		t.Fatalf("full EarmarkOf = %q, want full", got)
	}
	// Non-financial (no target) is always none even with stray allocations.
	if got := EarmarkOf(mkGoalWithAlloc("g", 0, 0, alloc("a", 500_00))); got != EarmarkNone {
		t.Fatalf("no-target EarmarkOf = %q, want none", got)
	}
}

func TestReviewDue(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	// No cadence → never due.
	if ReviewDue(domain.Goal{}, now) {
		t.Fatal("no cadence should not be due")
	}
	// Cadence set but never reviewed → due immediately.
	if !ReviewDue(domain.Goal{ReviewCadence: domain.CadenceWeekly}, now) {
		t.Fatal("cadence with zero LastReviewedAt should be due")
	}
	// Weekly, reviewed 3 days ago → not due.
	fresh := domain.Goal{ReviewCadence: domain.CadenceWeekly, LastReviewedAt: now.AddDate(0, 0, -3)}
	if ReviewDue(fresh, now) {
		t.Fatal("weekly reviewed 3 days ago should not be due")
	}
	// Weekly, reviewed 10 days ago → due.
	stale := domain.Goal{ReviewCadence: domain.CadenceWeekly, LastReviewedAt: now.AddDate(0, 0, -10)}
	if !ReviewDue(stale, now) {
		t.Fatal("weekly reviewed 10 days ago should be due")
	}
	// Daily, reviewed 2 days ago → due.
	daily := domain.Goal{ReviewCadence: domain.CadenceDaily, LastReviewedAt: now.AddDate(0, 0, -2)}
	if !ReviewDue(daily, now) {
		t.Fatal("daily reviewed 2 days ago should be due")
	}
}

// The Cam scenario (2026-07-16): $32,000 emergency fund due Dec 31 2026, $0
// saved, $22,000 + $4,500 earmarked. The pace layer must amortize the COVERED
// gap ($5,500) over the remaining months — not re-ask for the whole target —
// and re-derive month to month as earmarks and the calendar move.
func TestPaceIsCoverageAwareAndReamortizes(t *testing.T) {
	g := mkGoalWithAlloc("em", 32_000_00, 0, alloc("sccu", 22_000_00), alloc("hysa", 4_500_00))
	g.TargetDate = mustDate("2026-12-31")
	from := mustDate("2026-07-16")

	rem, err := CoveredRemaining(g)
	if err != nil || rem.Amount != 5_500_00 {
		t.Fatalf("CoveredRemaining = %v (err %v), want $5,500", rem.Amount, err)
	}

	// Jul 16 → Dec 31: five whole months + the partial final month = 6.
	per, ok, err := MonthlyNeeded(g, from)
	if err != nil || !ok {
		t.Fatalf("MonthlyNeeded ok=%v err=%v", ok, err)
	}
	if want := int64((5_500_00 + 5) / 6); per.Amount != want {
		t.Errorf("MonthlyNeeded = %d, want %d (the $5,500 gap over 6 months, NOT target/6)", per.Amount, want)
	}

	// At that pace the goal is on track — the trajectory must not read "behind".
	onTrack, known, err := OnTrack(g, per, from)
	if err != nil || !known || !onTrack {
		t.Errorf("OnTrack = %v (known %v, err %v), want on track", onTrack, known, err)
	}

	// Month-to-month: set aside another $2,500 in August and the September ask
	// re-amortizes ($3,000 over 4 months), it doesn't stay stale.
	g.Allocations = append(g.Allocations, alloc("extra", 2_500_00))
	sep := mustDate("2026-09-10")
	per2, ok, err := MonthlyNeeded(g, sep)
	if err != nil || !ok {
		t.Fatalf("MonthlyNeeded (Sep) ok=%v err=%v", ok, err)
	}
	if want := int64((3_000_00 + 3) / 4); per2.Amount != want {
		t.Errorf("MonthlyNeeded after adjustment = %d, want %d ($3,000 over 4 months)", per2.Amount, want)
	}

	// Fully covered → complete-paced, nothing more asked.
	g.Allocations = append(g.Allocations, alloc("final", 3_000_00))
	if _, ok, _ := MonthlyNeeded(g, sep); ok {
		t.Error("MonthlyNeeded should report ok=false once coverage meets the target")
	}
	if p := ClassifyPace(g, sep); p != PaceComplete {
		t.Errorf("ClassifyPace = %v, want complete once fully covered", p)
	}
	if d, ok, err := Project(g, money.New(0, "USD"), sep); err != nil || !ok || !d.Equal(sep) {
		t.Errorf("Project on a covered goal = %v/%v/%v, want immediate", d, ok, err)
	}
}
