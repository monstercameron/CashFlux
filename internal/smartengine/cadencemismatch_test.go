// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

var cadNow = time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)

// spendIn builds one expense per named month-offset back from now.
func spendIn(cat string, minor int64, monthsAgo ...int) []domain.Transaction {
	var out []domain.Transaction
	for _, m := range monthsAgo {
		d := cadNow.AddDate(0, -m, 0)
		out = append(out, domain.Transaction{
			ID: cat + "-" + d.Format("2006-01"), AccountID: "a", Date: d,
			CategoryID: cat, Amount: money.New(-minor, "USD"),
		})
	}
	return out
}

func cadInput(txns []domain.Transaction, budgets []domain.Budget) Input {
	return Input{
		Now: cadNow, Base: "USD", Rates: currency.Rates{Base: "USD"},
		Transactions: txns, Budgets: budgets,
		Categories: []domain.Category{{ID: "ins", Name: "Insurance"}, {ID: "food", Name: "Groceries"}},
		Accounts:   []domain.Account{{ID: "a", Name: "Checking", Class: domain.ClassAsset}},
	}
}

func TestB16FlagsAMonthlyBudgetForAYearlyBill(t *testing.T) {
	txns := spendIn("ins", 60_000, 5)
	budgets := []domain.Budget{{ID: "b1", Name: "Insurance", CategoryID: "ins", Period: domain.PeriodMonthly}}
	got := b16CadenceMismatch(cadInput(txns, budgets))
	if len(got) != 1 {
		t.Fatalf("insights = %d, want 1: %+v", len(got), got)
	}
	// The remedy is the set-aside, so it has to be in the sentence.
	if !strings.Contains(got[0].Detail, "Setting aside") {
		t.Errorf("no set-aside offered: %q", got[0].Detail)
	}
	if !strings.Contains(got[0].Detail, "once or twice a year") {
		t.Errorf("the rhythm was not named: %q", got[0].Detail)
	}
	// It declares a MONTHLY amount so the next-best-action ranking can weigh it.
	if got[0].AmountCadence != 1 { // smart.AmountMonthly
		t.Errorf("amount cadence = %v, want monthly", got[0].AmountCadence)
	}
}

// Groceries vary but happen every month. Flagging them would be noise.
func TestB16LeavesAnOrdinaryMonthlyBudgetAlone(t *testing.T) {
	var txns []domain.Transaction
	for m := range 12 {
		txns = append(txns, spendIn("food", int64(30_000+m*1_000), m)...)
	}
	budgets := []domain.Budget{{ID: "b2", Name: "Groceries", CategoryID: "food", Period: domain.PeriodMonthly}}
	if got := b16CadenceMismatch(cadInput(txns, budgets)); len(got) != 0 {
		t.Errorf("ordinary monthly spending was flagged: %+v", got)
	}
}

// A budget already on a lumpy period IS the remedy being suggested; flagging it
// would be advising somebody to do what they did.
func TestB16IgnoresBudgetsAlreadyOnALumpyPeriod(t *testing.T) {
	txns := spendIn("ins", 60_000, 5)
	budgets := []domain.Budget{{ID: "b3", Name: "Insurance", CategoryID: "ins", Period: domain.PeriodYearly}}
	if got := b16CadenceMismatch(cadInput(txns, budgets)); len(got) != 0 {
		t.Errorf("a yearly budget for a yearly bill was flagged: %+v", got)
	}
}

func TestB16DoesNotCrashOnTheSampleDataset(t *testing.T) {
	ds := store.SampleDataset()
	got := b16CadenceMismatch(Input{
		Now: time.Now(), Base: "USD", Rates: currency.Rates{Base: "USD"},
		Transactions: ds.Transactions, Budgets: ds.Budgets,
		Categories: ds.Categories, Accounts: ds.Accounts,
	})
	for _, g := range got {
		if g.Detail == "" || g.Title == "" {
			t.Errorf("a finding carried no copy: %+v", g)
		}
	}
	t.Logf("sample produced %d cadence findings", len(got))
}
