// SPDX-License-Identifier: MIT

package reportview

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
)

func day(month time.Month, d int) time.Time { return time.Date(2026, month, d, 0, 0, 0, 0, time.UTC) }

func TestBuildSeriesScopeAndHouseholdCapacity(t *testing.T) {
	accounts := []domain.Account{
		{ID: "mine", OwnerID: "me", Currency: "USD"},
		{ID: "theirs", OwnerID: "other", Currency: "EUR"},
		{ID: "closed", Archived: true, Currency: "USD"},
	}
	txns := []domain.Transaction{
		{AccountID: "mine", Date: day(time.June, 2), Amount: money.New(300000, "USD")},
		{AccountID: "mine", Date: day(time.June, 3), Amount: money.New(-100000, "USD")},
		{AccountID: "theirs", Date: day(time.June, 2), Amount: money.New(200000, "EUR")},
		{AccountID: "theirs", Date: day(time.June, 3), Amount: money.New(-50000, "EUR")},
		{AccountID: "closed", Date: day(time.June, 3), Amount: money.New(999999, "USD")},
		{AccountID: "mine", Date: day(time.July, 1), Amount: money.New(999999, "USD")},
	}
	bounds := []time.Time{day(time.June, 1), day(time.July, 1)}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	all := BuildSeries(accounts, txns, scope.ReportScope{}, bounds, rates)
	for _, sc := range []scope.ReportScope{{Owners: []string{"me"}}, {AccountIDs: []string{"mine"}}, {AccountIDs: []string{"missing"}}} {
		got := BuildSeries(accounts, txns, sc, bounds, rates)
		if got.FlowError != nil || got.HouseholdError != nil {
			t.Fatalf("unexpected errors: %v / %v", got.FlowError, got.HouseholdError)
		}
		if !reflect.DeepEqual(got.HouseholdFlows, all.MonthFlows) {
			t.Fatalf("scope %v changed household capacity: %v vs %v", sc, got.HouseholdFlows, all.MonthFlows)
		}
		if len(got.Accounts) > 0 && (got.Flow.Income != 300000 || got.Flow.Expense != 100000 || got.SavingsRates[0] != 66) {
			t.Fatalf("scope %v flow = %+v, savings %v", sc, got.Flow, got.SavingsRates)
		}
	}
	if all.Flow.Income != 700000 || all.Flow.Expense != 200000 {
		t.Fatalf("household base flow = %+v", all.Flow)
	}
}

func TestBuildSeriesReportsFXFailureSeparately(t *testing.T) {
	accounts := []domain.Account{{ID: "usd", Currency: "USD"}, {ID: "eur", Currency: "EUR"}}
	txns := []domain.Transaction{{AccountID: "eur", Date: day(time.June, 3), Amount: money.New(-100, "EUR")}}
	got := BuildSeries(accounts, txns, scope.ReportScope{AccountIDs: []string{"usd"}}, []time.Time{day(time.June, 1), day(time.July, 1)}, currency.Rates{Base: "USD"})
	if got.FlowError != nil || got.HouseholdError == nil {
		t.Fatalf("scoped/household errors = %v / %v", got.FlowError, got.HouseholdError)
	}
}

func TestBuildSeriesPreservesWholeWindowOverflowChecks(t *testing.T) {
	accounts := []domain.Account{{ID: "usd", Currency: "USD"}}
	txns := []domain.Transaction{
		{AccountID: "usd", Date: day(time.June, 3), Amount: money.New(math.MaxInt64, "USD")},
		{AccountID: "usd", Date: day(time.July, 3), Amount: money.New(1, "USD")},
	}
	got := BuildSeries(accounts, txns, scope.ReportScope{}, []time.Time{day(time.June, 1), day(time.July, 1), day(time.August, 1)}, currency.Rates{Base: "USD"})
	if !errors.Is(got.FlowError, money.ErrOverflow) || got.Flow.Income != 0 || got.Flow.Expense != 0 {
		t.Fatalf("whole-window overflow produced a valid headline: %+v", got)
	}
}

