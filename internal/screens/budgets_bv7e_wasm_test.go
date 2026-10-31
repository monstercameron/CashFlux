// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBV7EDailyAllowanceUsesCommittedFreeAmountOnce(t *testing.T) {
	anchor := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.October, 20, 12, 30, 0, 0, time.UTC)
	status := budgeting.Status{
		Budget:    domain.Budget{ID: "transport", Period: domain.PeriodMonthly},
		Remaining: money.New(130_000, "USD"), FreeAfterCommitted: money.New(20_000, "USD"),
		Committed:  budgeting.CommittedResult{Required: money.New(110_000, "USD")},
		Projection: budgeting.Pace{Committed: money.New(110_000, "USD")},
	}
	got, ok := budgetRowRestPerDay(status, anchor, prefs.Prefs{WeekStart: prefs.WeekMonday}, now)
	if !ok || got.Amount != 1_666 {
		t.Fatalf("daily allowance = %v, %v; want 1666 cents after bills across 12 days", got, ok)
	}
	if gotDays := budgetDaysRemaining(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)); gotDays != 12 {
		t.Fatalf("days remaining = %d, want inclusive 12", gotDays)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(bv7eDuplicateDailyCaptionProbe, struct{}{}))
	if f.Text() != "" {
		t.Fatalf("duplicate daily allowance caption rendered: %q", f.Text())
	}
}

func bv7eDuplicateDailyCaptionProbe(_ struct{}) ui.Node {
	click := ui.UseEvent(shorthand.Prevent(func() {}))
	status := budgeting.Status{Budget: domain.Budget{ID: "daily", Period: domain.PeriodMonthly}}
	return budgetRowPeriodNotes(false, budgetRowProps{}, status, click).proratedLine
}

func TestBV7EUnmetRolloverTargetIsVisibleForOverBudget(t *testing.T) {
	status := budgeting.Status{
		Budget: domain.Budget{ID: "groceries", Period: domain.PeriodMonthly, Rollover: true,
			TargetKind: domain.TargetRefillUpTo, TargetAmount: money.New(45_000, "USD")},
		Limit: money.New(20_520, "USD"), Remaining: money.New(-3_730, "USD"),
		State:    budgeting.StateOver,
		Rollover: budgeting.RolloverView{Current: budgeting.RolloverPeriodState{CarryIn: money.New(-24_480, "USD")}},
	}
	if !budgetTargetLineVisible(status) {
		t.Fatal("unmet rollover target was hidden")
	}
	f := render.New(t)
	f.Render(budgetTargetLine(status))
	line := byTestID(f, "span", "budget-target-groceries")
	if line == nil || line.Text() != "Refill to $450.00 · $244.80 to go" {
		t.Fatalf("target line = %v; want unmet $244.80 refill target", line)
	}
}

func TestBV7ERefillTargetMatchingLimitWithoutCarryIsHidden(t *testing.T) {
	status := budgeting.Status{Budget: domain.Budget{TargetKind: domain.TargetRefillUpTo, TargetAmount: money.New(45_000, "USD")},
		Limit: money.New(45_000, "USD")}
	if budgetTargetLineVisible(status) {
		t.Fatal("refill target that repeats the limit was shown without carry")
	}
}

func TestBV7ECompactAndFullBillsCaptionUsesNeutralCommitmentCopy(t *testing.T) {
	status := budgeting.Status{Budget: domain.Budget{ID: "transport"}, Remaining: money.New(130_000, "USD"),
		FreeAfterCommitted: money.New(20_000, "USD"),
		Committed:          budgeting.CommittedResult{Required: money.New(110_000, "USD")},
		Projection:         budgeting.Pace{Committed: money.New(110_000, "USD")}}
	f := render.New(t)
	f.Render(budgetRecurringStatusLine(status, "$200.00"))
	line := byTestID(f, "span", "budget-free-after-committed-transport")
	if line == nil || line.Text() != "$200.00 free after $1,100.00 in committed bills" {
		t.Fatalf("bills caption = %v, want neutral committed-bills wording", line)
	}
	if text := line.Text(); text == "" || containsAny(text, "due this month", "bills due") {
		t.Fatalf("bills caption makes an unsupported due-date claim: %q", text)
	}
	f.Render(budgetRowProjectionLine(status, budgetRowProps{}))
	compactLine := byTestID(f, "span", "budget-free-after-committed-transport")
	if compactLine == nil || compactLine.Text() != line.Text() {
		t.Fatalf("compact bills caption = %v, want the full-card copy %q", compactLine, line.Text())
	}
}

func TestBV7ERaiseRequiresRepeatedOverspendAndCurrentProjectionOverLimit(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		projected int64
		want      bool
	}{
		{name: "under pace now", count: 3, projected: 99_999},
		{name: "only one prior overage", count: 1, projected: 100_001},
		{name: "two prior overages and currently over", count: 2, projected: 100_001, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := budgetRaiseAllowed(tc.count, money.New(tc.projected, "USD"), money.New(100_000, "USD"))
			if got != tc.want {
				t.Fatalf("raise allowed = %t, want %t", got, tc.want)
			}
		})
	}
}

func containsAny(value string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(strings.ToLower(value), phrase) {
			return true
		}
	}
	return false
}
