// SPDX-License-Identifier: MIT

package monthclose

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestCarryTargetsHonorsBiweeklyPayAnchor(t *testing.T) {
	payday := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	ref := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	source, target := CarryTargetsAnchored(domain.PeriodBiweekly, ref, time.Monday, payday)
	if want := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC); !source.Equal(want) {
		t.Fatalf("source = %s, want %s", source.Format(time.DateOnly), want.Format(time.DateOnly))
	}
	if want := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC); !target.Equal(want) {
		t.Fatalf("target = %s, want %s", target.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}

func TestCopyBoostsBecomesIdempotentAfterTargetIsWritten(t *testing.T) {
	ref := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	current := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	next := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "dining", Period: domain.PeriodMonthly}.WithPeriodBoost(current, 5_000)
	starts := func(b domain.Budget) (time.Time, time.Time) {
		return CarryTargets(b.Period, ref, time.Sunday)
	}
	first := CopyBoosts([]domain.Budget{budget}, starts, nil)
	if first[budget.ID] != 5_000 {
		t.Fatalf("first copy plan = %v, want dining:5000", first)
	}
	written := budget.WithPeriodBoost(next, first[budget.ID])
	second := CopyBoosts([]domain.Budget{written}, starts, nil)
	if len(second) != 0 {
		t.Fatalf("repeat copy plan = %v, want no writes", second)
	}
}

func TestApplyCopyBoostsWritesReturnedTargetPeriod(t *testing.T) {
	ref := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	current := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	next := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "dining", Period: domain.PeriodMonthly}.WithPeriodBoost(current, 5_000)
	starts := func(b domain.Budget) (time.Time, time.Time) {
		return CarryTargets(b.Period, ref, time.Sunday)
	}
	writes, err := ApplyCopyBoosts([]domain.Budget{budget}, starts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || writes[0].PeriodBoost(next) != 5_000 {
		t.Fatalf("writes = %+v, want next-period boost 5000", writes)
	}
	if writes[0].PeriodBoost(current) != 5_000 {
		t.Fatalf("source boost changed: %+v", writes[0].PeriodBoosts)
	}
	repeated, err := ApplyCopyBoosts(writes, starts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(repeated) != 0 {
		t.Fatalf("repeated apply produced writes: %+v", repeated)
	}
}
