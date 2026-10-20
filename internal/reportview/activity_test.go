// SPDX-License-Identifier: MIT

package reportview

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
)

func TestActivityMatchesIndependentReportFunctionsUnderScope(t *testing.T) {
	accounts := []domain.Account{
		{ID: "mine", Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(50000, "USD")},
		{ID: "foreign", Type: domain.TypeChecking, Currency: "EUR", OpeningBalance: money.New(50000, "EUR")},
		{ID: "theirs", Type: domain.TypeChecking, Currency: "EUR", OpeningBalance: money.New(50000, "EUR")},
	}
	txns := []domain.Transaction{
		{ID: "prior", AccountID: "mine", Date: day(time.May, 3), CategoryID: "food", Desc: "Shop", Amount: money.New(-1000, "USD")},
		{ID: "income", AccountID: "mine", Date: day(time.June, 2), CategoryID: "salary", Desc: "Pay", Amount: money.New(50000, "USD")},
		{ID: "purchase", AccountID: "foreign", Date: day(time.June, 3), CategoryID: "food", Desc: "Shop", Amount: money.New(-6000, "EUR"), Tags: []string{"Trip"}},
		{ID: "refund", AccountID: "foreign", Date: day(time.July, 3), CategoryID: "food", Desc: "Return", Amount: money.New(2000, "EUR")},
		{ID: "split", AccountID: "mine", Date: day(time.July, 4), Desc: "Split", Amount: money.New(-4000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-1000, "USD")}, {CategoryID: "other", Amount: money.New(-3000, "USD")}}},
		{ID: "fee", AccountID: "mine", Date: day(time.June, 5), Desc: "Bank fee", Amount: money.New(-100, "USD")},
		{ID: "excluded", AccountID: "mine", Date: day(time.June, 6), Desc: "Hidden", Amount: money.New(-999999, "USD"), ExcludeFromReports: true},
		{ID: "transfer", AccountID: "mine", TransferAccountID: "theirs", Date: day(time.June, 6), Amount: money.New(-100, "USD")},
		{ID: "outside-scope", AccountID: "theirs", Date: day(time.June, 6), Desc: "Other", Amount: money.New(-20000, "EUR")},
		{ID: "end", AccountID: "mine", Date: day(time.August, 1), Desc: "Exclusive", Amount: money.New(-999999, "USD")},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2, "GBP": 1.5}}
	links := []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}, Amount: money.New(2000, "EUR")}}
	reports.SetRefundLinks(links)
	defer reports.SetRefundLinks(nil)
	bounds := []time.Time{day(time.June, 1), day(time.July, 1), day(time.August, 1)}
	series := BuildSeries(accounts, txns, scope.ReportScope{AccountIDs: []string{"mine", "foreign"}}, bounds, rates)
	in := ActivityInput{Series: series, Bounds: bounds, PriorStart: day(time.May, 1), PriorEnd: day(time.June, 1), Rates: rates, HouseholdAccounts: accounts, Now: day(time.July, 20)}
	got, err := BuildActivity(in)
	if err != nil {
		t.Fatal(err)
	}
	assertSame := func(name string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s mismatch: got %#v, want %#v", name, got, want)
		}
	}
	selected := series.Transactions
	wantCategories, _ := reports.SpendingByCategory(selected, bounds[0], bounds[2], true, in.PriorStart, in.PriorEnd, rates)
	assertSame("categories", got.Spending, wantCategories)
	wantTrends, _ := reports.CategoryTrends(selected, bounds, rates)
	assertSame("category trends", got.CategoryTrends, wantTrends)
	wantTags, _ := reports.SpendingByTag(selected, bounds[0], bounds[2], true, in.PriorStart, in.PriorEnd, rates)
	assertSame("tags", got.Tags, wantTags)
	wantExpenses, _ := reports.LargestExpenses(selected, bounds[0], bounds[2], rates, 10)
	assertSame("individual expense semantics", got.LargestExpenses, wantExpenses)
	wantStats, _ := reports.SpendingStats(selected, bounds[0], bounds[2], rates)
	assertSame("statistics", got.SpendStats, wantStats)
	wantNW, _ := ledger.NetWorthSeries(series.Accounts, selected, bounds, rates)
	assertSame("scoped net worth", got.NetWorthSeries, wantNW)
	wantCosts, _ := reports.CostOfMoney(selected, nil, bounds[0], bounds[2], rates)
	assertSame("costs", got.Costs, wantCosts)
	if got.NoSpendDays != reports.NoSpendDays(selected, bounds[0], bounds[2], in.Now) || txns[2].Amount.Amount != -6000 || txns[4].Splits[0].Amount.Amount != -1000 {
		t.Fatal("clock mismatch or input mutation")
	}
	again, err := BuildActivity(in)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("repeated activity calculation changed results")
	}
}

func TestActivitySubscriptionConfirmationPlanningAndClockAreExplicit(t *testing.T) {
	accounts := []domain.Account{{ID: "cash", Currency: "USD", Type: domain.TypeChecking}}
	var txns []domain.Transaction
	for _, month := range []time.Month{time.March, time.April, time.May} {
		txns = append(txns, domain.Transaction{AccountID: "cash", Date: day(month, 2), Desc: "Netflix", Amount: money.New(-1000, "USD")})
	}
	bounds := []time.Time{day(time.March, 1), day(time.April, 1), day(time.May, 1), day(time.June, 1)}
	rates := currency.Rates{Base: "USD"}
	in := ActivityInput{Series: BuildSeries(accounts, txns, scope.ReportScope{}, bounds, rates), Bounds: bounds, Rates: rates, HouseholdAccounts: accounts, Now: day(time.May, 10)}
	review, err := BuildActivity(in)
	if err != nil || len(review.Subscriptions) != 0 {
		t.Fatalf("unconfirmed review pattern included: %+v / %v", review.Subscriptions, err)
	}
	in.ConfirmedSubs = map[string]bool{"netflix": true}
	confirmed, err := BuildActivity(in)
	if err != nil || len(confirmed.Subscriptions) != 1 || confirmed.SubscriptionAnnualMinor != 12000 {
		t.Fatalf("confirmation ignored: %+v / %v", confirmed.Subscriptions, err)
	}
	in.Recurring = []domain.Recurring{{Label: " Netflix "}}
	planned, _ := BuildActivity(in)
	if len(planned.Subscriptions) != 0 {
		t.Fatal("planned recurring charge counted as cancellable")
	}
	in.Recurring = nil
	in.Now = day(time.September, 1)
	lapsed, _ := BuildActivity(in)
	if len(lapsed.Subscriptions) != 0 {
		t.Fatal("explicit clock did not lapse old detections")
	}
}

func TestActivitySurfacesSectionFXFailures(t *testing.T) {
	in := ActivityInput{Series: Series{Accounts: []domain.Account{{ID: "eur", Currency: "EUR"}}, Transactions: []domain.Transaction{{AccountID: "eur", Date: day(time.June, 2), Desc: "Missing rate", Amount: money.New(-100, "EUR")}}}, Bounds: []time.Time{day(time.June, 1), day(time.July, 1)}, Rates: currency.Rates{Base: "USD"}, Now: day(time.June, 3)}
	got, err := BuildActivity(in)
	if err == nil || got.Spending != nil || got.CategoryTrends != nil || got.LargestExpenses != nil {
		t.Fatalf("failed FX produced financial rows: %+v / %v", got, err)
	}
	if _, err := BuildActivity(ActivityInput{}); err != nil {
		t.Fatal(err)
	}
}
