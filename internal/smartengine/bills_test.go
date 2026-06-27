// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestBL2CanCover(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 10000, ref)} // $100 liquid
	in.Recurring = []domain.Recurring{{
		ID: "r1", Label: "Rent", Amount: usd(-50000), Cadence: domain.CadenceMonthly,
		NextDue: ref.AddDate(0, 0, 8),
	}}
	got := bl2CanCover(in)
	if len(got) != 1 {
		t.Fatalf("want 1 coverage warning, got %d: %+v", len(got), got)
	}
	if got[0].Severity != smart.SeverityAlert || !got[0].HasAmount {
		t.Errorf("expected alert with shortfall amount, got %+v", got[0])
	}
}

func TestBL2HealthyNoWarning(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 1000000, ref)}
	in.Recurring = []domain.Recurring{{
		ID: "r1", Label: "Rent", Amount: usd(-50000), Cadence: domain.CadenceMonthly,
		NextDue: ref.AddDate(0, 0, 8),
	}}
	if got := bl2CanCover(in); len(got) != 0 {
		t.Errorf("healthy cash — want 0, got %d", len(got))
	}
}

func liabilityCard(id, name string, dueDay int, minPay int64) domain.Account {
	return domain.Account{
		ID: id, Name: name, Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		Currency: "USD", DueDayOfMonth: dueDay, MinPayment: usd(minPay),
	}
}

func TestBL3MissedBill(t *testing.T) {
	in := baseInput() // now = June 15
	in.Accounts = []domain.Account{liabilityCard("c", "Visa", 5, 5000)}
	// Previous due was June 5; no payment recorded since → missed.
	got := bl3MissedBill(in)
	if len(got) != 1 {
		t.Fatalf("want 1 missed-bill insight, got %d: %+v", len(got), got)
	}
	if got[0].Severity != smart.SeverityAlert {
		t.Errorf("missed bill should alert, got %v", got[0].Severity)
	}
}

func TestBL3PaidNotFlagged(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{liabilityCard("c", "Visa", 5, 5000)}
	in.Transactions = []domain.Transaction{
		txn("p", "c", time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), 5000), // a payment after due
	}
	if got := bl3MissedBill(in); len(got) != 0 {
		t.Errorf("payment recorded — want 0, got %d: %+v", len(got), got)
	}
}

func TestBL7BillIncrease(t *testing.T) {
	in := baseInput()
	var txns []domain.Transaction
	for i := range 6 {
		d := time.Date(2026, time.Month(1+i), 8, 0, 0, 0, 0, time.UTC)
		amt := int64(-5000)
		if i >= 3 {
			amt = -6000 // a 20% rise mid-series
		}
		txns = append(txns, domain.Transaction{ID: "x" + itoa64(int64(i)), AccountID: "a", Date: d, Amount: usd(amt), Desc: "Internet"})
	}
	in.Transactions = txns
	got := bl7BillIncrease(in)
	if len(got) != 1 {
		t.Fatalf("want 1 increase insight, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 1000 { // $10 delta
		t.Errorf("delta amount = %d, want 1000", got[0].Amount.Amount)
	}
}

func TestBL9SinkingFund(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{{
		ID: "ins", Label: "Car Insurance", Amount: usd(-60000), Cadence: domain.CadenceYearly,
		NextDue: ref.AddDate(0, 0, 30),
	}}
	got := bl9SinkingFund(in)
	if len(got) != 1 {
		t.Fatalf("want 1 sinking-fund nudge, got %d: %+v", len(got), got)
	}
	// $600/yr → $50/mo set-aside.
	if got[0].Amount.Amount != 5000 {
		t.Errorf("monthly set-aside = %d, want 5000", got[0].Amount.Amount)
	}
	if got[0].Action == nil || got[0].Action.Kind != smart.ActionCreateGoal {
		t.Errorf("expected a create-goal action, got %v", got[0].Action)
	}
	if !got[0].Action.GoalIsSinkingFund {
		t.Errorf("expected GoalIsSinkingFund = true")
	}
	if got[0].Page != smart.PageGoals {
		t.Errorf("expected Page = PageGoals, got %q", got[0].Page)
	}
}

func TestBL9SkipsSmallAndMonthly(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{
		{ID: "s", Label: "Tiny", Amount: usd(-1000), Cadence: domain.CadenceYearly, NextDue: ref.AddDate(0, 0, 30)},  // < $200/yr
		{ID: "m", Label: "Rent", Amount: usd(-50000), Cadence: domain.CadenceMonthly, NextDue: ref.AddDate(0, 0, 5)}, // monthly, not irregular
	}
	if got := bl9SinkingFund(in); len(got) != 0 {
		t.Errorf("want 0, got %d: %+v", len(got), got)
	}
}
