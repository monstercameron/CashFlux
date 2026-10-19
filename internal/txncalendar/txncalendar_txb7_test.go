// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func txb7Date(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := dateutil.ParseDate(value)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", value, err)
	}
	return date
}

func TestBucketByDayInBaseConvertsAndReportsMissingRates(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "usd", Date: txb7Date(t, "2026-07-03"), Amount: money.New(10000, "USD")},
		{ID: "eur", Date: txb7Date(t, "2026-07-03"), Amount: money.New(10000, "EUR")},
	}
	got, err := BucketByDayInBase(txns, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.2}})
	if err != nil {
		t.Fatalf("BucketByDayInBase: %v", err)
	}
	if got["2026-07-03"].Net != 22000 {
		t.Errorf("day net = %d, want 22000 USD minor units", got["2026-07-03"].Net)
	}
	if _, err := BucketByDayInBase(txns[1:], currency.Rates{Base: "USD"}); err == nil {
		t.Error("missing conversion rate should be returned")
	}
}

func TestMonthInBaseUsesConvertedDayTotals(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "usd", Date: txb7Date(t, "2026-07-03"), Amount: money.New(10000, "USD")},
		{ID: "eur", Date: txb7Date(t, "2026-07-03"), Amount: money.New(10000, "EUR")},
	}
	weeks, err := MonthInBase(txb7Date(t, "2026-07-01"), time.Sunday, txns, nil,
		currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.2}})
	if err != nil {
		t.Fatalf("MonthInBase: %v", err)
	}
	for _, week := range weeks {
		for _, cell := range week {
			if DayKey(cell.Date) == "2026-07-03" {
				if cell.Stat.Net != 22000 || cell.Stat.Count != 2 {
					t.Fatalf("July 3 stat = %+v, want net 22000 and count 2", cell.Stat)
				}
				return
			}
		}
	}
	t.Fatal("July 3 cell missing")
}

func TestMonthInBaseIgnoresMissingRatesOutsideVisibleMonth(t *testing.T) {
	txns := []domain.Transaction{{
		ID: "outside-month", Date: txb7Date(t, "2026-06-03"), Amount: money.New(10000, "EUR"),
	}}
	if _, err := MonthInBase(txb7Date(t, "2026-07-01"), time.Sunday, txns, nil, currency.Rates{Base: "USD"}); err != nil {
		t.Fatalf("unconvertible transaction outside visible month should not block calendar: %v", err)
	}
}

func TestScopeRecurringUsesAccountCategoryAndMemberFilters(t *testing.T) {
	recurring := []domain.Recurring{
		{ID: "match", AccountID: "a", CategoryID: "food"},
		{ID: "wrong-account", AccountID: "b", CategoryID: "food"},
		{ID: "wrong-category", AccountID: "a", CategoryID: "rent"},
		{ID: "unassigned", CategoryID: "food"},
	}
	accounts := []domain.Account{
		{ID: "a", OwnerID: "m1"},
		{ID: "b", OwnerID: "m2"},
	}
	got := ScopeRecurring(recurring, txnfilter.Criteria{Account: "a", Category: "food", Member: "m1"}, accounts)
	if len(got) != 1 || got[0].ID != "match" {
		t.Fatalf("filtered recurrings = %#v, want only match", got)
	}
}

func TestGhostsSkipsPausedRecurring(t *testing.T) {
	paused := domain.Recurring{
		ID: "paused", Label: "Paused bill", Amount: money.New(-5000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: txb7Date(t, "2026-07-15"), Paused: true,
	}
	got := Ghosts([]domain.Recurring{paused}, txb7Date(t, "2026-07-01"), txb7Date(t, "2026-08-01"))
	if len(got) != 0 {
		t.Fatalf("paused recurring ghosts = %d, want 0", len(got))
	}
}

func TestGhostsCatchUpOldDailyAndWeeklySchedules(t *testing.T) {
	from, to := txb7Date(t, "2026-07-01"), txb7Date(t, "2026-08-01")
	cases := []struct {
		name    string
		cadence domain.RecurringCadence
		due     string
		want    int
	}{
		{name: "daily", cadence: domain.CadenceDaily, due: "2020-01-01", want: 31},
		{name: "weekly", cadence: domain.CadenceWeekly, due: "2010-01-01", want: 5},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recurring := domain.Recurring{
				ID: tt.name, Cadence: tt.cadence, NextDue: txb7Date(t, tt.due),
				Amount: money.New(-100, "USD"),
			}
			got := Ghosts([]domain.Recurring{recurring}, from, to)
			if len(got) != tt.want {
				t.Fatalf("occurrences = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestGhostsCatchUpOldMonthBasedSchedules(t *testing.T) {
	from, to := txb7Date(t, "2026-07-01"), txb7Date(t, "2026-08-01")
	cases := []struct {
		name    string
		cadence domain.RecurringCadence
		due     string
		want    int
	}{
		{name: "monthly end-of-month normalization", cadence: domain.CadenceMonthly, due: "2010-01-31", want: 1},
		{name: "quarterly", cadence: domain.CadenceQuarterly, due: "2010-01-01", want: 1},
		{name: "yearly", cadence: domain.CadenceYearly, due: "2010-07-15", want: 1},
		{name: "semimonthly", cadence: domain.CadenceSemimonthly, due: "2010-01-01", want: 2},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recurring := domain.Recurring{
				ID: tt.name, Cadence: tt.cadence, NextDue: txb7Date(t, tt.due),
				Amount: money.New(-100, "USD"),
			}
			got := Ghosts([]domain.Recurring{recurring}, from, to)
			if len(got) != tt.want {
				t.Fatalf("occurrences = %d, want %d", len(got), tt.want)
			}
		})
	}
}
