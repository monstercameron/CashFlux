// SPDX-License-Identifier: MIT

package assistantdata

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
)

func financeFixture() FinanceInput {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	in := FinanceInput{Now: now, Base: "USD", Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, WeekStart: time.Monday,
		Accounts: []domain.Account{
			{ID: "cash", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")},
			{ID: "euro", Name: "Euro cash", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "EUR", OpeningBalance: money.New(2000, "EUR")},
			{ID: "stock", Name: "Stock", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: money.New(1000000, "USD")},
		}, Categories: []domain.Category{{ID: "food", Name: "Groceries"}, {ID: "rent", Name: "Rent"}, {ID: "empty", Name: "Empty"}}}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	in.Transactions = []domain.Transaction{
		{ID: "salary", AccountID: "cash", Date: start, Amount: money.New(40000, "USD")},
		{ID: "buy", AccountID: "cash", Date: start, CategoryID: "food", Amount: money.New(-10000, "USD")},
		{ID: "refund", AccountID: "cash", Date: start.AddDate(0, 1, 0), CategoryID: "food", Amount: money.New(4000, "USD")},
		{ID: "split", AccountID: "cash", Date: start, Amount: money.New(-3000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-1000, "USD")}, {CategoryID: "rent", Amount: money.New(-2000, "USD")}}},
		{ID: "foreign", AccountID: "euro", Date: start, CategoryID: "food", Amount: money.New(-500, "EUR")},
		{ID: "excluded", AccountID: "cash", Date: start, CategoryID: "food", ExcludeFromReports: true, Amount: money.New(-900, "USD")},
		{ID: "transfer", AccountID: "cash", Date: start, CategoryID: "food", TransferAccountID: "stock", Amount: money.New(-1000, "USD")},
		{ID: "future", AccountID: "cash", Date: start.AddDate(0, 1, 0), CategoryID: "food", Amount: money.New(-2000, "USD")},
	}
	in.TxnLinks = []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}}}
	return in
}

func TestFinanceUsesScopedRefundSplitAndExactWindowFacts(t *testing.T) {
	in := financeFixture()
	f := BuildFinance(in)
	w, err := f.ResolveWindow("this_month", "", "")
	if err != nil {
		t.Fatal(err)
	}
	totals := f.Totals(w)
	if !totals.Known || totals.Income.Amount != 40000 || totals.Expense.Amount != 10000 || totals.Net.Amount != 30000 || totals.SavingsPercent != 75 {
		t.Fatalf("totals=%+v", totals)
	}
	food, err := f.Category(w, "food")
	if err != nil || food.Amount.Amount != 8000 || food.Count != 3 {
		t.Fatalf("food=%+v %v", food, err)
	}
	empty, err := f.Category(w, "Empty")
	if err != nil || empty.Amount.Amount != 0 || empty.Count != 0 {
		t.Fatalf("known zero=%+v %v", empty, err)
	}
	if _, err = f.Category(w, "mystery"); err == nil {
		t.Fatal("unknown category became zero")
	}
	in.Scope = scope.ReportScope{AccountIDs: []string{"cash"}}
	scoped := BuildFinance(in)
	food, err = scoped.Category(w, "Groceries")
	if err != nil || food.Amount.Amount != 7000 || food.Count != 2 {
		t.Fatalf("scoped food=%+v %v", food, err)
	}
	if totals = scoped.Totals(w); !totals.Known || totals.Expense.Amount != 9000 {
		t.Fatalf("scoped totals=%+v", totals)
	}
	// Balance sheets stay raw and explicitly household-wide: excluded reporting
	// rows and transfers still affect assets, and the stock account stays present.
	if !reflect.DeepEqual(f.HouseholdNetWorth(), scoped.HouseholdNetWorth()) {
		t.Fatal("activity scope changed household balance sheet")
	}
	balances := scoped.AccountBalances()
	if len(balances) != 1 || balances[0].Err != nil || balances[0].Balance.Amount != 37100 {
		t.Fatalf("raw balance=%+v", balances)
	}
	if in.Transactions[1].Amount.Amount != -10000 {
		t.Fatal("preparation mutated source purchase")
	}
	// Either explicit day is inclusive at the input boundary; the resolved end
	// remains exclusive so September's refund/future charge do not leak in.
	w, err = scoped.ResolveWindow("", "2026-08-01", "2026-08-31")
	if err != nil || !w.End.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("window=%+v %v", w, err)
	}
}

