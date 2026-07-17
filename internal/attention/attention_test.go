// SPDX-License-Identifier: MIT

package attention

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/money"
)

var now = time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)

func usd(n int64) money.Money { return money.New(n, "USD") }

func sampleInputs() Inputs {
	return Inputs{
		Now: now,
		Bills: []bills.Bill{
			{Name: "Card", Amount: usd(5000), DueDate: now.AddDate(0, 0, 1), DaysUntil: 1},   // critical
			{Name: "Rent", Amount: usd(120000), DueDate: now.AddDate(0, 0, 5), DaysUntil: 5}, // warning
			{Name: "Gym", Amount: usd(2000), DueDate: now.AddDate(0, 0, 20), DaysUntil: 20},  // out of window
		},
		Budgets: []budgeting.Status{
			{Budget: domain.Budget{ID: "b1", Name: "Dining"}, Percent: 130, State: budgeting.StateOver}, // critical
			{Budget: domain.Budget{ID: "b2", Name: "Fuel"}, Percent: 92, State: budgeting.StateNear},    // warning
			{Budget: domain.Budget{ID: "b3", Name: "Misc"}, Percent: 40, State: budgeting.StateOK},      // dropped
		},
		Stale: []domain.Account{
			{ID: "a1", Name: "Checking", BalanceAsOf: now.AddDate(0, 0, -40)}, // warning
		},
		Tasks: []domain.Task{
			{ID: "t1", Title: "Pay tax", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: now.AddDate(0, 0, -3)},    // overdue → critical
			{ID: "t2", Title: "Call bank", Status: domain.StatusOpen, Priority: domain.PriorityHigh},                              // high, not overdue → warning
			{ID: "t3", Title: "Someday", Status: domain.StatusOpen, Priority: domain.PriorityLow},                                 // dropped
			{ID: "t4", Title: "Done thing", Status: domain.StatusDone, Priority: domain.PriorityHigh, Due: now.AddDate(0, 0, -9)}, // dropped (done)
		},
		Anomaly: &insights.Anomaly{Category: "Groceries", PctChange: 80, Direction: insights.Up}, // info
	}
}

func TestRankAllOnOrdersBySeverityThenSoonness(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxItems = 0 // no cap for this test
	got := Rank(sampleInputs(), cfg)

	// Expected inclusion: 2 bills (card,rent) + 2 budgets (over,near) + 1 stale +
	// 2 tasks (overdue,high) + 1 spending = 8. Gym bill, OK budget, low task, done
	// task are excluded.
	if len(got) != 8 {
		t.Fatalf("got %d items, want 8: %+v", len(got), got)
	}

	// Severity must be non-increasing across the list.
	for i := 1; i < len(got); i++ {
		if got[i].Severity > got[i-1].Severity {
			t.Fatalf("not ordered by severity at %d: %v then %v", i, got[i-1].Severity, got[i].Severity)
		}
	}

	// Within Critical, soonest deadline wins — an overdue task (3 days past) beats
	// the Card bill due tomorrow, which in turn beats the dateless over-budget.
	if got[0].Kind != KindTask || got[0].Label != "Pay tax" {
		t.Fatalf("first item = %v %q, want the overdue task", got[0].Kind, got[0].Label)
	}
	if got[1].Kind != KindBill || got[1].Label != "Card" {
		t.Fatalf("second item = %v %q, want the Card bill", got[1].Kind, got[1].Label)
	}

	// Criticals: Card bill, Dining over, overdue task. Warnings: Rent bill, Fuel
	// near, Checking stale, high-priority task. Spending spike is Info.
	crit, warn := Counts(got)
	if crit != 3 || warn != 4 {
		t.Fatalf("counts crit=%d warn=%d, want 3/4", crit, warn)
	}
}

func TestRankTogglesDropSources(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxItems = 0
	cfg.Bills = false
	cfg.Spending = false
	got := Rank(sampleInputs(), cfg)
	for _, it := range got {
		if it.Kind == KindBill || it.Kind == KindSpending {
			t.Fatalf("disabled source still present: %v", it.Kind)
		}
	}
	// budgets(2) + stale(1) + tasks(2) = 5
	if len(got) != 5 {
		t.Fatalf("got %d, want 5 with bills+spending off", len(got))
	}
}

func TestRankBillsWindow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxItems = 0
	cfg.Budgets, cfg.Stale, cfg.Tasks, cfg.Spending = false, false, false, false
	cfg.BillsWindowDays = 3 // Rent (5d) now falls outside; only Card (1d) remains
	got := Rank(sampleInputs(), cfg)
	if len(got) != 1 || got[0].Label != "Card" {
		t.Fatalf("window=3 got %+v, want only Card", got)
	}
}

func TestRankMinSeverityFloor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxItems = 0
	cfg.MinSeverity = SeverityCritical
	got := Rank(sampleInputs(), cfg)
	for _, it := range got {
		if it.Severity != SeverityCritical {
			t.Fatalf("below-floor item survived: %v sev=%v", it.Kind, it.Severity)
		}
	}
	// criticals: Card bill, Dining over-budget, overdue task = 3
	if len(got) != 3 {
		t.Fatalf("got %d criticals, want 3", len(got))
	}
}

func TestRankMaxItemsCap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxItems = 2
	got := Rank(sampleInputs(), cfg)
	if len(got) != 2 {
		t.Fatalf("got %d, want capped to 2", len(got))
	}
	// The cap keeps the most urgent, so both should be Critical.
	for _, it := range got {
		if it.Severity != SeverityCritical {
			t.Fatalf("cap kept a non-critical over a critical: %v", it)
		}
	}
}

func TestRankEmptyIsCalm(t *testing.T) {
	if got := Rank(Inputs{Now: now}, DefaultConfig()); len(got) != 0 {
		t.Fatalf("empty inputs should yield no items, got %+v", got)
	}
}

// TestRankHouseholdSplit verifies chore classification: a plain manual to-do is
// Household; to-dos linked to a financial entity or created by a nudge — and
// every non-task kind — are money items.
func TestRankHouseholdSplit(t *testing.T) {
	in := Inputs{
		Now: now,
		Tasks: []domain.Task{
			{ID: "chore", Title: "Replace air filters", Status: domain.StatusOpen, Priority: domain.PriorityHigh},
			{ID: "linked", Title: "Top up baby fund", Status: domain.StatusOpen, Priority: domain.PriorityHigh, RelatedType: domain.RelatedGoal, RelatedID: "g1"},
			{ID: "nudged", Title: "Refresh balances", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Source: domain.SourceNudge},
		},
	}
	got := Rank(in, DefaultConfig())
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}
	byID := map[string]Item{}
	for _, it := range got {
		byID[it.AnchorID] = it
	}
	if !byID["chore"].Household {
		t.Fatalf("plain manual task should be Household: %+v", byID["chore"])
	}
	if byID["linked"].Household {
		t.Fatalf("goal-linked task should be a money item: %+v", byID["linked"])
	}
	if byID["nudged"].Household {
		t.Fatalf("nudge-created task should be a money item: %+v", byID["nudged"])
	}
}
