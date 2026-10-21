// SPDX-License-Identifier: MIT

package subscriptions

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCheckedCadenceAmountsPreserveNormalRounding(t *testing.T) {
	for _, tc := range []struct {
		cadence Cadence
		amount  int64
		monthly int64
		annual  int64
	}{
		{CadenceMonthly, 1599, 1599, 1599 * 12},
		{CadenceWeekly, 2500, 2500 * 52 / 12, 2500 * 52},
		{CadenceYearly, 1200, 100, 1200},
	} {
		sub := Subscription{Name: "sub", Cadence: tc.cadence, Amount: tc.amount, Currency: "USD"}
		monthly, err := sub.MonthlyAmountChecked()
		if err != nil || monthly != tc.monthly {
			t.Errorf("%s monthly=(%d,%v), want %d", tc.cadence, monthly, err, tc.monthly)
		}
		annual, err := sub.AnnualAmountChecked()
		if err != nil || annual != tc.annual {
			t.Errorf("%s annual=(%d,%v), want %d", tc.cadence, annual, err, tc.annual)
		}
	}
}

func TestCheckedSubscriptionAmountsRejectMagnitudeAndCadenceOverflow(t *testing.T) {
	for _, tc := range []struct {
		sub             Subscription
		monthly, annual bool
	}{
		{Subscription{Name: "min", Cadence: CadenceMonthly, Amount: math.MinInt64, Currency: "USD"}, true, true},
		{Subscription{Name: "weekly", Cadence: CadenceWeekly, Amount: math.MaxInt64, Currency: "USD"}, true, true},
		{Subscription{Name: "annual", Cadence: CadenceMonthly, Amount: math.MaxInt64, Currency: "USD"}, false, true},
	} {
		_, monthlyErr := tc.sub.MonthlyAmountChecked()
		if (monthlyErr != nil) != tc.monthly {
			t.Errorf("MonthlyAmountChecked(%+v) err=%v, overflow=%v", tc.sub, monthlyErr, tc.monthly)
		}
		_, annualErr := tc.sub.AnnualAmountChecked()
		if (annualErr != nil) != tc.annual {
			t.Errorf("AnnualAmountChecked(%+v) err=%v, overflow=%v", tc.sub, annualErr, tc.annual)
		}
	}
}

func TestCheckedSubscriptionTotalsRejectOverflow(t *testing.T) {
	rows := []Subscription{{Name: "a", Cadence: CadenceMonthly, Amount: math.MaxInt64}, {Name: "b", Cadence: CadenceMonthly, Amount: 1}}
	if _, err := MonthlyTotalChecked(rows); err == nil {
		t.Fatal("MonthlyTotalChecked wrapped sum")
	}
	if _, err := AnnualTotalChecked([]Subscription{{Name: "a", Amount: math.MaxInt64, Cadence: CadenceYearly}, {Name: "b", Amount: 1, Cadence: CadenceYearly}}); err == nil {
		t.Fatal("AnnualTotalChecked wrapped sum")
	}
	if _, err := AnnualSavingsChecked([]Subscription{{Name: "a", Amount: math.MaxInt64, Cadence: CadenceYearly}, {Name: "b", Amount: 1, Cadence: CadenceYearly}}, map[string]bool{"a": true, "b": true}); err == nil {
		t.Fatal("AnnualSavingsChecked wrapped sum")
	}
}

func TestCheckedSubscriptionShareUsesWideIntermediate(t *testing.T) {
	got, err := SharePercentChecked(math.MaxInt64, math.MaxInt64)
	if err != nil || got != 100 {
		t.Fatalf("SharePercentChecked(max,max)=(%d,%v), want 100", got, err)
	}
	got, err = SpendingSharePercentChecked(math.MaxInt64, math.MaxInt64, 3)
	if err != nil || got != 300 {
		t.Fatalf("SpendingSharePercentChecked(max,max,3)=(%d,%v), want 300", got, err)
	}
	if _, err := SharePercentChecked(1, 0); err == nil {
		t.Fatal("zero denominator was accepted")
	}
	if _, err := money.MulDiv(math.MaxInt64, 100, 1, money.RoundTowardZero); err == nil {
		t.Fatal("test precondition: expected oversized result to overflow")
	}
}

func TestBuildFinancialSummarySeparatesActiveRowsFromCountedTotals(t *testing.T) {
	active := []Subscription{
		{Name: "weekly", Cadence: CadenceWeekly, Amount: 1200},
		{Name: "yearly", Cadence: CadenceYearly, Amount: 12000},
	}
	counted := active[:1]
	got, err := BuildFinancialSummary(active, counted, map[string]bool{"yearly": true}, 30000, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveMonthly != 6200 || got.CountedMonthly != 5200 || got.CountedAnnual != 62400 || got.SelectedAnnual != 12000 {
		t.Fatalf("summary totals=%+v", got)
	}
	if got.SharePercentByName["weekly"] != 83 || got.SharePercentByName["yearly"] != 16 || !got.HasSpendingShare || got.SpendingSharePercent != 52 {
		t.Fatalf("summary shares=%+v", got)
	}
}

func BenchmarkSubscriptionMonthlyTotals(b *testing.B) {
	rows := make([]Subscription, 100)
	for i := range rows {
		rows[i] = Subscription{Name: "service", Cadence: CadenceWeekly, Amount: 999, Currency: "USD"}
	}
	b.Run("unchecked-legacy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = MonthlyTotal(rows)
		}
	})
	b.Run("checked", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = MonthlyTotalChecked(rows)
		}
	})
}
