// SPDX-License-Identifier: MIT

package widgetsource

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func expense(amount int64, cat, day string) domain.Transaction {
	return domain.Transaction{Amount: usd(-amount), CategoryID: cat, Date: mustDate(day)}
}

// TestBudgetStatusFrame verifies the Budgets resolver yields a rich Frame whose
// percent and tone columns are computed from the engine (rollup), so a renderer
// colors bars straight from the data — and that atRisk/limit filter rows.
func TestBudgetStatusFrame(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start, end := mustDate("2026-06-01"), mustDate("2026-07-01")
	cats := []domain.Category{
		{ID: "food", Name: "Food"},
		{ID: "rent", Name: "Rent"},
		{ID: "fun", Name: "Fun"},
	}
	budgets := []domain.Budget{
		{Name: "Food", CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(10000)}, // $100 limit
		{Name: "Rent", CategoryID: "rent", Scope: domain.ScopeShared, Limit: usd(10000)}, // $100 limit, $200 spent → over
		{Name: "Fun", CategoryID: "fun", Scope: domain.ScopeShared, Limit: usd(10000)},
	}
	txns := []domain.Transaction{
		expense(9000, "food", "2026-06-03"),  // 90% → near
		expense(20000, "rent", "2026-06-04"), // 200% → over
		expense(1000, "fun", "2026-06-05"),   // 10% → ok
	}

	fr := BudgetStatus(budgets, cats, txns, rates, start, end, false, 0)
	if fr.Rows != 3 {
		t.Fatalf("rows = %d, want 3", fr.Rows)
	}
	nameCol, _ := fr.Column("name")
	pctCol, _ := fr.Column("percent")
	stateCol, _ := fr.Column("state")
	overCol, _ := fr.Column("over")

	if pctCol.Type != domain.FieldPercent || stateCol.Type != domain.FieldTone {
		t.Fatalf("column types: percent=%s state=%s", pctCol.Type, stateCol.Type)
	}
	if got := nameCol.Str(0); got != "Food" {
		t.Errorf("row0 name = %q, want Food", got)
	}
	if got := pctCol.Num(0); got != 90 {
		t.Errorf("Food percent = %v, want 90", got)
	}
	if got := stateCol.Str(0); got != "near" {
		t.Errorf("Food tone = %q, want near", got)
	}
	if got := stateCol.Str(1); got != "over" {
		t.Errorf("Rent tone = %q, want over", got)
	}
	if over, ok := overCol.Values[1].(bool); !ok || !over {
		t.Errorf("Rent over = %v, want true", overCol.Values[1])
	}
	if got := stateCol.Str(2); got != "ok" {
		t.Errorf("Fun tone = %q, want ok", got)
	}

	// atRisk drops the on-track ("Fun") budget.
	risk := BudgetStatus(budgets, cats, txns, rates, start, end, true, 0)
	if risk.Rows != 2 {
		t.Fatalf("atRisk rows = %d, want 2 (near+over)", risk.Rows)
	}

	// limit caps rows.
	capped := BudgetStatus(budgets, cats, txns, rates, start, end, false, 1)
	if capped.Rows != 1 {
		t.Fatalf("limited rows = %d, want 1", capped.Rows)
	}
}

