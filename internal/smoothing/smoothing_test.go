// SPDX-License-Identifier: MIT

package smoothing

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func rec(cadence domain.RecurringCadence, minor int64, next time.Time, smooth bool) domain.Recurring {
	return domain.Recurring{
		ID:                "r1",
		Label:             "Insurance",
		Amount:            money.New(minor, "USD"),
		Cadence:           cadence,
		NextDue:           next,
		SmoothIntoBudgets: smooth,
	}
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestMonthlyAccrual(t *testing.T) {
	tests := []struct {
		name string
		r    domain.Recurring
		want int64
	}{
		{"annual $600 -> $50/mo", rec(domain.CadenceYearly, -60000, date(2026, 6, 1), true), 5000},
		{"quarterly $300 -> $100/mo", rec(domain.CadenceQuarterly, -30000, date(2026, 6, 1), true), 10000},
		{"annual flag off -> 0", rec(domain.CadenceYearly, -60000, date(2026, 6, 1), false), 0},
		{"monthly with flag -> 0 (no off periods)", rec(domain.CadenceMonthly, -5000, date(2026, 6, 1), true), 0},
		{"weekly with flag -> 0", rec(domain.CadenceWeekly, -2000, date(2026, 6, 1), true), 0},
		{"positive amount magnitude", rec(domain.CadenceYearly, 60000, date(2026, 6, 1), true), 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MonthlyAccrual(tt.r); got != tt.want {
				t.Errorf("MonthlyAccrual = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSmooths(t *testing.T) {
	tests := []struct {
		cadence domain.RecurringCadence
		flag    bool
		want    bool
	}{
		{domain.CadenceYearly, true, true},
		{domain.CadenceQuarterly, true, true},
		{domain.CadenceMonthly, true, false},
		{domain.CadenceWeekly, true, false},
		{domain.CadenceYearly, false, false},
	}
	for _, tt := range tests {
		r := rec(tt.cadence, -60000, date(2026, 6, 1), tt.flag)
		if got := r.Smooths(); got != tt.want {
			t.Errorf("Smooths(%s,%v) = %v, want %v", tt.cadence, tt.flag, got, tt.want)
		}
	}
}

func TestOccurrencesIn(t *testing.T) {
	// Yearly bill due 2026-06-15.
	r := rec(domain.CadenceYearly, -60000, date(2026, 6, 15), true)

	tests := []struct {
		name       string
		start, end time.Time
		want       int
	}{
		{"landing month June", date(2026, 6, 1), date(2026, 7, 1), 1},
		{"off month July", date(2026, 7, 1), date(2026, 8, 1), 0},
		{"off month May (before NextDue, prior year)", date(2026, 5, 1), date(2026, 6, 1), 0},
		{"prior-year landing (June 2025 via backward walk)", date(2025, 6, 1), date(2025, 7, 1), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := len(OccurrencesIn(r, tt.start, tt.end))
			if got != tt.want {
				t.Errorf("OccurrencesIn = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestOccurrencesInMonthly(t *testing.T) {
	// A monthly bill due on the 5th; a full year window should yield 12.
	r := rec(domain.CadenceMonthly, -5000, date(2026, 6, 5), false)
	got := len(OccurrencesIn(r, date(2026, 1, 1), date(2027, 1, 1)))
	if got != 12 {
		t.Errorf("monthly occurrences in a year = %d, want 12", got)
	}
}

func TestLandsIn(t *testing.T) {
	r := rec(domain.CadenceYearly, -60000, date(2026, 6, 15), true)
	if !LandsIn(r, date(2026, 6, 1), date(2026, 7, 1)) {
		t.Error("expected June to be the landing month")
	}
	if LandsIn(r, date(2026, 7, 1), date(2026, 8, 1)) {
		t.Error("July is an off month, should not land")
	}
	// Non-smoothed recurring never "lands" for smoothing purposes.
	off := rec(domain.CadenceYearly, -60000, date(2026, 6, 15), false)
	if LandsIn(off, date(2026, 6, 1), date(2026, 7, 1)) {
		t.Error("non-smoothed recurring must not land")
	}
}

func TestSmoothingGoalHelpers(t *testing.T) {
	managed := domain.Goal{
		ID:     "g1",
		Name:   "Set aside for Insurance",
		Custom: map[string]any{GoalCustomKey: "r1"},
	}
	plain := domain.Goal{ID: "g2", Name: "Vacation"}

	if !IsSmoothingGoal(managed) {
		t.Error("managed goal should be recognised as a smoothing goal")
	}
	if IsSmoothingGoal(plain) {
		t.Error("plain goal should not be a smoothing goal")
	}
	if got := SmoothingRecurringID(managed); got != "r1" {
		t.Errorf("SmoothingRecurringID = %q, want r1", got)
	}
	goals := []domain.Goal{plain, managed}
	if g, ok := SmoothingGoalFor(goals, "r1"); !ok || g.ID != "g1" {
		t.Errorf("SmoothingGoalFor(r1) = %+v, %v", g, ok)
	}
	// Delete-dissolves scenario: once the goal is removed, the lookup fails.
	if _, ok := SmoothingGoalFor([]domain.Goal{plain}, "r1"); ok {
		t.Error("after dissolving, no smoothing goal should be found")
	}
}
