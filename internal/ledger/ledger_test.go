package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

// mustDate parses a YYYY-MM-DD date for tests.
func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestBalance(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(10000)}
	all := []domain.Transaction{
		{ID: "t1", AccountID: "a1", Amount: usd(5000)},
		{ID: "t2", AccountID: "a1", Amount: usd(-3000)},
		{ID: "t3", AccountID: "other", Amount: usd(9999)}, // ignored
	}
	bal, err := Balance(acc, all)
	if err != nil {
		t.Fatalf("Balance error: %v", err)
	}
	if !bal.Equal(usd(12000)) {
		t.Errorf("Balance = %v, want 12000 USD", bal)
	}
}

func TestClearedBalance(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(10000)}
	all := []domain.Transaction{
		{ID: "t1", AccountID: "a1", Amount: usd(5000), Cleared: true},
		{ID: "t2", AccountID: "a1", Amount: usd(-3000)}, // not cleared → excluded
		{ID: "t3", AccountID: "a1", Amount: usd(-1000), Cleared: true},
		{ID: "t4", AccountID: "other", Amount: usd(9999), Cleared: true}, // other account → ignored
	}
	cleared, err := ClearedBalance(acc, all)
	if err != nil {
		t.Fatalf("ClearedBalance error: %v", err)
	}
	// 10000 + 5000 - 1000 = 14000 (the uncleared -3000 is excluded).
	if !cleared.Equal(usd(14000)) {
		t.Errorf("ClearedBalance = %v, want 14000 USD", cleared)
	}
	// Full balance includes the uncleared txn: 11000.
	full, _ := Balance(acc, all[:3])
	if !full.Equal(usd(11000)) {
		t.Errorf("Balance = %v, want 11000 USD", full)
	}
}

func TestBalanceZeroOpening(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD"} // no opening balance
	bal, err := Balance(acc, []domain.Transaction{{AccountID: "a1", Amount: usd(250)}})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !bal.Equal(usd(250)) {
		t.Errorf("Balance = %v, want 250 USD", bal)
	}
}

func TestBalanceCurrencyMismatch(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(100)}
	all := []domain.Transaction{{AccountID: "a1", Amount: money.New(100, "EUR")}}
	if _, err := Balance(acc, all); err == nil {
		t.Error("expected currency mismatch error")
	}

	bad := domain.Account{ID: "a2", Currency: "USD", OpeningBalance: money.New(100, "EUR")}
	if _, err := Balance(bad, nil); err == nil {
		t.Error("expected opening-balance currency mismatch error")
	}
}

func TestRunningBalances(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(0)}
	ordered := []domain.Transaction{
		{AccountID: "a1", Amount: usd(100)},
		{AccountID: "a1", Amount: usd(50)},
		{AccountID: "a1", Amount: usd(-30)},
	}
	got, err := RunningBalances(acc, ordered)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := []money.Money{usd(100), usd(150), usd(120)}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("step %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPeriodTotals(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	mk := func(amount int64, cur, day string, transfer bool) domain.Transaction {
		tx := domain.Transaction{Date: mustDate(day), Amount: money.New(amount, cur)}
		if transfer {
			tx.TransferAccountID = "x"
		}
		return tx
	}
	all := []domain.Transaction{
		mk(20000, "USD", "2026-06-05", false), // +200 income
		mk(-5000, "USD", "2026-06-10", false), // -50 expense
		mk(10000, "EUR", "2026-06-12", false), // +100 EUR -> +110 income
		mk(-9999, "USD", "2026-06-20", true),  // transfer, ignored
		mk(-7000, "USD", "2026-07-01", false), // out of range, ignored
	}
	start, end := dateutil.MonthRange(mustDate("2026-06-15"))
	income, expense, err := PeriodTotals(all, start, end, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !income.Equal(usd(31000)) { // 200 + 110 = 310.00
		t.Errorf("income = %v, want 31000 USD", income)
	}
	if !expense.Equal(usd(5000)) { // 50.00
		t.Errorf("expense = %v, want 5000 USD", expense)
	}
}

func TestNetWorthSeries(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	accounts := []domain.Account{
		{ID: "a", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(10000)}, // $100 opening
	}
	all := []domain.Transaction{
		{AccountID: "a", Date: mustDate("2026-01-15"), Amount: usd(5000)},  // +$50 in Jan
		{AccountID: "a", Date: mustDate("2026-02-20"), Amount: usd(-2000)}, // -$20 in Feb
	}
	// Cutoffs at the first of Feb, Mar, Apr → end-of-Jan, end-of-Feb, end-of-Mar.
	cutoffs := []time.Time{mustDate("2026-02-01"), mustDate("2026-03-01"), mustDate("2026-04-01")}
	got, err := NetWorthSeries(accounts, all, cutoffs, rates)
	if err != nil {
		t.Fatalf("NetWorthSeries: %v", err)
	}
	want := []money.Money{usd(15000), usd(13000), usd(13000)} // $150, $130, $130
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("series[%d] = %s, want %s", i, got[i].Format(2), want[i].Format(2))
		}
	}
}

func TestNetWorth(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.20}}
	accounts := []domain.Account{
		{ID: "sav", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(100000)},             // +1000
		{ID: "eur", Class: domain.ClassAsset, Currency: "EUR", OpeningBalance: money.New(50000, "EUR")}, // 500 EUR -> 600 USD
		{ID: "cc", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-20000)},          // owe 200
		{ID: "old", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(99999), Archived: true},
	}
	net, assets, liabilities, err := NetWorth(accounts, nil, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !assets.Equal(usd(160000)) {
		t.Errorf("assets = %v, want 160000 USD", assets)
	}
	if !liabilities.Equal(usd(20000)) {
		t.Errorf("liabilities = %v, want 20000 USD", liabilities)
	}
	if !net.Equal(usd(140000)) {
		t.Errorf("net = %v, want 140000 USD", net)
	}
}

func TestNetByOwner(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	accounts := []domain.Account{
		{ID: "a", OwnerID: "m1", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(50000)},
		{ID: "b", OwnerID: "m1", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-10000)},
		{ID: "c", OwnerID: domain.GroupOwnerID, Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(30000)},
	}
	got, err := NetByOwner(accounts, nil, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !got["m1"].Equal(usd(40000)) {
		t.Errorf("m1 net = %v, want 40000 USD", got["m1"])
	}
	if !got[domain.GroupOwnerID].Equal(usd(30000)) {
		t.Errorf("group net = %v, want 30000 USD", got[domain.GroupOwnerID])
	}
}