// TestAccountBalancesFrame verifies signed money columns, the negative→down tone,
// skipping un-computable accounts, archived exclusion and the row cap.
func TestAccountBalancesFrame(t *testing.T) {
	accounts := []domain.Account{
		{ID: "a1", Name: "Checking", Currency: "USD", OpeningBalance: usd(10000)},
		{ID: "a2", Name: "Card", Currency: "USD", OpeningBalance: usd(0)},
		{ID: "a3", Name: "Old", Currency: "USD", OpeningBalance: usd(500), Archived: true}, // excluded
	}
	txns := []domain.Transaction{
		{ID: "t1", AccountID: "a1", Amount: usd(5000)},  // Checking → +150.00
		{ID: "t2", AccountID: "a2", Amount: usd(-3000)}, // Card → -30.00
	}

	fr := AccountBalances(accounts, txns, false, 0)
	if fr.Rows != 2 {
		t.Fatalf("rows = %d, want 2 (archived excluded)", fr.Rows)
	}
	nameCol, _ := fr.Column("name")
	balCol, _ := fr.Column("balance")
	curCol, _ := fr.Column("currency")
	toneCol, _ := fr.Column("tone")

	if balCol.Type != domain.FieldMoney {
		t.Fatalf("balance type = %s, want money", balCol.Type)
	}
	if got := balCol.Int64(0); got != 15000 {
		t.Errorf("Checking balance = %d, want 15000", got)
	}
	if got := curCol.Str(0); got != "USD" {
		t.Errorf("Checking currency = %q, want USD", got)
	}
	if got := toneCol.Str(0); got != "" {
		t.Errorf("Checking tone = %q, want empty (positive)", got)
	}
	if got := nameCol.Str(1); got != "Card" {
		t.Errorf("row1 name = %q, want Card", got)
	}
	if got := balCol.Int64(1); got != -3000 {
		t.Errorf("Card balance = %d, want -3000", got)
	}
	if got := toneCol.Str(1); got != "down" {
		t.Errorf("Card tone = %q, want down (negative)", got)
	}

	if capped := AccountBalances(accounts, txns, false, 1); capped.Rows != 1 {
		t.Fatalf("limited rows = %d, want 1", capped.Rows)
	}
}

// TestAccountBalancesLiabilityPresentation locks the QA CF-09 fix: a liability
// presents as its owed magnitude NEGATIVE (accounting parens + down tone) under
// both at-rest sign conventions — positive-stored (the "amount you owe" add
// form) and negative-stored (the sample data).
func TestAccountBalancesLiabilityPresentation(t *testing.T) {
	accounts := []domain.Account{
		{ID: "l1", Name: "Loan+", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(55000)},
		{ID: "l2", Name: "Loan-", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-55000)},
	}
	fr := AccountBalances(accounts, nil, false, 0)
	if fr.Rows != 2 {
		t.Fatalf("rows = %d, want 2", fr.Rows)
	}
	balCol, _ := fr.Column("balance")
	toneCol, _ := fr.Column("tone")
	for i := range 2 {
		if got := balCol.Int64(i); got != -55000 {
			t.Errorf("row%d balance = %d, want -55000 (owed magnitude, negative)", i, got)
		}
		if got := toneCol.Str(i); got != "down" {
			t.Errorf("row%d tone = %q, want down", i, got)
		}
	}
}

// TestRecentTransactionsFrame verifies the recent resolver returns every txn,
// newest first, with the txn's own signed amount + currency.
func TestRecentTransactionsFrame(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "t1", Desc: "Old", Amount: usd(-1000), Date: mustDate("2026-06-01")},
		{ID: "t2", Desc: "New", Amount: usd(5000), Date: mustDate("2026-06-20")},
		{ID: "t3", Desc: "Mid", Amount: usd(-250), Date: mustDate("2026-06-10")},
	}
	fr := RecentTransactions(txns)
	if fr.Rows != 3 {
		t.Fatalf("rows = %d, want 3", fr.Rows)
	}
	descCol, _ := fr.Column("desc")
	amtCol, _ := fr.Column("amount")
	if descCol.Str(0) != "New" {
		t.Errorf("row0 = %q, want New (newest first)", descCol.Str(0))
	}
	if amtCol.Int64(0) != 5000 {
		t.Errorf("row0 amount = %d, want 5000", amtCol.Int64(0))
	}
	if amtCol.Type != domain.FieldMoney {
		t.Errorf("amount type = %s, want money", amtCol.Type)
	}
}

