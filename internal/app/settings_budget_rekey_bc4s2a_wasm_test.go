// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
)

func TestBudgetPeriodPreferenceChangeRekeysAllStoredPeriodData(t *testing.T) {
	oldPrefs := prefs.Prefs{WeekStart: prefs.WeekMonday}
	nextPrefs := prefs.Prefs{WeekStart: prefs.WeekSunday}
	budget := domain.Budget{
		ID: "groceries", Period: domain.PeriodWeekly,
		PeriodBoosts:  map[string]int64{"2026-10-12": 500},
		LimitHistory:  map[string]int64{"2026-10-12": 2_500},
		PeriodNotes:   map[string]string{"2026-10-12": "Guests came over"},
		RolloverStart: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
	}
	writes, err := rekeyBudgetPeriodData([]domain.Budget{budget}, oldPrefs, nextPrefs)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(writes))
	}
	got := writes[0]
	if got.PeriodBoosts["2026-10-11"] != 500 || got.LimitHistory["2026-10-11"] != 2_500 || got.PeriodNotes["2026-10-11"] != "Guests came over" {
		t.Fatalf("rekeyed period maps = boosts %v, history %v, notes %v", got.PeriodBoosts, got.LimitHistory, got.PeriodNotes)
	}
	if want := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC); !got.RolloverStart.Equal(want) {
		t.Fatalf("rollover start = %s, want %s", got.RolloverStart.Format(time.DateOnly), want.Format(time.DateOnly))
	}
	if budget.PeriodBoosts["2026-10-12"] != 500 {
		t.Fatal("input budget was mutated")
	}
}

func TestPayAnchorChangeRekeysBiweeklyBoost(t *testing.T) {
	oldPrefs := prefs.Prefs{WeekStart: prefs.WeekMonday, PayCycleAnchor: "2026-10-09"}
	nextPrefs := prefs.Prefs{WeekStart: prefs.WeekMonday, PayCycleAnchor: "2026-10-16"}
	budget := domain.Budget{ID: "bills", Period: domain.PeriodBiweekly, PeriodBoosts: map[string]int64{"2026-10-09": -700}}
	writes, err := rekeyBudgetPeriodData([]domain.Budget{budget}, oldPrefs, nextPrefs)
	if err != nil {
		t.Fatal(err)
	}
	if got := writes[0].PeriodBoosts["2026-10-16"]; got != -700 {
		t.Fatalf("rekeyed biweekly boost = %v, want 2026-10-16: -700", writes[0].PeriodBoosts)
	}
}

func TestChangingBudgetPeriodPrefsPersistsRekeyedBudget(t *testing.T) {
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Store().Close()
	if err := a.SetSettingKV("cashflux:prefs", `{"weekStart":"monday"}`); err != nil {
		t.Fatal(err)
	}
	if err := a.PutCategory(domain.Category{ID: "groceries", Name: "Groceries", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{
		ID: "groceries-budget", Name: "Groceries", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		CategoryID: "groceries", Period: domain.PeriodWeekly, Limit: money.New(20_000, "USD"),
		PeriodBoosts:  map[string]int64{"2026-10-12": 500},
		PeriodNotes:   map[string]string{"2026-10-12": "Guests came over"},
		LimitHistory:  map[string]int64{"2026-10-12": 20_000},
		RolloverStart: start,
	}
	if err := a.PutBudgetAt(budget, start); err != nil {
		t.Fatal(err)
	}
	seeded := a.Budgets()[0]
	seeded.RolloverStart = start
	if err := a.PutBudgetAt(seeded, start); err != nil {
		t.Fatal(err)
	}
	oldPrefs := prefs.Prefs{WeekStart: prefs.WeekMonday}
	nextPrefs := prefs.Prefs{WeekStart: prefs.WeekSunday}
	if err := rekeyBudgetPeriodPreferences(a, oldPrefs, nextPrefs, start); err != nil {
		t.Fatalf("save re-keyed preferences: %v", err)
	}
	got := a.Budgets()[0]
	if got.PeriodBoosts["2026-10-11"] != 500 || got.PeriodNotes["2026-10-11"] != "Guests came over" || got.LimitHistory["2026-10-11"] != 20_000 {
		t.Fatalf("persisted period data: boosts=%v notes=%v history=%v", got.PeriodBoosts, got.PeriodNotes, got.LimitHistory)
	}
	if want := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC); !got.RolloverStart.Equal(want) {
		t.Fatalf("persisted rollover start = %s, want %s", got.RolloverStart.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}
