// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestB8SafeToSpend(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 300000, ref)} // $3000 liquid
	got := b8SafeToSpend(in)
	if len(got) != 1 {
		t.Fatalf("want 1 safe-to-spend, got %d", len(got))
	}
	// No bills, no goals → safe == liquid == $3000.
	if got[0].Amount.Amount != 300000 {
		t.Errorf("safe = %d, want 300000", got[0].Amount.Amount)
	}
	if got[0].Severity != smart.SeverityInfo {
		t.Errorf("positive safe → info, got %v", got[0].Severity)
	}
}

func TestB8EmptyDatasetSaysNothing(t *testing.T) {
	// A brand-new dataset (no accounts at all) must not warn "liquid cash is
	// very low — $0.00": there is nothing to be low on yet (C356).
	in := baseInput()
	if got := b8SafeToSpend(in); len(got) != 0 {
		t.Fatalf("empty dataset should produce no insight, got %+v", got)
	}
}

func TestB8TightMonthWarns(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 10000, ref)} // $100
	in.Recurring = []domain.Recurring{{
		ID: "r", Label: "Rent", Amount: usd(-50000), Cadence: domain.CadenceMonthly, NextDue: ref.AddDate(0, 0, 3),
	}}
	got := b8SafeToSpend(in)
	if len(got) != 1 || got[0].Severity != smart.SeverityWarn {
		t.Fatalf("expected a tight-month warning, got %+v", got)
	}
}

func TestB9PacingNudge(t *testing.T) {
	in := baseInput()
	in.Categories = []domain.Category{{ID: "dining", Name: "Dining", Kind: domain.KindExpense}}
	in.Budgets = []domain.Budget{{
		ID: "b", Name: "Dining", CategoryID: "dining", Period: domain.PeriodMonthly,
		Limit: usd(20000), Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
	}}
	// Half the month elapsed (June 15), already spent the whole limit → projects 2× over.
	in.Transactions = []domain.Transaction{
		txn("t", "x", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), -20000),
	}
	in.Transactions[0].CategoryID = "dining"
	got := b9PacingNudge(in)
	if len(got) != 1 {
		t.Fatalf("want 1 pacing nudge, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount <= 0 {
		t.Errorf("expected projected overspend amount, got %+v", got[0].Amount)
	}
}

func TestB9OnTrackNoNudge(t *testing.T) {
	in := baseInput()
	in.Budgets = []domain.Budget{{
		ID: "b", Name: "Dining", CategoryID: "dining", Period: domain.PeriodMonthly,
		Limit: usd(20000), Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
	}}
	in.Transactions = []domain.Transaction{
		{ID: "t", AccountID: "x", CategoryID: "dining", Date: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), Amount: usd(-1000)},
	}
	if got := b9PacingNudge(in); len(got) != 0 {
		t.Errorf("under-budget pace — want 0, got %d: %+v", len(got), got)
	}
}

func TestB10UncoveredSpending(t *testing.T) {
	in := baseInput()
	in.Categories = []domain.Category{{ID: "rides", Name: "Rideshare"}}
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	var txns []domain.Transaction
	for k := 1; k <= 3; k++ {
		txns = append(txns, domain.Transaction{
			ID: "r" + itoa64(int64(k)), AccountID: "x", CategoryID: "rides",
			Date: monthStart.AddDate(0, -k, 9), Amount: usd(-10000), Desc: "Uber", // $100/mo
		})
	}
	in.Transactions = txns
	got := b10UncoveredSpending(in)
	if len(got) != 1 {
		t.Fatalf("want 1 uncovered nudge, got %d: %+v", len(got), got)
	}
	if got[0].Key != "SMART-B10:rides" {
		t.Errorf("wrong category flagged: %s", got[0].Key)
	}
}

func TestB10CoveredCategorySkipped(t *testing.T) {
	in := baseInput()
	in.Categories = []domain.Category{{ID: "rides", Name: "Rideshare"}}
	in.Budgets = []domain.Budget{{ID: "b", Name: "Rideshare", CategoryID: "rides", Period: domain.PeriodMonthly, Limit: usd(15000)}}
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	var txns []domain.Transaction
	for k := 1; k <= 3; k++ {
		txns = append(txns, domain.Transaction{ID: "r" + itoa64(int64(k)), AccountID: "x", CategoryID: "rides",
			Date: monthStart.AddDate(0, -k, 9), Amount: usd(-10000), Desc: "Uber"})
	}
	in.Transactions = txns
	if got := b10UncoveredSpending(in); len(got) != 0 {
		t.Errorf("budgeted category — want 0, got %d", len(got))
	}
}
