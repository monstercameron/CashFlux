// SPDX-License-Identifier: MIT

package accountview_test

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func at(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.UTC)
}

func TestProjectionOccurrenceWindowing(t *testing.T) {
	cases := []struct {
		name    string
		row     domain.Recurring
		asOf    time.Time
		horizon int
		want    []time.Time
	}{
		{
			"a bill due at midnight today survives an asOf with a time of day",
			domain.Recurring{NextDue: at(2026, time.October, 20, 0), Cadence: domain.CadenceYearly},
			at(2026, time.October, 20, 15), 30,
			[]time.Time{at(2026, time.October, 20, 0)},
		},
		{
			"a bill due late on the last day of the window is inside it",
			domain.Recurring{NextDue: at(2026, time.November, 19, 23), Cadence: domain.CadenceYearly},
			at(2026, time.October, 20, 12), 30,
			[]time.Time{at(2026, time.November, 19, 0)},
		},
		{
			"a bill the day after the window is outside it",
			domain.Recurring{NextDue: at(2026, time.November, 20, 0), Cadence: domain.CadenceYearly},
			at(2026, time.October, 20, 12), 30,
			nil,
		},
		{
			"an overdue monthly bill lands today once and then resumes",
			domain.Recurring{NextDue: at(2026, time.October, 5, 0), Cadence: domain.CadenceMonthly},
			at(2026, time.October, 20, 0), 30,
			[]time.Time{at(2026, time.October, 20, 0), at(2026, time.November, 5, 0)},
		},
		{
			"a daily bill overdue for years still projects its future dates",
			domain.Recurring{NextDue: at(2020, time.January, 1, 0), Cadence: domain.CadenceDaily},
			at(2026, time.October, 20, 0), 2,
			[]time.Time{at(2026, time.October, 20, 0), at(2026, time.October, 20, 0), at(2026, time.October, 21, 0), at(2026, time.October, 22, 0)},
		},
		{
			"a monthly bill on the 31st returns to the 31st after a short month",
			domain.Recurring{NextDue: at(2026, time.January, 31, 0), Cadence: domain.CadenceMonthly},
			at(2026, time.January, 31, 0), 70,
			[]time.Time{at(2026, time.January, 31, 0), at(2026, time.February, 28, 0), at(2026, time.March, 31, 0)},
		},
		{
			"a quarterly bill on the 31st keeps its anchor",
			domain.Recurring{NextDue: at(2026, time.January, 31, 0), Cadence: domain.CadenceQuarterly},
			at(2026, time.January, 31, 0), 400,
			[]time.Time{at(2026, time.January, 31, 0), at(2026, time.April, 30, 0), at(2026, time.July, 31, 0), at(2026, time.October, 31, 0), at(2027, time.January, 31, 0)},
		},
		{
			"an overdue monthly bill on the 31st resumes on the 31st",
			domain.Recurring{NextDue: at(2026, time.January, 31, 0), Cadence: domain.CadenceMonthly},
			at(2026, time.March, 1, 0), 40,
			[]time.Time{at(2026, time.March, 1, 0), at(2026, time.March, 31, 0)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			row.AccountID, row.Label, row.Amount = "acct", "bill", money.New(-100, "USD")
			inputs := accountview.PrepareProjectionInputs([]domain.Recurring{row})
			projection, err := inputs.Project("acct", money.New(10000, "USD"), tc.asOf, tc.horizon)
			if err != nil {
				t.Fatal(err)
			}
			var got []time.Time
			for _, d := range projection.Drivers {
				got = append(got, d.Date)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("driver dates = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrepareProjectionInputsSkipsPausedRowsAndServices(t *testing.T) {
	due := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	row := func(id string, paused, service bool) domain.Recurring {
		return domain.Recurring{ID: id, Label: id, AccountID: "acct", Amount: money.New(-100, "USD"), NextDue: due, Cadence: domain.CadenceYearly, Paused: paused, Service: service}
	}
	inputs := accountview.PrepareProjectionInputs([]domain.Recurring{row("active", false, false), row("paused", true, false), row("service", false, true)})
	projection, err := inputs.Project("acct", money.New(10000, "USD"), time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Drivers) != 1 || projection.Drivers[0].Label != "active" {
		t.Fatalf("drivers = %+v, want only the active non-service row", projection.Drivers)
	}
}

func TestProjectForNormalisesLiabilityBalances(t *testing.T) {
	asOf := at(2026, time.October, 20, 0)
	charge := domain.Recurring{
		ID: "r", Label: "charge", AccountID: "card", Amount: money.New(-60000, "USD"),
		NextDue: at(2026, time.October, 25, 0), Cadence: domain.CadenceYearly,
	}
	inputs := accountview.PrepareProjectionInputs([]domain.Recurring{charge})
	account := func(class domain.AccountClass, sign domain.LiabilitySign) domain.Account {
		return domain.Account{ID: "card", Class: class, Currency: "USD", LiabilitySign: sign}
	}
	cases := []struct {
		name         string
		account      domain.Account
		stored       int64
		wantStart    int64
		wantLow, end int64
	}{
		{"positive-owed card is stated negative", account(domain.ClassLiability, domain.LiabilityPositiveOwed), 50000, -50000, -110000, -110000},
		{"negative-owed card is already stated", account(domain.ClassLiability, domain.LiabilityNegativeOwed), -50000, -50000, -110000, -110000},
		{"asset is untouched", account(domain.ClassAsset, ""), 50000, 50000, -10000, -10000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := inputs.ProjectFor(tc.account, money.New(tc.stored, "USD"), asOf, 30)
			if err != nil {
				t.Fatal(err)
			}
			if got.Start != tc.wantStart || got.Low != tc.wantLow || got.End != tc.end {
				t.Fatalf("start/low/end = %d/%d/%d, want %d/%d/%d", got.Start, got.Low, got.End, tc.wantStart, tc.wantLow, tc.end)
			}
			near, far := inputs.ProjectPairFor(tc.account, money.New(tc.stored, "USD"), asOf, 30, 90)
			if near.Err != nil || far.Err != nil || near.Projection.Start != tc.wantStart || far.Projection.Start != tc.wantStart {
				t.Fatalf("pair starts = %d/%d (%v/%v), want %d", near.Projection.Start, far.Projection.Start, near.Err, far.Err, tc.wantStart)
			}
		})
	}
}

func TestFixedCadenceFromZeroOrAncientNextDueStaysBounded(t *testing.T) {
	asOf := at(2026, time.October, 20, 0)
	cases := []struct {
		name    string
		cadence domain.RecurringCadence
		nextDue time.Time
		horizon int
		want    []time.Time
	}{
		{"daily zero NextDue", domain.CadenceDaily, time.Time{}, 2, []time.Time{asOf, asOf, at(2026, time.October, 21, 0), at(2026, time.October, 22, 0)}},
		{"weekly zero NextDue keeps its weekday grid", domain.CadenceWeekly, time.Time{}, 6, []time.Time{asOf, at(2026, time.October, 26, 0)}},
		{"weekly 500 years overdue resumes on its own weekday grid", domain.CadenceWeekly, at(1526, time.October, 20, 0), 2, []time.Time{asOf, at(2026, time.October, 21, 0)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := domain.Recurring{AccountID: "acct", Label: "bill", Amount: money.New(-100, "USD"), NextDue: tc.nextDue, Cadence: tc.cadence}
			projection, err := accountview.PrepareProjectionInputs([]domain.Recurring{row}).Project("acct", money.New(10000, "USD"), asOf, tc.horizon)
			if err != nil {
				t.Fatal(err)
			}
			var got []time.Time
			for _, d := range projection.Drivers {
				got = append(got, d.Date)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("driver dates = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrepareHistoryReportsOverflow(t *testing.T) {
	account := domain.Account{ID: "a", Currency: "USD", OpeningBalance: money.New(math.MaxInt64-5, "USD")}
	at0 := at(2026, time.October, 1, 0)
	cases := []struct {
		name string
		txns []domain.Transaction
		want bool
	}{
		{"no rows", nil, false},
		{"fits exactly", []domain.Transaction{{ID: "t1", AccountID: "a", Date: at0, Amount: money.New(5, "USD")}}, false},
		{"one past the limit", []domain.Transaction{{ID: "t1", AccountID: "a", Date: at0, Amount: money.New(6, "USD")}}, true},
		{"other accounts are ignored", []domain.Transaction{{ID: "t1", AccountID: "b", Date: at0, Amount: money.New(60, "USD")}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := accountview.PrepareHistory(account, tc.txns).Overflowed(); got != tc.want {
				t.Fatalf("Overflowed = %v, want %v", got, tc.want)
			}
		})
	}
	var nilHistory *accountview.History
	if nilHistory.Overflowed() {
		t.Fatal("nil History reported overflow")
	}
}
