// SPDX-License-Identifier: MIT

package txncalendar

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func tc4n3Day(day int) time.Time {
	return time.Date(2026, time.October, day, 0, 0, 0, 0, time.UTC)
}

func TestSettledLiabilityOccurrencesUsesEachDueDateAndInflowSign(t *testing.T) {
	weeks := []CashflowWeek{{Days: []CashflowCell{
		{Date: tc4n3Day(22), Ghosts: []Ghost{{RecurringID: "account:positive", Date: tc4n3Day(22), Liability: true}}},
		{Date: time.Date(2026, time.November, 22, 0, 0, 0, 0, time.UTC), Ghosts: []Ghost{{RecurringID: "account:positive", Date: time.Date(2026, time.November, 22, 0, 0, 0, 0, time.UTC), Liability: true}}},
		{Date: tc4n3Day(20), Ghosts: []Ghost{{RecurringID: "account:negative", Date: tc4n3Day(20), Liability: true}}},
	}}}
	accounts := []domain.Account{
		{ID: "positive", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed},
		{ID: "negative", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityNegativeOwed},
	}
	txns := []domain.Transaction{
		{ID: "positive-payment", AccountID: "positive", TransferAccountID: "checking", Date: tc4n3Day(15), Amount: money.New(-12000, "USD")},
		{ID: "wrong-positive-sign", AccountID: "positive", TransferAccountID: "checking", Date: tc4n3Day(16), Amount: money.New(5000, "USD")},
		{ID: "negative-payment", AccountID: "negative", TransferAccountID: "checking", Date: tc4n3Day(17), Amount: money.New(7000, "USD")},
		{ID: "future-payment", AccountID: "positive", TransferAccountID: "checking", Date: tc4n3Day(21), Amount: money.New(-3000, "USD")},
	}
	got := SettledLiabilityOccurrences(weeks, txns, accounts, tc4n3Day(18))
	if !got[domain.OccurrenceKey("account:positive", tc4n3Day(22))] {
		t.Error("signed payment into positive-owed liability did not settle its upcoming occurrence")
	}
	if !got[domain.OccurrenceKey("account:negative", tc4n3Day(20))] {
		t.Error("signed payment into negative-owed liability did not settle its occurrence")
	}
	if got[domain.OccurrenceKey("account:positive", time.Date(2026, time.November, 22, 0, 0, 0, 0, time.UTC))] {
		t.Error("October payment incorrectly settled the November occurrence")
	}
}

func TestSettledLiabilityOccurrencesRequiresPaymentInsideWindow(t *testing.T) {
	due := tc4n3Day(22)
	week := []CashflowWeek{{Days: []CashflowCell{{Date: due, Ghosts: []Ghost{{RecurringID: "account:card", Date: due, Liability: true}}}}}}
	account := []domain.Account{{ID: "card", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed}}
	cases := []struct {
		name string
		date time.Time
		bill string
		want bool
	}{
		{name: "payment on due date", date: due, bill: "card", want: true},
		{name: "linked payment inside window", date: due.AddDate(0, 0, -31), bill: "card", want: true},
		{name: "payment outside window", date: due.AddDate(0, 0, -32), bill: "card"},
		{name: "payment after due date", date: due.AddDate(0, 0, 1), bill: "card"},
		{name: "unscoped account", date: due, bill: "other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SettledLiabilityOccurrences(week, []domain.Transaction{{BillAccountID: tc.bill, Date: tc.date}}, account, due.AddDate(0, 0, 5))
			settled := got[domain.OccurrenceKey("account:card", due)]
			if settled != tc.want {
				t.Fatalf("occurrence settled = %v, want %v", settled, tc.want)
			}
		})
	}
}

func TestFoldSettledDoesNotApplyLegacyAccountWideFlag(t *testing.T) {
	due := tc4n3Day(22)
	weeks := []CashflowWeek{{Days: []CashflowCell{{Date: due, Ghosts: []Ghost{{RecurringID: "account:card", Date: due, Liability: true}}}}}}
	got := FoldSettled(weeks, map[string]bool{"account:card": true}, tc4n3Day(23))
	if len(got[0].Days[0].Ghosts) != 1 {
		t.Fatal("legacy account-wide flag hid a due-date occurrence")
	}
}

func TestMonthCashflowRejectsMinorUnitOverflow(t *testing.T) {
	day := tc4n3Day(5)
	txns := []domain.Transaction{
		{ID: "one", Date: day, Amount: money.New(math.MaxInt64, "USD")},
		{ID: "two", Date: day, Amount: money.New(1, "USD")},
	}
	_, err := MonthCashflow(day, time.Sunday, txns, nil, currency.Rates{Base: "USD"})
	if err == nil {
		t.Fatal("overflowing daily sum succeeded")
	}
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("overflow error = %v, want money.ErrOverflow", err)
	}
}
