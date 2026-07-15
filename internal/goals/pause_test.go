// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestComputePauseCostProjectionShift(t *testing.T) {
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	// $100 target, $0 saved, $50/mo → 2 months → finish 2026-03-15. Pause 2 months
	// pushes the finish to 2026-05-15.
	g := domain.Goal{
		TargetAmount:  money.New(10000, "USD"),
		CurrentAmount: money.New(0, "USD"),
	}
	cost, err := ComputePauseCost(g, money.New(5000, "USD"), from, 2)
	if err != nil {
		t.Fatalf("ComputePauseCost: %v", err)
	}
	if !cost.HasFinish {
		t.Fatal("want a datable finish")
	}
	wantOrig := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	wantShift := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	if !cost.Original.Equal(wantOrig) {
		t.Errorf("Original = %v, want %v", cost.Original, wantOrig)
	}
	if !cost.Shifted.Equal(wantShift) {
		t.Errorf("Shifted = %v, want %v", cost.Shifted, wantShift)
	}
	if cost.Months != 2 {
		t.Errorf("Months = %d, want 2", cost.Months)
	}
}

func TestComputePauseCostFallsBackToTargetDate(t *testing.T) {
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	target := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	// No monthly rate → no projection → falls back to the target date shift.
	g := domain.Goal{
		TargetAmount:  money.New(10000, "USD"),
		CurrentAmount: money.New(0, "USD"),
		TargetDate:    target,
	}
	cost, err := ComputePauseCost(g, money.New(0, "USD"), from, 3)
	if err != nil {
		t.Fatalf("ComputePauseCost: %v", err)
	}
	if !cost.HasFinish || !cost.Original.Equal(target) {
		t.Errorf("Original = %v (has=%v), want target %v", cost.Original, cost.HasFinish, target)
	}
	want := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	if !cost.Shifted.Equal(want) {
		t.Errorf("Shifted = %v, want %v", cost.Shifted, want)
	}
}

func TestComputePauseCostNoFinishWhenUndatable(t *testing.T) {
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	// No monthly rate and no target date → nothing datable to preview.
	g := domain.Goal{
		TargetAmount:  money.New(10000, "USD"),
		CurrentAmount: money.New(0, "USD"),
	}
	cost, err := ComputePauseCost(g, money.New(0, "USD"), from, 2)
	if err != nil {
		t.Fatalf("ComputePauseCost: %v", err)
	}
	if cost.HasFinish {
		t.Errorf("want no datable finish, got %+v", cost)
	}
}

func TestPausedUntilFrom(t *testing.T) {
	from := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	if got := PausedUntilFrom(from, 0); !got.IsZero() {
		t.Errorf("zero months should give zero time, got %v", got)
	}
	got := PausedUntilFrom(from, 2)
	want := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("PausedUntilFrom = %v, want %v", got, want)
	}
}

func TestClassifyPacePausedNotScolding(t *testing.T) {
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	// An overdue goal that is paused reads as PacePaused, not PaceOverdue.
	g := domain.Goal{
		TargetAmount:  money.New(10000, "USD"),
		CurrentAmount: money.New(1000, "USD"),
		TargetDate:    time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), // in the past
		PausedUntil:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),  // paused
	}
	if p := ClassifyPace(g, from); p != PacePaused {
		t.Errorf("paused overdue goal pace = %q, want %q", p, PacePaused)
	}
	// After the pause ends, it scolds normally again.
	after := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if p := ClassifyPace(g, after); p != PaceOverdue {
		t.Errorf("post-pause pace = %q, want %q", p, PaceOverdue)
	}
}

func TestOnTrackPausedIsNotBehind(t *testing.T) {
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	g := domain.Goal{
		TargetAmount:  money.New(10000, "USD"),
		CurrentAmount: money.New(0, "USD"),
		TargetDate:    time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), // very soon, unreachable at $0/mo
		PausedUntil:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	onTrack, known, err := OnTrack(g, money.New(0, "USD"), from)
	if err != nil {
		t.Fatalf("OnTrack: %v", err)
	}
	if !known || !onTrack {
		t.Errorf("paused goal should read on-track (known=%v onTrack=%v)", known, onTrack)
	}
}
