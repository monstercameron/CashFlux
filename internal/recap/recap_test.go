// SPDX-License-Identifier: MIT

package recap

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func dtu(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func usd(minor int64) money.Money { return money.New(minor, "USD") }

// exp is a positive-magnitude expense (stored as a negative amount) on the shared
// "checking" account.
func exp(cat, desc string, minor int64, on time.Time) domain.Transaction {
	return domain.Transaction{AccountID: "checking", CategoryID: cat, Desc: desc, Amount: usd(-minor), Date: on}
}

// inc is income on the shared "checking" account.
func inc(desc string, minor int64, on time.Time) domain.Transaction {
	return domain.Transaction{AccountID: "checking", Desc: desc, Amount: usd(minor), Date: on}
}

func checking() domain.Account {
	return domain.Account{ID: "checking", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(1000000)}
}

func usdRates() currency.Rates { return currency.Rates{Base: "USD"} }

func TestComputeMidMonth(t *testing.T) {
	now := dtu(2026, time.June, 15) // window [Jun 1, Jun 16); prior [May 1, May 16)
	txns := []domain.Transaction{
		inc("June salary", 400000, dtu(2026, time.June, 1)),
		exp("food", "Groceries", 100000, dtu(2026, time.June, 5)),
		exp("rent", "Rent", 200000, dtu(2026, time.June, 6)),
		// Prior comparable span (first 15 days of May):
		inc("May salary", 300000, dtu(2026, time.May, 2)),
		exp("food", "Groceries", 50000, dtu(2026, time.May, 3)),
		// In May but AFTER the 15-day prior span — excluded from PrevExpense,
		// still counted in the entering net worth (it's before June 1).
		exp("fun", "Concert", 30000, dtu(2026, time.May, 20)),
	}
	r, err := Compute(now, txns, []domain.Account{checking()}, usdRates())
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if !r.HasData {
		t.Fatal("HasData = false, want true")
	}
	if r.Income != 400000 || r.Expense != 300000 || r.Net != 100000 {
		t.Errorf("income/expense/net = %d/%d/%d, want 400000/300000/100000", r.Income, r.Expense, r.Net)
	}
	if r.SavingsRate != 25 {
		t.Errorf("SavingsRate = %d, want 25", r.SavingsRate)
	}
	if r.PrevExpense != 50000 {
		t.Errorf("PrevExpense = %d, want 50000 (the May-20 concert is outside the 15-day span)", r.PrevExpense)
	}
	if !r.SpendDeltaKnown || r.SpendDeltaPct != 500 {
		t.Errorf("SpendDelta = %d (known=%v), want 500 known", r.SpendDeltaPct, r.SpendDeltaKnown)
	}
	if r.TopCategoryID != "rent" || r.TopCategoryAmount != 200000 {
		t.Errorf("top category = %q/%d, want rent/200000", r.TopCategoryID, r.TopCategoryAmount)
	}
	// food is the only category with a defined delta (rent/June are new → prior 0).
	if !r.MoverHasData || r.MoverID != "food" || r.MoverDelta != 50000 {
		t.Errorf("mover = %q Δ%d (data=%v), want food Δ50000", r.MoverID, r.MoverDelta, r.MoverHasData)
	}
	if !r.BiggestExpenseKnown || r.BiggestExpenseDesc != "Rent" || r.BiggestExpenseAmount != 200000 {
		t.Errorf("biggest expense = %q/%d, want Rent/200000", r.BiggestExpenseDesc, r.BiggestExpenseAmount)
	}
	// Entering NW = 1,000,000 opening + May flows (300000-50000-30000)=220000 → 1,220,000.
	// Ending NW adds June flows (400000-100000-200000)=100000 → 1,320,000.
	if r.NetWorthStart != 1220000 || r.NetWorthEnd != 1320000 || r.NetWorthDelta != 100000 {
		t.Errorf("net worth start/end/delta = %d/%d/%d, want 1220000/1320000/100000", r.NetWorthStart, r.NetWorthEnd, r.NetWorthDelta)
	}
	if r.TxnCount != 3 {
		t.Errorf("TxnCount = %d, want 3 (only in-window non-transfer)", r.TxnCount)
	}
	// Window is Jun 1–15 (15 days); expenses fall on Jun 5 and Jun 6, so 13 days
	// had no spending (the Jun 1 income-only day counts as no-spend).
	if r.NoSpendDays != 13 {
		t.Errorf("NoSpendDays = %d, want 13", r.NoSpendDays)
	}
	if r.Complete {
		t.Error("Complete = true, want false for a mid-month recap")
	}
	if !r.Saved() {
		t.Error("Saved() = false, want true (net positive)")
	}
}

func TestComputeEmpty(t *testing.T) {
	now := dtu(2026, time.June, 15)
	r, err := Compute(now, nil, []domain.Account{checking()}, usdRates())
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.HasData {
		t.Error("HasData = true, want false for no activity")
	}
	if r.Income != 0 || r.Expense != 0 || r.TxnCount != 0 {
		t.Errorf("expected all-zero flows, got income=%d expense=%d count=%d", r.Income, r.Expense, r.TxnCount)
	}
	if r.NetWorthDelta != 0 {
		t.Errorf("NetWorthDelta = %d, want 0 (no activity moves an opening balance)", r.NetWorthDelta)
	}
	if r.SpendDeltaKnown {
		t.Error("SpendDeltaKnown = true, want false with zero prior spend")
	}
}

func TestComputeSpendDown(t *testing.T) {
	now := dtu(2026, time.June, 10) // window [Jun 1, Jun 11); prior [May 1, May 11)
	txns := []domain.Transaction{
		exp("food", "Groceries", 20000, dtu(2026, time.June, 4)),
		exp("food", "Groceries", 100000, dtu(2026, time.May, 4)),
	}
	r, err := Compute(now, txns, []domain.Account{checking()}, usdRates())
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if !r.SpendDeltaKnown || r.SpendDeltaPct != -80 {
		t.Errorf("SpendDeltaPct = %d (known=%v), want -80", r.SpendDeltaPct, r.SpendDeltaKnown)
	}
	if !r.SpendDown() {
		t.Error("SpendDown() = false, want true")
	}
}

func TestComputeCompleteMonth(t *testing.T) {
	// On the last day of the month the window reaches the month end, so the recap
	// covers the whole (now fully-elapsed) month.
	now := dtu(2026, time.June, 30)
	txns := []domain.Transaction{
		exp("food", "Groceries", 10000, dtu(2026, time.June, 4)),
	}
	r, err := Compute(now, txns, []domain.Account{checking()}, usdRates())
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if !r.Complete {
		t.Error("Complete = false, want true on the last day of the month")
	}
	if !r.AsOf.Equal(dtu(2026, time.July, 1)) {
		t.Errorf("AsOf = %v, want Jul 1 (capped at month end)", r.AsOf)
	}
}
