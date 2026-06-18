package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
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

func TestAdjustmentToTarget(t *testing.T) {
	adj, ok := AdjustmentToTarget(usd(14000), 12500)
	if !ok {
		t.Fatal("expected an adjustment")
	}
	if !adj.Equal(usd(-1500)) {
		t.Errorf("adjustment = %v, want -1500 USD", adj)
	}
	if none, ok := AdjustmentToTarget(usd(14000), 14000); ok || none != (money.Money{}) {
		t.Errorf("no-op adjustment = %v ok=%v, want zero/false", none, ok)
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

func TestTransferBalancesNetWorthAndTotals(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	checking := domain.Account{ID: "checking", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(100000)}
	savings := domain.Account{ID: "savings", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(50000)}
	txns := []domain.Transaction{
		{
			ID: "out", AccountID: "checking", TransferAccountID: "savings",
			Date: mustDate("2026-06-10"), Amount: usd(-25000),
		},
		{
			ID: "in", AccountID: "savings", TransferAccountID: "checking",
			Date: mustDate("2026-06-10"), Amount: usd(25000),
		},
	}

	checkingBal, err := Balance(checking, txns)
	if err != nil {
		t.Fatalf("checking Balance: %v", err)
	}
	if !checkingBal.Equal(usd(75000)) {
		t.Errorf("checking balance = %v, want 75000 USD", checkingBal)
	}
	savingsBal, err := Balance(savings, txns)
	if err != nil {
		t.Fatalf("savings Balance: %v", err)
	}
	if !savingsBal.Equal(usd(75000)) {
		t.Errorf("savings balance = %v, want 75000 USD", savingsBal)
	}

	net, assets, liabilities, err := NetWorth([]domain.Account{checking, savings}, txns, rates)
	if err != nil {
		t.Fatalf("NetWorth: %v", err)
	}
	if !net.Equal(usd(150000)) || !assets.Equal(usd(150000)) || !liabilities.Equal(usd(0)) {
		t.Errorf("net/assets/liabilities = %v/%v/%v, want 150000/150000/0 USD", net, assets, liabilities)
	}

	start, end := dateutil.MonthRange(mustDate("2026-06-15"))
	income, expense, err := PeriodTotals(txns, start, end, rates)
	if err != nil {
		t.Fatalf("PeriodTotals: %v", err)
	}
	if !income.Equal(usd(0)) || !expense.Equal(usd(0)) {
		t.Errorf("transfer totals = income %v expense %v, want both zero", income, expense)
	}
}

func TestCategorySpendSeries(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	tx := func(cat string, amount int64, cur, day string, transfer bool) domain.Transaction {
		t := domain.Transaction{CategoryID: cat, Date: mustDate(day), Amount: money.New(amount, cur)}
		if transfer {
			t.TransferAccountID = "x"
		}
		return t
	}
	all := []domain.Transaction{
		tx("food", -3000, "USD", "2026-04-10", false), // Apr food 30
		tx("food", -2000, "USD", "2026-04-25", false), // Apr food 20 (same period → 50)
		tx("food", -4000, "USD", "2026-05-05", false), // May food 40
		tx("food", -9000, "USD", "2026-06-15", false), // Jun food 90
		tx("gas", -1000, "EUR", "2026-06-02", false),  // Jun gas 10 EUR -> 11 USD
		tx("food", 5000, "USD", "2026-06-20", false),  // income, ignored
		tx("food", -7777, "USD", "2026-06-09", true),  // transfer, ignored
		tx("food", -1234, "USD", "2026-03-01", false), // before window, ignored
		tx("", -2500, "USD", "2026-05-12", false),     // uncategorized May 25
	}
	// Three monthly periods: Apr, May, Jun.
	bounds := []time.Time{
		mustDate("2026-04-01"), mustDate("2026-05-01"), mustDate("2026-06-01"), mustDate("2026-07-01"),
	}
	got, err := CategorySpendSeries(all, bounds, rates)
	if err != nil {
		t.Fatalf("CategorySpendSeries: %v", err)
	}
	want := map[string][]int64{
		"food": {5000, 4000, 9000}, // Apr 50, May 40, Jun 90
		"gas":  {0, 0, 1100},       // only Jun, EUR converted
		"":     {0, 2500, 0},       // uncategorized in May
	}
	if len(got) != len(want) {
		t.Fatalf("got %d categories, want %d (%v)", len(got), len(want), got)
	}
	for cat, w := range want {
		g := got[cat]
		if len(g) != len(w) {
			t.Fatalf("category %q: len %d, want %d", cat, len(g), len(w))
		}
		for i := range w {
			if g[i] != w[i] {
				t.Errorf("category %q period %d = %d, want %d", cat, i, g[i], w[i])
			}
		}
	}
}

func TestCategorySpendSeriesTooFewBounds(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	got, err := CategorySpendSeries(nil, []time.Time{mustDate("2026-06-01")}, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty for a single bound", got)
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

func TestFXAggregatesRecomputeWithRateChange(t *testing.T) {
	accounts := []domain.Account{
		{ID: "checking", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(100000)},
		{ID: "travel", Class: domain.ClassAsset, Currency: "EUR", OpeningBalance: money.New(10000, "EUR")},
	}
	txns := []domain.Transaction{
		{
			ID: "eur-expense", AccountID: "travel", CategoryID: "food",
			Amount: money.New(-2000, "EUR"), Date: mustDate("2026-06-04"),
		},
		{
			ID: "eur-income", AccountID: "travel",
			Amount: money.New(5000, "EUR"), Date: mustDate("2026-06-05"),
		},
	}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(100000)}
	start, end := dateutil.MonthRange(mustDate("2026-06-15"))

	assertAggregates := func(t *testing.T, rates currency.Rates, wantNet, wantIncome, wantExpense, wantSpent money.Money) {
		t.Helper()
		net, _, _, err := NetWorth(accounts, txns, rates)
		if err != nil {
			t.Fatalf("NetWorth: %v", err)
		}
		income, expense, err := PeriodTotals(txns, start, end, rates)
		if err != nil {
			t.Fatalf("PeriodTotals: %v", err)
		}
		spent, err := budgeting.Spent(budget, txns, start, end, rates)
		if err != nil {
			t.Fatalf("Spent: %v", err)
		}
		if !net.Equal(wantNet) || !income.Equal(wantIncome) || !expense.Equal(wantExpense) || !spent.Equal(wantSpent) {
			t.Fatalf("net/income/expense/spent = %v/%v/%v/%v, want %v/%v/%v/%v",
				net, income, expense, spent, wantNet, wantIncome, wantExpense, wantSpent)
		}
	}

	assertAggregates(t, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.20}},
		usd(115600), usd(6000), usd(2400), usd(2400))
	assertAggregates(t, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.50}},
		usd(119500), usd(7500), usd(3000), usd(3000))
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

