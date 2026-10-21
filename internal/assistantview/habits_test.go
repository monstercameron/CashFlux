// SPDX-License-Identifier: MIT

package assistantview

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendpattern"
)

func TestHabitSeriesPreservesWindowZeroesAndReportedFlows(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	start := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	excluded := questionTxn("excluded", "", "USD", -9000, now)
	excluded.ExcludeFromReports = true
	transfer := questionTxn("transfer", "", "USD", -9000, now)
	transfer.TransferAccountID = "savings"
	excludedIncome := questionTxn("excluded income", "", "USD", 9000, start)
	excludedIncome.ExcludeFromReports = true
	txns := []domain.Transaction{
		questionTxn("today", "", "EUR", -100, now),
		questionTxn("income", "", "USD", 1000, now), questionTxn("second income", "", "USD", 2000, now),
		questionTxn("first day", "", "USD", -50, start),
		questionTxn("too early", "", "USD", -9000, start.AddDate(0, 0, -1)),
		questionTxn("too late", "", "USD", -9000, now.AddDate(0, 0, 1)),
		questionTxn("missing FX", "", "JPY", -9000, now), excluded, transfer, excludedIncome,
	}
	days, paydays := HabitSeries(txns, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, now)
	if len(days) != HabitWindowDays+1 || !days[0].Date.Equal(start) || days[0].Minor != 50 || days[len(days)-1].Minor != 200 || days[1].Minor != 0 {
		t.Fatalf("wrong boundaries, zeroes or converted reported spend: first=%+v last=%+v count=%d", days[0], days[len(days)-1], len(days))
	}
	if len(paydays) != 1 || paydays[0].Format("2006-01-02") != "2026-10-20" {
		t.Fatalf("income dates were not deduplicated or filtered: %+v", paydays)
	}
	if txns[0].Amount != money.New(-100, "EUR") {
		t.Fatal("habit derivation mutated its input")
	}
}

func TestHabitSeriesChecksMagnitudeAndDailyAddition(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	days, _ := HabitSeries([]domain.Transaction{
		questionTxn("invalid magnitude", "", "USD", math.MinInt64, now),
		questionTxn("valid large", "", "USD", -math.MaxInt64, now),
		questionTxn("would overflow", "", "USD", -1, now),
	}, currency.Rates{Base: "USD"}, now)
	if got := days[len(days)-1].Minor; got != math.MaxInt64 {
		t.Fatalf("invalid expense wrapped daily spend: %d", got)
	}
}

func TestHabitsFindsRepeatWeekendAndPaydayPatterns(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, phase := range []string{"weekend", "payday"} {
		t.Run(phase, func(t *testing.T) {
			var txns []domain.Transaction
			for date := now.AddDate(0, 0, -HabitWindowDays); !date.After(now); date = date.AddDate(0, 0, 1) {
				inside := spendpattern.IsWeekend(date)
				if phase == "payday" {
					inside = date.Day() <= 4 || date.Day() >= 15 && date.Day() <= 18
					if date.Day() == 1 || date.Day() == 15 {
						txns = append(txns, questionTxn("pay", "", "USD", 1000, date))
					}
				}
				spend := int64(-10)
				if inside {
					spend = -100
				}
				txns = append(txns, questionTxn("spend", "", "USD", spend, date))
			}
			facts := Habits(txns, currency.Rates{Base: "USD"}, now)
			finding := facts.Weekend
			if phase == "payday" {
				finding = facts.Payday
			}
			if !finding.Holds() || finding.Periods < 3 || finding.PeriodsHeld != finding.Periods || finding.ExtraPerDayMinor() != 90 {
				t.Fatalf("repeat %s pattern missing: %+v", phase, finding)
			}
		})
	}
}

func TestHabitsLargeDaysKeepRepresentableAverage(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	var txns []domain.Transaction
	for date := now.AddDate(0, 0, -HabitWindowDays); !date.After(now); date = date.AddDate(0, 0, 1) {
		txns = append(txns, questionTxn("large", "", "USD", -math.MaxInt64, date))
	}
	got := Habits(txns, currency.Rates{Base: "USD"}, now).Weekend
	if !got.Known || got.InsideRateMinor != math.MaxInt64 || got.OutsideRateMinor != math.MaxInt64 || got.Holds() {
		t.Fatalf("large daily values overflowed a valid average: %+v", got)
	}
}
