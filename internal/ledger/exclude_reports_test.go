// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestExcludeFromReports_AnalyticsNotBalance is the core TXC-1 invariant: a
// transaction marked ExcludeFromReports drops out of the income/expense
// analytics (PeriodTotals, CategorySpendSeries) but STILL moves the account
// balance and net worth.
func TestExcludeFromReports_AnalyticsNotBalance(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	d := func(day int) time.Time { return time.Date(2026, time.June, day, 0, 0, 0, 0, time.UTC) }
	start, end := d(1), time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	acct := domain.Account{ID: "a", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(100000, "USD")}
	normal := domain.Transaction{ID: "n", AccountID: "a", CategoryID: "food", Amount: money.New(-2000, "USD"), Date: d(5)}
	excluded := domain.Transaction{ID: "x", AccountID: "a", CategoryID: "food", Amount: money.New(-5000, "USD"), Date: d(6), ExcludeFromReports: true}
	txns := []domain.Transaction{normal, excluded}

	// Analytics: only the normal $20 counts, not the excluded $50.
	_, exp, err := PeriodTotals(txns, start, end, rates)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Amount != 2000 {
		t.Errorf("PeriodTotals expense = %d, want 2000 (excluded txn must not count)", exp.Amount)
	}
	series, err := CategorySpendSeries(txns, []time.Time{start, end}, rates)
	if err != nil {
		t.Fatal(err)
	}
	if got := series["food"][0]; got != 2000 {
		t.Errorf("CategorySpendSeries[food] = %d, want 2000 (excluded txn must not count)", got)
	}

	// Balance: BOTH move the money — 100000 - 2000 - 5000 = 93000.
	bal, err := Balance(acct, txns)
	if err != nil {
		t.Fatal(err)
	}
	if bal.Amount != 93000 {
		t.Errorf("Balance = %d, want 93000 (an excluded txn STILL moves the balance)", bal.Amount)
	}

	// Net worth includes it too.
	net, _, _, err := NetWorth([]domain.Account{acct}, txns, rates)
	if err != nil {
		t.Fatal(err)
	}
	if net.Amount != 93000 {
		t.Errorf("NetWorth = %d, want 93000 (excluded txn still counts toward net worth)", net.Amount)
	}
}
