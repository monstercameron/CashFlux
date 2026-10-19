// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestScopeRecurringUsesAccountCategoryAndMemberFilters(t *testing.T) {
	recurring := []domain.Recurring{
		{ID: "match", AccountID: "a", CategoryID: "food"},
		{ID: "wrong-account", AccountID: "b", CategoryID: "food"},
		{ID: "wrong-category", AccountID: "a", CategoryID: "rent"},
		{ID: "unassigned", CategoryID: "food"},
	}
	accounts := []domain.Account{{ID: "a", OwnerID: "m1"}, {ID: "b", OwnerID: "m2"}}
	got := ScopeRecurring(recurring, txnfilter.Criteria{Account: "a", Category: "food", Member: "m1"}, accounts)
	if len(got) != 1 || got[0].ID != "match" {
		t.Fatalf("filtered recurrings = %#v, want only match", got)
	}
}

func TestGhostsSkipsPausedRecurring(t *testing.T) {
	paused := domain.Recurring{
		ID: "paused", Label: "Paused bill", Amount: money.New(-5000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: mustDate("2026-07-15"), Paused: true,
	}
	if got := Ghosts([]domain.Recurring{paused}, mustDate("2026-07-01"), mustDate("2026-08-01")); len(got) != 0 {
		t.Fatalf("paused recurring ghosts = %d, want 0", len(got))
	}
}

func TestGhostsCatchUpOldSchedules(t *testing.T) {
	from, to := mustDate("2026-07-01"), mustDate("2026-08-01")
	cases := []struct {
		name    string
		cadence domain.RecurringCadence
		due     string
		want    int
	}{
		{name: "daily", cadence: domain.CadenceDaily, due: "2020-01-01", want: 31},
		{name: "weekly", cadence: domain.CadenceWeekly, due: "2010-01-01", want: 5},
		{name: "monthly end-of-month normalization", cadence: domain.CadenceMonthly, due: "2010-01-31", want: 1},
		{name: "quarterly", cadence: domain.CadenceQuarterly, due: "2010-01-01", want: 1},
		{name: "yearly", cadence: domain.CadenceYearly, due: "2010-07-15", want: 1},
		{name: "semimonthly", cadence: domain.CadenceSemimonthly, due: "2010-01-01", want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := domain.Recurring{ID: tc.name, Cadence: tc.cadence, NextDue: mustDate(tc.due), Amount: money.New(-100, "USD")}
			if got := len(Ghosts([]domain.Recurring{rule}, from, to)); got != tc.want {
				t.Fatalf("occurrences = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDayKeyUsesCalendarDate(t *testing.T) {
	local := time.Date(2026, time.July, 3, 23, 30, 0, 0, time.FixedZone("west", -7*60*60))
	if got := DayKey(local); got != "2026-07-03" {
		t.Fatalf("DayKey(%s) = %q, want local calendar day 2026-07-03", local, got)
	}
}
