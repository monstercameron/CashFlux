// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAssistantGoalCoachUsesCanonicalCoverageFreshFXAndTargetCurrency(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	oldNow := assistantGoalCoachNow
	assistantGoalCoachNow = func() time.Time { return now }
	t.Cleanup(func() { assistantGoalCoachNow = oldNow })
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	g := domain.Goal{ID: "coach-trip", Name: "Euro trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(10000, "EUR"), CurrentAmount: money.New(2000, "EUR"), MonthlyContribution: money.New(500, "USD"), TargetDate: now.AddDate(0, 2, 1), Allocations: []domain.GoalAllocation{{AccountID: "a-check", Amount: money.New(500, "USD")}}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	tools := agToolsCoach(app, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 999}})
	tool := assistantFinanceTestTool(t, tools, "check_goal_pace")
	raw := json.RawMessage(`{"goal":"Euro trip"}`)
	out := tool.run(raw)
	for _, want := range []string{"configured monthly plan", "2.50 EUR", "25.84 EUR/month", "23.34 EUR/month", "77.50 EUR", "not an observed funding pace"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing canonical fact %q: %s", want, out)
		}
	}
	if strings.Contains(out, "$") {
		t.Fatalf("goal values relabeled as captured USD: %s", out)
	}
	settings.FXRates = map[string]float64{"EUR": 4}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	out = tool.run(raw)
	for _, want := range []string{"1.25 EUR", "26.25 EUR/month", "25.00 EUR/month", "78.75 EUR"} {
		if !strings.Contains(out, want) {
			t.Fatalf("handler retained old FX (%q): %s", want, out)
		}
	}
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	out = tool.run(raw)
	if !strings.Contains(out, "coaching unavailable") || strings.Contains(out, "target covered") || strings.Contains(out, "meets the canonical requirement") {
		t.Fatalf("unknown FX became funding verdict: %s", out)
	}
	// A same-currency goal remains usable without an unrelated base FX rate.
	g.Allocations = nil
	g.MonthlyContribution = money.New(1000, "EUR")
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	out = tool.run(raw)
	if strings.Contains(out, "coaching unavailable") || !strings.Contains(out, "10.00 EUR") {
		t.Fatalf("native goal currency required unrelated base conversion: %s", out)
	}
	g.Allocations = []domain.GoalAllocation{{AccountID: "a-check", Amount: money.New(8000, "EUR")}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	out = tool.run(raw)
	for _, want := range []string{"target covered", "saved", "20.00 EUR", "set aside", "80.00 EUR", "100.00 EUR"} {
		if !strings.Contains(out, want) {
			t.Fatalf("reserved coverage missed %q: %s", want, out)
		}
	}
}

func TestAssistantGoalCoachReadsCanonicalContributionHistoryAndQuietTransitions(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	oldNow := assistantGoalCoachNow
	assistantGoalCoachNow = func() time.Time { return now }
	t.Cleanup(func() { assistantGoalCoachNow = oldNow })
	g := domain.Goal{ID: "coach-contribution", Name: "Manual saving", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(10000, "EUR"), CurrentAmount: money.New(2000, "EUR"), MonthlyContribution: money.Zero("EUR"), TargetDate: now.AddDate(0, 2, 1), Contributions: []domain.GoalContribution{{At: now.AddDate(0, -1, 0), Amount: money.New(1000, "EUR")}}, ContributionHistory: []domain.GoalMonthTotal{{Month: "2026-08", Amount: money.New(1000, "EUR"), Count: 1}}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	tool := assistantFinanceTestTool(t, agToolsCoach(app, "USD", currency.Rates{Base: "USD"}), "check_goal_pace")
	raw := json.RawMessage(`{"goal":"Manual saving"}`)
	before := tool.run(raw)
	for _, want := range []string{"no positive monthly contribution is configured", "80.00 EUR", "26.67 EUR/month", "not evidence of actual funding"} {
		if !strings.Contains(before, want) {
			t.Fatalf("history was doubled or used as a monthly plan (%q): %s", want, before)
		}
	}
	if _, err := app.ContributeToGoal(g, money.New(1000, "EUR"), false); err != nil {
		t.Fatal(err)
	}
	updated := assistantStoredGoal(t, app, g.ID)
	if updated.CurrentAmount.Amount != 3000 || len(updated.Contributions) != 2 {
		t.Fatalf("canonical contribution not stored: %+v", updated)
	}
	now = updated.LastReviewedAt.Add(14*24*time.Hour - time.Nanosecond)
	quiet := tool.run(raw)
	if !strings.Contains(quiet, "reviewed within fourteen days") || strings.Contains(quiet, "Manual saving:") {
		t.Fatalf("quiet period not honored: %s", quiet)
	}
	now = now.Add(time.Nanosecond)
	after := tool.run(raw)
	if strings.Contains(after, "reviewed within fourteen days") || !strings.Contains(after, "70.00 EUR") {
		t.Fatalf("contribution/history update or exact quiet cutoff stale: %s", after)
	}
	// A source failure is not an empty goal list or a successful quiet review.
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	out := tool.run(raw)
	if !strings.HasPrefix(out, "Goal coaching unavailable: read goal-coaching settings:") || strings.Contains(out, "No matching goals.") {
		t.Fatalf("closed store masked: %s", out)
	}
}

func TestAssistantGoalCoachDoesNotFundNonfinancialOrWrapExtremePlans(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	oldNow := assistantGoalCoachNow
	assistantGoalCoachNow = func() time.Time { return now }
	t.Cleanup(func() { assistantGoalCoachNow = oldNow })
	for _, g := range []domain.Goal{
		{ID: "coach-habit", Name: "Habit", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Kind: domain.GoalKindHabit, HabitTarget: 10, TargetDate: now.AddDate(0, 1, 0)},
		{ID: "coach-checklist", Name: "Checklist", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Kind: domain.GoalKindChecklist, TargetDate: now.AddDate(0, 1, 0)},
	} {
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
	}
	tool := assistantFinanceTestTool(t, agToolsCoach(app, "USD", currency.Rates{Base: "USD"}), "check_goal_pace")
	out := tool.run(json.RawMessage(`{}`))
	if !strings.Contains(out, "nonfinancial goal; use list_goals for progress") || strings.Contains(out, "target covered") || strings.Contains(out, "fully funded") || strings.Contains(out, "meets the canonical requirement") {
		t.Fatalf("nonfinancial goals got money verdicts: %s", out)
	}
	g := domain.Goal{ID: "coach-large", Name: "Large plan", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(10000, "USD"), CurrentAmount: money.New(0, "USD"), MonthlyContribution: money.New(math.MaxInt64, "USD"), TargetDate: now.AddDate(0, 2, 0)}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{"goal":"Large plan"}`))
	if !strings.Contains(out, "meets the canonical requirement") || strings.Contains(out, "is below") || strings.Contains(out, "unavailable") {
		t.Fatalf("large configured plan wrapped: %s", out)
	}
	// Legitimate finite coverage whose sum cannot fit is explicitly unknown.
	g.CurrentAmount = money.New(math.MaxInt64, "USD")
	g.TargetAmount = money.New(math.MaxInt64, "USD")
	g.Allocations = []domain.GoalAllocation{{AccountID: "a-check", Amount: money.New(1, "USD")}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{"goal":"Large plan"}`))
	if !strings.Contains(out, "coaching unavailable") || strings.Contains(out, "target covered") {
		t.Fatalf("coverage overflow became completion: %s", out)
	}
}
