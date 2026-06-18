package widgetdata

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
)

func day(d int) time.Time { return time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC) }

func TestListRowsTransactionsNewestFirstAndCapped(t *testing.T) {
	d := Data{Transactions: []domain.Transaction{
		{ID: "a", Date: day(1), Desc: "Oldest", Amount: money.New(-100, "USD")},
		{ID: "b", Date: day(20), Desc: "Newest", Amount: money.New(-200, "USD")},
		{ID: "c", Date: day(10), Desc: "Middle", Amount: money.New(300, "USD")},
		{ID: "d", Date: day(5), Desc: "", Payee: "PayeeFallback", Amount: money.New(-50, "USD")},
	}}
	rows, ok := ListRows(widgetspec.SourceTransactions, d, 2)
	if !ok {
		t.Fatal("transactions should be a known source")
	}
	if len(rows) != 2 {
		t.Fatalf("cap not applied: got %d rows, want 2", len(rows))
	}
	// Newest first: day 20 then day 10.
	if rows[0].Label != "Newest" || rows[1].Label != "Middle" {
		t.Errorf("ordering wrong: %+v", rows)
	}
	// Accounting format with parentheses for negatives.
	if rows[0].Value != "($2.00)" {
		t.Errorf("value format wrong: %q", rows[0].Value)
	}
	// Description falls back to payee when empty. Newest-first order is
	// day20, day10, day5(payee), day1 — so the fallback row is index 2.
	all, _ := ListRows(widgetspec.SourceTransactions, d, 4)
	if all[2].Label != "PayeeFallback" {
		t.Errorf("desc->payee fallback failed: %+v", all)
	}
	// Input slice not mutated (still original order).
	if d.Transactions[0].ID != "a" {
		t.Error("ListRows mutated its input order")
	}
}

func TestListRowsOtherSources(t *testing.T) {
	asOf := day(1)
	d := Data{
		Accounts: []domain.Account{{ID: "ac", Name: "Checking", Currency: "USD", Class: domain.ClassAsset,
			OpeningBalance: money.New(100000, "USD"), BalanceAsOf: asOf}},
		Budgets: []domain.Budget{{ID: "b", Name: "Food", Limit: money.New(50000, "USD")}},
		Goals:   []domain.Goal{{ID: "g", Name: "Trip", TargetAmount: money.New(200000, "USD"), CurrentAmount: money.New(50000, "USD")}},
		Tasks:   []domain.Task{{ID: "t", Title: "Pay rent", Status: domain.StatusOpen}},
		Rates:   currency.Rates{Base: "USD"},
	}
	if rows, _ := ListRows(widgetspec.SourceAccounts, d, 5); len(rows) != 1 || rows[0].Value != "$1,000.00" {
		t.Errorf("accounts row wrong: %+v", rows)
	}
	if rows, _ := ListRows(widgetspec.SourceBudgets, d, 5); rows[0].Value != "$500.00" {
		t.Errorf("budgets row wrong: %+v", rows)
	}
	if rows, _ := ListRows(widgetspec.SourceGoals, d, 5); rows[0].Value != "25%" {
		t.Errorf("goals row wrong: %+v", rows)
	}
	if rows, _ := ListRows(widgetspec.SourceTasks, d, 5); rows[0].Value != "open" {
		t.Errorf("tasks row wrong: %+v", rows)
	}
}

func TestListRowsUnknownSource(t *testing.T) {
	if rows, ok := ListRows("bogus", Data{}, 5); ok || rows != nil {
		t.Errorf("unknown source should return ok=false, got %v %+v", ok, rows)
	}
}

func TestKPIText(t *testing.T) {
	cases := []struct {
		v      float64
		format string
		base   string
		want   string
	}{
		{15343.50, widgetspec.FormatCurrency, "USD", "$15,343.50"}, // rounding, no dropped cent
		{60, widgetspec.FormatPercent, "USD", "60%"},
		{1234.5, widgetspec.FormatNumber, "USD", "1234.5"},
		{-200, widgetspec.FormatCurrency, "USD", "($200.00)"},
	}
	for _, c := range cases {
		if got := KPIText(c.v, c.format, c.base); got != c.want {
			t.Errorf("KPIText(%v,%q) = %q, want %q", c.v, c.format, got, c.want)
		}
	}
}

func TestChartWindow(t *testing.T) {
	w := ChartWindow(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), 6)
	if len(w) != 6 {
		t.Fatalf("want 6 cutoffs, got %d", len(w))
	}
	// Monotonic, month-aligned.
	for i := 1; i < len(w); i++ {
		if !w[i].After(w[i-1]) {
			t.Errorf("cutoffs not increasing: %v", w)
		}
	}
	if ChartWindow(time.Now(), 0); len(ChartWindow(time.Now(), 0)) != 1 {
		t.Error("months<1 should clamp to 1")
	}
}
