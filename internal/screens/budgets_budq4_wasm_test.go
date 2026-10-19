// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestBUDQ4PayCycleAnchorParser(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "valid", text: "2026-10-03", want: "2026-10-03"},
		{name: "empty"},
		{name: "invalid", text: "2026-02-30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := budgetPayCycleAnchor(tt.text)
			if tt.want == "" {
				if !got.IsZero() {
					t.Fatalf("anchor = %s, want zero", got)
				}
				return
			}
			if got.Format("2006-01-02") != tt.want {
				t.Fatalf("anchor = %s, want %s", got.Format("2006-01-02"), tt.want)
			}
		})
	}
}

func TestBUDQ4MajorAmountRejectsNonFiniteAndHugeValues(t *testing.T) {
	for _, input := range []string{"NaN", "+Inf", "-Inf", "1e30", "999999999999999999999999999999"} {
		if _, ok := majorStrToMinor(input, "USD"); ok {
			t.Errorf("majorStrToMinor(%q) accepted invalid or overflowing money", input)
		}
	}
	if got, ok := majorStrToMinor("12.34", "USD"); !ok || got != 1234 {
		t.Fatalf("majorStrToMinor(12.34) = %d, %t; want 1234, true", got, ok)
	}
}

func TestBUDQ4NameSortUsesDisplayedTitle(t *testing.T) {
	statuses := []budgeting.Status{
		{Budget: domain.Budget{ID: "named", Name: "Zebra", CategoryID: "a"}},
		{Budget: domain.Budget{ID: "unnamed", CategoryID: "b"}},
	}
	sortBudgetStatuses(statuses, uistate.BudgetSortName, map[string]string{"a": "Alpha", "b": "Bravo"})
	if statuses[0].Budget.ID != "unnamed" {
		t.Fatalf("first sorted budget = %q, want unnamed budget's displayed title Bravo before Zebra", statuses[0].Budget.ID)
	}
}

func TestBUDQ4PeriodKeyHonorsWeekStartPreference(t *testing.T) {
	ref := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC)
	got := budgeting.PeriodKeyWithWeekStart(domain.PeriodWeekly, ref, time.Time{}, time.Sunday)
	if got != "2026-10-18" {
		t.Fatalf("Sunday week key = %q, want 2026-10-18", got)
	}
	if got = budgeting.PeriodKeyWithWeekStart(domain.PeriodWeekly, ref, time.Time{}, time.Monday); got != "2026-10-12" {
		t.Fatalf("Monday week key = %q, want 2026-10-12", got)
	}
}