func TestBuildPositionUsesBaseInterestAndHouseholdFlows(t *testing.T) {
	accounts := []domain.Account{
		{ID: "cash", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(900000, "USD")},
		{ID: "eur", Name: "Euro loan", Class: domain.ClassLiability, Currency: "EUR", OpeningBalance: money.New(-100000, "EUR"), InterestRateAPR: domain.APR(12), MinPayment: money.New(5000, "EUR")},
		{ID: "usd", Name: "Dollar card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(-200000, "USD"), CreditLimit: money.New(400000, "USD"), InterestRateAPR: domain.APR(18), MinPayment: money.New(6000, "USD")},
		{ID: "jpy", Name: "Yen loan", Class: domain.ClassLiability, Currency: "JPY", OpeningBalance: money.New(-100000, "JPY"), InterestRateAPR: domain.APR(6)},
		{ID: "free", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(-999999, "USD"), InterestRateAPR: domain.APR(0)},
		{ID: "unknown", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(-999999, "USD")},
		{ID: "archived", Archived: true, Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(-999999, "USD"), InterestRateAPR: domain.APR(99)},
	}
	flows := make([]reports.PeriodFlow, 8)
	flows[0] = reports.PeriodFlow{Income: 99999999, Expense: 99999999}
	for i := 2; i < len(flows); i++ {
		flows[i] = reports.PeriodFlow{Income: 600000, Expense: 300000}
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2, "JPY": 0.01}}
	got := BuildPosition(accounts, nil, flows, rates, 150000)
	if got.MonthsAveraged != 6 || got.Vitals.IncomeMonthlyMinor != 600000 || got.Vitals.ExpenseMonthlyMinor != 300000 {
		t.Fatalf("household capacity = %+v", got.Vitals)
	}
	// $360 + €120 → $240 + ¥6000 → $60 = $660, in USD cents.
	if got.AnnualInterestMinor != 66000 || len(got.Debts) != 3 {
		t.Fatalf("interest = %d, rows %+v", got.AnnualInterestMinor, got.Debts)
	}
	if got.Debts[0].Name != "Dollar card" || got.Debts[1].Balance.Currency != "EUR" || got.Debts[1].AnnualInterestMinor != 24000 {
		t.Fatalf("native/base debt rows = %+v", got.Debts)
	}
}

func TestCreditSeriesMatchesStandaloneCutoffs(t *testing.T) {
	cards := []domain.Account{
		{ID: "card", Currency: "USD", Type: domain.TypeCreditCard, OpeningBalance: money.New(-40000, "USD"), CreditLimit: money.New(100000, "USD"), BalanceAsOf: day(time.January, 1), DueDayOfMonth: 10},
		{ID: "closed", Archived: true, Currency: "USD", Type: domain.TypeCreditCard, OpeningBalance: money.New(-99999, "USD")},
	}
	txns := []domain.Transaction{
		{AccountID: "card", Date: day(time.July, 1), Amount: money.New(-30000, "USD")},
		{AccountID: "card", Date: day(time.June, 10), Amount: money.New(10000, "USD")},
		{AccountID: "other", Date: day(time.June, 4), Amount: money.New(-50000, "EUR")},
	}
	bounds := []time.Time{day(time.June, 1), day(time.July, 1), day(time.August, 1)}
	got := CreditSeries(cards, txns, bounds)
	for i, cutoff := range bounds[1:] {
		var upto []domain.Transaction
		for _, txn := range txns {
			if txn.Date.Before(cutoff) {
				upto = append(upto, txn)
			}
		}
		bal, err := ledger.Balance(cards[0], upto)
		if err != nil {
			t.Fatal(err)
		}
		want := credithealth.Evaluate(credithealth.Inputs{Accounts: cards, Transactions: upto, Balances: map[string]int64{"card": bal.Amount}, Now: cutoff})
		if got[i] != want.ProxyScore {
			t.Fatalf("cutoff %s score = %d, want %d", cutoff, got[i], want.ProxyScore)
		}
	}
	if got := CreditSeries(cards[1:], txns, bounds); got != nil {
		t.Fatalf("archived-only credit series = %v", got)
	}
}