func TestNetWorthRollupsMultiMemberMultiCurrencyArchived(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.20}}
	accounts := []domain.Account{
		{ID: "m1save", OwnerID: "m1", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(100000)},
		{ID: "m2eur", OwnerID: "m2", Class: domain.ClassAsset, Currency: "EUR", OpeningBalance: money.New(50000, "EUR")},
		{ID: "group", OwnerID: domain.GroupOwnerID, Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(25000)},
		{ID: "m1card", OwnerID: "m1", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-30000)},
		{ID: "old", OwnerID: "m2", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(999999), Archived: true},
	}
	txns := []domain.Transaction{
		{AccountID: "m1save", Amount: usd(10000)},
		{AccountID: "m2eur", Amount: money.New(10000, "EUR")},
		{AccountID: "old", Amount: usd(999999)},
	}

	net, assets, liabilities, err := NetWorth(accounts, txns, rates)
	if err != nil {
		t.Fatalf("NetWorth error: %v", err)
	}
	if !assets.Equal(usd(207000)) {
		t.Errorf("assets = %v, want 207000 USD", assets)
	}
	if !liabilities.Equal(usd(30000)) {
		t.Errorf("liabilities = %v, want 30000 USD", liabilities)
	}
	if !net.Equal(usd(177000)) {
		t.Errorf("net = %v, want 177000 USD", net)
	}

	byOwner, err := NetByOwner(accounts, txns, rates)
	if err != nil {
		t.Fatalf("NetByOwner error: %v", err)
	}
	want := map[string]money.Money{
		"m1":                usd(80000),
		"m2":                usd(72000),
		domain.GroupOwnerID: usd(25000),
	}
	for owner, w := range want {
		if !byOwner[owner].Equal(w) {
			t.Errorf("owner %q = %v, want %v", owner, byOwner[owner], w)
		}
	}
	if _, ok := byOwner["old"]; ok {
		t.Error("archived account owner should not appear")
	}
}