func TestFinanceUnknownFXAndOverflowStayDistinctFromEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*FinanceInput)
		overflow bool
	}{
		{"missing FX", func(in *FinanceInput) { in.Rates.Rates = nil }, false},
		{"magnitude", func(in *FinanceInput) {
			in.Transactions = []domain.Transaction{{AccountID: "cash", Date: in.Now, CategoryID: "food", Amount: money.New(math.MinInt64, "USD")}}
			in.TxnLinks = nil
		}, true},
		{"sum", func(in *FinanceInput) {
			in.Transactions = []domain.Transaction{{AccountID: "cash", Date: in.Now, CategoryID: "food", Amount: money.New(-math.MaxInt64, "USD")}, {AccountID: "cash", Date: in.Now, CategoryID: "food", Amount: money.New(-1, "USD")}}
			in.TxnLinks = nil
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := financeFixture()
			tc.change(&in)
			f := BuildFinance(in)
			w, _ := f.ResolveWindow("this_month", "", "")
			totals := f.Totals(w)
			rows, err := f.Spending(w)
			if totals.Known || totals.Error == nil || err == nil || rows != nil {
				t.Fatalf("unknown claimed as zero: totals=%+v rows=%v err=%v", totals, rows, err)
			}
			if tc.overflow && (!errors.Is(totals.Error, money.ErrOverflow) || !errors.Is(err, money.ErrOverflow)) {
				t.Fatalf("lost overflow: %v %v", totals.Error, err)
			}
		})
	}
	in := financeFixture()
	in.Rates.Rates = nil
	f := BuildFinance(in)
	nw := f.HouseholdNetWorth()
	if !nw.Known || !nw.Partial || nw.Complete() || len(nw.MissingCurrencies) != 1 || nw.MissingCurrencies[0] != "EUR" {
		t.Fatalf("partial balance sheet=%+v", nw)
	}
	// A clean zero-activity window remains available even with no rows.
	w, err := f.ResolveWindow("", "2025-01-01", "2025-01-31")
	totals := f.Totals(w)
	if err != nil || !totals.Known || totals.Expense.Amount != 0 {
		t.Fatalf("empty=%+v %v", totals, err)
	}
}