// TestCashFlowSeriesFrame verifies the cash-flow resolver yields one row per month
// in the trailing window with base-currency income/expense.
func TestCashFlowSeriesFrame(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "t1", Amount: usd(5000), Date: mustDate("2026-06-10")},  // income, current month
		{ID: "t2", Amount: usd(-2000), Date: mustDate("2026-06-12")}, // expense, current month
	}
	fr := CashFlowSeries(txns, currency.Rates{Base: "USD"}, mustDate("2026-06-15"), 4)
	if fr.Rows != 4 {
		t.Fatalf("rows = %d, want 4", fr.Rows)
	}
	incCol, _ := fr.Column("income")
	expCol, _ := fr.Column("expense")
	// Last row is the current month.
	if incCol.Int64(3) != 5000 {
		t.Errorf("current income = %d, want 5000", incCol.Int64(3))
	}
	if expCol.Int64(3) != 2000 {
		t.Errorf("current expense = %d, want 2000", expCol.Int64(3))
	}
}

// TestSpendingBreakdownFrame verifies the breakdown resolver rolls sub-categories up
// to their root and ranks by spend with a correct percent share.
func TestSpendingBreakdownFrame(t *testing.T) {
	cats := []domain.Category{
		{ID: "food", Name: "Food"},
		{ID: "groc", Name: "Groceries", ParentID: "food"},
		{ID: "rent", Name: "Rent"},
	}
	txns := []domain.Transaction{
		{Amount: usd(-3000), CategoryID: "groc", Date: mustDate("2026-06-03")}, // rolls up to Food
		{Amount: usd(-1000), CategoryID: "food", Date: mustDate("2026-06-04")}, // Food = 4000
		{Amount: usd(-1000), CategoryID: "rent", Date: mustDate("2026-06-05")}, // Rent = 1000
	}
	fr := SpendingBreakdown(cats, txns, currency.Rates{Base: "USD"}, mustDate("2026-06-01"), mustDate("2026-07-01"))
	if fr.Rows != 2 {
		t.Fatalf("rows = %d, want 2 (Food, Rent)", fr.Rows)
	}
	nameCol, _ := fr.Column("name")
	amtCol, _ := fr.Column("amount")
	pctCol, _ := fr.Column("percent")
	if nameCol.Str(0) != "Food" || amtCol.Int64(0) != 4000 {
		t.Errorf("top = %q/%d, want Food/4000 (rolled up)", nameCol.Str(0), amtCol.Int64(0))
	}
	if got := pctCol.Num(0); got < 79 || got > 81 {
		t.Errorf("Food percent = %v, want ~80", got)
	}
}

// TestNetWorthSeriesFrame verifies the chart Frame carries a time column and a
// money value column, one row per cutoff, growing with the ledger.
func TestNetWorthSeriesFrame(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{{ID: "a1", Currency: "USD", OpeningBalance: usd(10000)}}
	txns := []domain.Transaction{
		{ID: "t1", AccountID: "a1", Amount: usd(5000), Date: mustDate("2026-05-15")},
		{ID: "t2", AccountID: "a1", Amount: usd(5000), Date: mustDate("2026-06-15")},
	}
	cutoffs := []time.Time{mustDate("2026-05-01"), mustDate("2026-06-01"), mustDate("2026-07-01")}

	fr := NetWorthSeries(accounts, txns, rates, cutoffs)
	if fr.Rows != 3 {
		t.Fatalf("rows = %d, want 3", fr.Rows)
	}
	tCol, _ := fr.Column("t")
	vCol, _ := fr.Column("value")
	if tCol.Type != domain.FieldNumber || vCol.Type != domain.FieldMoney {
		t.Fatalf("column types: t=%s value=%s", tCol.Type, vCol.Type)
	}
	// Opening only by 05-01, +5000 by 06-01, +10000 by 07-01.
	if got := vCol.Int64(0); got != 10000 {
		t.Errorf("value0 = %d, want 10000", got)
	}
	if got := vCol.Int64(1); got != 15000 {
		t.Errorf("value1 = %d, want 15000", got)
	}
	if got := vCol.Int64(2); got != 20000 {
		t.Errorf("value2 = %d, want 20000", got)
	}
	if got := int64(tCol.Num(2)); got != cutoffs[2].Unix() {
		t.Errorf("t2 = %d, want %d", got, cutoffs[2].Unix())
	}
}