func TestNetWorthRollupsSumToHouseholdAndRestoreArchived(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.20}}
	accounts := []domain.Account{
		{ID: "m1asset", OwnerID: "m1", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(100000)},
		{ID: "m2asset", OwnerID: "m2", Class: domain.ClassAsset, Currency: "EUR", OpeningBalance: money.New(50000, "EUR")},
		{ID: "shared", OwnerID: domain.GroupOwnerID, Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(25000)},
		{ID: "m1debt", OwnerID: "m1", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-30000)},
		{ID: "restored", OwnerID: "m2", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(40000), Archived: true},
	}

	net, assets, liabilities, err := NetWorth(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetWorth archived: %v", err)
	}
	if got, err := assets.Sub(liabilities); err != nil || !got.Equal(net) {
		t.Fatalf("assets - liabilities = %v/%v, want net %v", got, err, net)
	}
	byOwner, err := NetByOwner(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetByOwner archived: %v", err)
	}
	sum := money.Zero("USD")
	for owner, bal := range byOwner {
		var err error
		sum, err = sum.Add(bal)
		if err != nil {
			t.Fatalf("sum owner %q: %v", owner, err)
		}
	}
	if !sum.Equal(net) {
		t.Fatalf("owner rollups sum = %v, want household net %v", sum, net)
	}

	accounts[4].Archived = false
	restoredNet, _, _, err := NetWorth(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetWorth restored: %v", err)
	}
	if !restoredNet.Equal(usd(195000)) {
		t.Fatalf("restored net = %v, want 195000 USD", restoredNet)
	}
	restoredByOwner, err := NetByOwner(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetByOwner restored: %v", err)
	}
	if !restoredByOwner["m2"].Equal(usd(100000)) {
		t.Fatalf("m2 restored rollup = %v, want 100000 USD", restoredByOwner["m2"])
	}
}

func TestUtilization(t *testing.T) {
	cases := []struct {
		name           string
		balance, limit int64
		wantPct        int
		wantOK         bool
	}{
		{"no limit is not ok", -500, 0, 0, false},
		{"negative limit is not ok", -500, -100, 0, false},
		{"owed (negative balance) 50pct", -5000, 10000, 50, true},
		{"positive balance magnitude", 5000, 10000, 50, true},
		{"zero owed", 0, 10000, 0, true},
		{"over limit", -12000, 10000, 120, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pct, ok := Utilization(c.balance, c.limit)
			if pct != c.wantPct || ok != c.wantOK {
				t.Errorf("Utilization(%d, %d) = (%d, %v), want (%d, %v)", c.balance, c.limit, pct, ok, c.wantPct, c.wantOK)
			}
		})
	}
}

func TestSavingsRate(t *testing.T) {
	cases := []struct {
		name            string
		income, expense int64
		want            int
	}{
		{"no income is zero", 0, 500, 0},
		{"negative income is zero", -100, 50, 0},
		{"saved 20pct", 1000, 800, 20},
		{"overspent is negative", 1000, 1200, -20},
		{"spent nothing is 100", 1000, 0, 100},
		{"truncates toward zero", 300, 100, 66},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SavingsRate(c.income, c.expense); got != c.want {
				t.Errorf("SavingsRate(%d, %d) = %d, want %d", c.income, c.expense, got, c.want)
			}
		})
	}
}

func TestPercentChange(t *testing.T) {
	cases := []struct {
		name       string
		curr, prev int64
		wantPct    int64
		wantOK     bool
	}{
		{"zero baseline has no change", 500, 0, 0, false},
		{"increase", 150, 100, 50, true},
		{"decrease", 50, 100, -50, true},
		{"no movement", 100, 100, 0, true},
		{"negative baseline improving is positive", -50, -100, 50, true},
		{"negative baseline worsening is negative", -150, -100, -50, true},
		{"crossing zero upward", 50, -100, 150, true},
		{"truncates toward zero", 133, 100, 33, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pct, ok := PercentChange(c.curr, c.prev)
			if ok != c.wantOK || pct != c.wantPct {
				t.Errorf("PercentChange(%d, %d) = (%d, %v), want (%d, %v)", c.curr, c.prev, pct, ok, c.wantPct, c.wantOK)
			}
		})
	}
}
