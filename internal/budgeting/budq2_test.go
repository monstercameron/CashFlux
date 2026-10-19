// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSplitDiscountsNetBeforeExpenseSign(t *testing.T) {
	b := domain.Budget{ID: "groceries", CategoryID: "groceries", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")}
	txn := domain.Transaction{Amount: money.New(-10000, "USD"), Date: mustDate("2026-10-05"), Splits: []domain.CategorySplit{
		{CategoryID: "groceries", Amount: money.New(-12000, "USD")},
		{CategoryID: "groceries", Amount: money.New(2000, "USD")},
	}}
	got, err := Spent(b, []domain.Transaction{txn}, mustDate("2026-10-01"), mustDate("2026-11-01"), currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 10000 {
		t.Fatalf("spent = %d, want 10000", got.Amount)
	}
}

func TestPeriodRangeUsesUTCCalendarDatesAcrossLocations(t *testing.T) {
	for _, name := range []string{"America/New_York", "Asia/Tokyo"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		ref := time.Date(2026, time.October, 1, 20, 0, 0, 0, loc)
		start, end := PeriodRange(domain.PeriodQuarterly, ref, time.Monday)
		if !start.Equal(mustDate("2026-10-01")) || !end.Equal(mustDate("2027-01-01")) || start.Location() != time.UTC {
			t.Errorf("%s Q4 boundaries = %s..%s, want UTC 2026-10-01..2027-01-01", name, start, end)
		}
		semiStart, semiEnd := PeriodRange(domain.PeriodSemimonthly, ref, time.Monday)
		if !semiStart.Equal(mustDate("2026-10-01")) || !semiEnd.Equal(mustDate("2026-10-16")) {
			t.Errorf("%s first half = %s..%s", name, semiStart, semiEnd)
		}
		yearStart, yearEnd := PeriodRange(domain.PeriodYearly, ref, time.Monday)
		if !yearStart.Equal(mustDate("2026-01-01")) || !yearEnd.Equal(mustDate("2027-01-01")) {
			t.Errorf("%s yearly = %s..%s", name, yearStart, yearEnd)
		}
	}
}

func TestSemimonthlyStoredFirstAndSixteenthAreInTheirOwnPeriods(t *testing.T) {
	b := domain.Budget{CategoryID: "food", Limit: money.New(50000, "USD"), Period: domain.PeriodSemimonthly}
	txns := []domain.Transaction{
		{CategoryID: "food", Amount: money.New(-1000, "USD"), Date: mustDate("2026-10-01")},
		{CategoryID: "food", Amount: money.New(-1600, "USD"), Date: mustDate("2026-10-16")},
	}
	for _, name := range []string{"America/New_York", "Asia/Tokyo"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		firstStart, firstEnd := PeriodRange(b.Period, time.Date(2026, 10, 8, 12, 0, 0, 0, loc), time.Monday)
		first, err := Spent(b, txns, firstStart, firstEnd, currency.Rates{Base: "USD"})
		if err != nil {
			t.Fatal(err)
		}
		secondStart, secondEnd := PeriodRange(b.Period, time.Date(2026, 10, 20, 12, 0, 0, 0, loc), time.Monday)
		second, err := Spent(b, txns, secondStart, secondEnd, currency.Rates{Base: "USD"})
		if err != nil {
			t.Fatal(err)
		}
		if first.Amount != 1000 || second.Amount != 1600 {
			t.Errorf("%s first-half spend=%d second-half spend=%d; want 1000/1600", name, first.Amount, second.Amount)
		}
	}
}

func TestAnchoredBiweeklyKeyAndPreviousPeriod(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, time.October, 3, 0, 0, 0, 0, loc)
	ref := time.Date(2026, time.October, 10, 12, 0, 0, 0, loc)
	if got := PeriodKey(domain.PeriodBiweekly, ref, anchor); got != "2026-10-03" {
		t.Fatalf("period key = %s", got)
	}
	start, end := PreviousPeriodRangeAnchored(domain.PeriodBiweekly, ref, time.Monday, anchor)
	if !start.Equal(mustDate("2026-09-19")) || !end.Equal(mustDate("2026-10-03")) {
		t.Fatalf("previous = %s..%s", start, end)
	}
	rollover := time.Date(2026, time.October, 16, 20, 0, 0, 0, loc)
	start, end = PeriodRangeAnchored(domain.PeriodBiweekly, rollover, time.Monday, anchor)
	if !start.Equal(mustDate("2026-10-03")) || !end.Equal(mustDate("2026-10-17")) {
		t.Fatalf("local last-day rollover = %s..%s", start, end)
	}
}

func TestSummarizeRollupRefusesMixedCurrencies(t *testing.T) {
	statuses := []Status{
		rollupStatus("USD", domain.DirectionSpend, 100, 500, StateOK),
		{Budget: domain.Budget{ID: "EUR"}, Spent: money.New(100, "EUR"), Limit: money.New(500, "EUR"), Remaining: money.New(400, "EUR")},
	}
	if _, err := SummarizeRollupChecked(statuses); err == nil {
		t.Fatal("mixed currencies should fail")
	}
}

func TestRecommendationMADFloorKeepsPlausibleHighMonth(t *testing.T) {
	values := []SuggestionMonth{{Month: "a", Amount: 300}, {Month: "b", Amount: 300}, {Month: "c", Amount: 300}, {Month: "d", Amount: 320}}
	used, excluded, median, err := excludeSpendOutliers(values)
	if err != nil {
		t.Fatal(err)
	}
	if median != 300 || len(used) != 4 || len(excluded) != 0 {
		t.Fatalf("used=%v excluded=%v median=%d", used, excluded, median)
	}
}

func TestTrueUpSignedSplitNet(t *testing.T) {
	b := domain.Budget{ID: "b", CategoryID: "groceries", Limit: money.New(10000, "USD")}
	txn := domain.Transaction{Amount: money.New(-10000, "USD"), Date: mustDate("2026-10-05"), Splits: []domain.CategorySplit{
		{CategoryID: "groceries", Amount: money.New(-12000, "USD")},
		{CategoryID: "groceries", Amount: money.New(2000, "USD")},
	}}
	got, err := sumCoveredInRange(b, []domain.Transaction{txn}, mustDate("2026-10-01"), mustDate("2026-11-01"), currency.Rates{Base: "USD"}, b.TracksCategory)
	if err != nil {
		t.Fatal(err)
	}
	if got != 10000 {
		t.Fatalf("covered = %d, want 10000", got)
	}
}