func TestFinanceBudgetsUseCadenceCarryBoostAndUnavailableState(t *testing.T) {
	in := financeFixture()
	in.Scope = scope.ReportScope{AccountIDs: []string{"cash"}}
	in.Budgets = []domain.Budget{{ID: "food-plan", Name: "Food", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD"), Rollover: true, RolloverStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), PeriodBoosts: map[string]int64{"2026-08-01": 500}}}
	in.Transactions = append(in.Transactions, domain.Transaction{AccountID: "cash", CategoryID: "food", Date: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(-3000, "USD")})
	f := BuildFinance(in)
	w, _ := f.ResolveWindow("this_month", "", "")
	rows := f.Budgets(w)
	if len(rows) != 1 || !rows[0].Valid || rows[0].Status.Limit.Amount != 17500 || rows[0].Status.Spent.Amount != 7000 || rows[0].Status.Remaining.Amount != 10500 || rows[0].Budget.Limit.Amount != 10000 {
		t.Fatalf("carry/boost/scope=%+v", rows)
	}
	if !rows[0].Start.Equal(w.Start) || !rows[0].End.Equal(w.End) {
		t.Fatalf("cadence=%v..%v", rows[0].Start, rows[0].End)
	}
	in.Transactions = append(in.Transactions, domain.Transaction{AccountID: "cash", CategoryID: "food", Date: in.Now, Amount: money.New(-1, "JPY")})
	rows = BuildFinance(in).Budgets(w)
	if len(rows) != 1 || rows[0].Valid || rows[0].Status.State != budgeting.StateUnavailable || rows[0].Error == nil {
		t.Fatalf("failed budget hidden or valid=%+v", rows)
	}
}

func TestFinanceAffordabilityUsesLiquidCashAndCheckedProjection(t *testing.T) {
	in := financeFixture()
	in.TxnLinks = nil
	in.Scope = scope.ReportScope{AccountIDs: []string{"cash", "stock"}}
	in.Accounts[0].OpeningBalance = money.New(1000, "USD")
	in.Transactions = []domain.Transaction{{AccountID: "cash", Date: in.Now, Amount: money.New(1000, "USD")}, {AccountID: "cash", Date: in.Now, Amount: money.New(-200, "USD")}}
	f := BuildFinance(in)
	got := f.Affordability(money.New(3000, "USD"), money.New(500, "USD"), 2)
	if !got.Known || got.Liquid.Amount != 1800 || got.MonthlyNet.Amount != 800 || got.Result.ProjectedBalance != 3400 || got.Result.Available != 2900 || got.Result.Affordable || got.Result.Shortfall != 100 {
		t.Fatalf("liquid projection=%+v", got)
	}
	got = f.Affordability(money.New(100000, "USD"), money.Zero("USD"), 0)
	if !got.Known || got.Result.Affordable {
		t.Fatalf("investment treated as spendable=%+v", got)
	}
	// Restricted cash and liabilities cannot become a purchase source merely
	// because they use a cash-like account type. Their flows are excluded too.
	in.Accounts[0].LockUntil = in.Now.AddDate(0, 1, 0)
	locked := BuildFinance(in).Affordability(money.New(1, "USD"), money.Zero("USD"), 1)
	if !locked.Known || locked.Liquid.Amount != 0 || locked.MonthlyNet.Amount != 0 || locked.Result.Affordable {
		t.Fatalf("locked source=%+v", locked)
	}
	in.Accounts[0].LockUntil = time.Time{}
	in.Accounts[0].Class = domain.ClassLiability
	debt := BuildFinance(in).Affordability(money.New(1, "USD"), money.Zero("USD"), 1)
	if !debt.Known || debt.Liquid.Amount != 0 || debt.MonthlyNet.Amount != 0 || debt.Result.Affordable {
		t.Fatalf("liability source=%+v", debt)
	}
	in.Accounts[0].Class = domain.ClassAsset
	in.Transactions[0].Amount = money.New(math.MaxInt64, "USD")
	in.Transactions = in.Transactions[:1]
	in.Accounts[0].OpeningBalance = money.Zero("USD")
	got = BuildFinance(in).Affordability(money.New(1, "USD"), money.Zero("USD"), 2)
	if got.Known || !errors.Is(got.Err, money.ErrOverflow) {
		t.Fatalf("wrapped projection=%+v", got)
	}
}

func TestFinanceExactDecimalParsingAndInvalidWindows(t *testing.T) {
	for _, tc := range []struct {
		raw, code string
		want      int64
	}{
		{"92233720368547758.07", "USD", math.MaxInt64}, {"9007199254740993", "JPY", 9007199254740993}, {"1.23", "EUR", 123},
	} {
		got, err := ParseAmount(tc.raw, tc.code)
		if err != nil || got.Amount != tc.want {
			t.Fatalf("parse %s %s =%+v %v", tc.raw, tc.code, got, err)
		}
	}
	for _, tc := range []struct{ raw, code string }{{"1.001", "USD"}, {"1.1", "JPY"}, {"92233720368547758.08", "USD"}, {"1e3", "USD"}, {"NaN", "USD"}, {"1", "XXX"}} {
		if _, err := ParseAmount(tc.raw, tc.code); err == nil {
			t.Fatalf("accepted %v", tc)
		}
	}
	f := BuildFinance(financeFixture())
	for _, tc := range []struct{ period, from, to string }{{"unknown", "", ""}, {"", "not-a-date", "2026-08-31"}, {"", "2026-09-01", "2026-08-31"}} {
		if _, err := f.ResolveWindow(tc.period, tc.from, tc.to); err == nil {
			t.Fatalf("accepted window=%v", tc)
		}
	}
}
