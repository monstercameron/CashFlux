// SPDX-License-Identifier: MIT

package reports

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
)

func TestCheckedReportComparisonsAndTrendsKeepExactBounds(t *testing.T) {
	start := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	prior := start.AddDate(0, -1, 0)
	rows := budgeting.NettedLedger{
		{ID: "prior", Date: prior, CategoryID: "food", Amount: money.New(-100, "USD")},
		{ID: "before", Date: start.Add(-time.Nanosecond), CategoryID: "food", Amount: money.New(-9900, "USD")},
		{ID: "first", Date: start, CategoryID: "food", Amount: money.New(-200, "USD")},
		{ID: "second", Date: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), CategoryID: "food", Amount: money.New(-600, "USD")},
		{ID: "end", Date: end, CategoryID: "food", Amount: money.New(-9900, "USD")},
	}
	rates := currency.Rates{Base: "USD"}
	comparison, err := CategoryComparisonChecked(rows, start, end, true, prior, prior.AddDate(0, 0, 1), rates)
	if err != nil || len(comparison) != 1 || comparison[0].Amount != 800 || comparison[0].Prior != 100 || comparison[0].DeltaPct != 700 {
		t.Fatalf("comparison=%+v err=%v", comparison, err)
	}
	bounds, err := MonthlyBoundsChecked(AskedWindow{Start: start, End: end}, time.Time{}, 36)
	if err != nil {
		t.Fatal(err)
	}
	trend, err := CategoryTrendsChecked(rows, bounds, rates)
	if err != nil || len(trend) != 1 || trend[0].Total != 800 || !reflect.DeepEqual(trend[0].Spend, []int64{200, 600}) || trend[0].DeltaPct != 200 {
		t.Fatalf("trends=%+v err=%v", trend, err)
	}
}

func TestCheckedReportArithmeticDoesNotWrap(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rates := currency.Rates{Base: "USD"}
	if _, err := TotalChecked([]CategorySpend{{Amount: math.MaxInt64}, {Amount: 1}}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("total: %v", err)
	}
	rows := budgeting.NettedLedger{{Date: start, CategoryID: "a", Amount: money.New(-math.MaxInt64, "USD")}, {Date: end, CategoryID: "a", Amount: money.New(-math.MaxInt64, "USD")}}
	if _, err := CategoryTrendsChecked(rows, []time.Time{start, end, end.AddDate(0, 1, 0)}, rates); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("trend total: %v", err)
	}
	rows = budgeting.NettedLedger{{Date: start, CategoryID: "a", Amount: money.New(-1, "USD")}, {Date: end, CategoryID: "a", Amount: money.New(-math.MaxInt64, "USD")}}
	if _, err := CategoryComparisonChecked(rows, end, end.AddDate(0, 1, 0), true, start, end, rates); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("change percentage: %v", err)
	}
	for _, in := range []MoneyFlowInputs{
		{Spending: []CategorySpend{{Amount: math.MinInt64}}},
		{Income: []CategorySpend{{Amount: math.MaxInt64}, {Amount: 1}}},
		{Income: []CategorySpend{{Amount: math.MaxInt64}}, Net: -1},
	} {
		if _, err := BuildMoneyFlowChecked(in); !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("diagram input %+v: %v", in, err)
		}
	}
}

func TestExplicitReportLedgersDoNotApplyGlobalRefundsTwice(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rates := currency.Rates{Base: "USD"}
	raw := []domain.Transaction{
		{ID: "purchase", Date: start, Desc: "Bank fee", Tags: []string{"bank"}, Amount: money.New(-100, "USD")},
		{ID: "return", Date: end, Amount: money.New(25, "USD")},
	}
	links := []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "return"}}}
	prepared := budgeting.NewNettedLedger(raw, links)
	old := refundLinks
	refundLinks = links
	t.Cleanup(func() { refundLinks = old })
	flows, err := IncomeExpenseSeriesFromLedger(prepared, []time.Time{start, end}, rates)
	if err != nil || len(flows) != 1 || flows[0].Expense != 75 {
		t.Fatalf("flow=%+v err=%v", flows, err)
	}
	tags, err := SpendingByTagFromLedger(prepared, start, end, false, time.Time{}, time.Time{}, rates)
	if err != nil || len(tags) != 1 || tags[0].Amount != 75 {
		t.Fatalf("tags=%+v err=%v", tags, err)
	}
	costs, err := CostOfMoneyFromLedger(prepared, nil, start, end, rates)
	if err != nil || costs.FeeTotal != 75 {
		t.Fatalf("costs=%+v err=%v", costs, err)
	}
}

func TestActivityValidationUsesOnlyRequestedAttribution(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rates := currency.Rates{Base: "USD"}
	rows := budgeting.NettedLedger{{Date: start, Amount: money.New(100, "USD"), Splits: []domain.CategorySplit{{Amount: money.New(100, "EUR")}}}}
	if err := ValidateActivityAmounts(rows, start, end, rates, false); err != nil {
		t.Fatalf("parent-only report required split FX: %v", err)
	}
	if err := ValidateActivityAmounts(rows, start, end, rates, true); err == nil {
		t.Fatal("split-attributed report hid missing FX")
	}
	rows = budgeting.NettedLedger{{Date: start, Amount: money.New(math.MinInt64, "USD")}}
	if err := ValidateActivityAmounts(rows, start, end, rates, false); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("source magnitude: %v", err)
	}
	rows = budgeting.NettedLedger{{Date: start, Amount: money.New(math.MaxInt64, "USD")}, {Date: start, Amount: money.New(1, "USD")}}
	if err := ValidateActivityAmounts(rows, start, end, rates, false); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("parent accumulation: %v", err)
	}
}

func TestInvestmentPerformanceCheckedKeepsRepresentableRatios(t *testing.T) {
	account := domain.Account{ID: "fund", Type: domain.TypeInvestment, OpeningBalance: money.New(math.MaxInt64/2, "USD")}
	rows := []domain.Transaction{{AccountID: "fund", Amount: money.New(math.MaxInt64/2, "USD")}}
	perf, err := InvestmentPerformanceChecked([]domain.Account{account}, rows, currency.Rates{Base: "USD"})
	if err != nil || len(perf) != 1 || perf[0].ReturnBips != 10000 || perf[0].Current != math.MaxInt64-1 {
		t.Fatalf("large valid ratio=%+v err=%v", perf, err)
	}
	account.OpeningBalance = money.New(math.MaxInt64, "USD")
	if _, err := InvestmentPerformanceChecked([]domain.Account{account}, rows, currency.Rates{Base: "USD"}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("opening plus history: %v", err)
	}
	account.OpeningBalance = money.New(1, "USD")
	rows[0].Amount = money.New(math.MaxInt64-1, "USD")
	if _, err := InvestmentPerformanceChecked([]domain.Account{account}, rows, currency.Rates{Base: "USD"}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("unrepresentable ratio: %v", err)
	}
}
