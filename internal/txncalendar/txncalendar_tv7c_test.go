// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestTV7CMonthCashflowIncludesLiabilityPaymentAndAvoidsDuplicateRecurring(t *testing.T) {
	day := func(n int) time.Time { return time.Date(2026, time.October, n, 0, 0, 0, 0, time.UTC) }
	accounts := []domain.Account{
		{ID: "card", Name: "Rewards Credit Card", OwnerID: "household", Scope: domain.ScopeShared,
			Class: domain.ClassLiability, DueDayOfMonth: 22, MinPayment: money.New(22000, "USD")},
		{ID: "other-card", Name: "Other card", OwnerID: "other-member", Scope: domain.ScopeIndividual,
			Class: domain.ClassLiability, DueDayOfMonth: 22, MinPayment: money.New(15000, "USD")},
	}
	recurring := []domain.Recurring{{ID: "card-payment", Label: "Card payment", AccountID: "checking",
		Amount: money.New(-22000, "USD"), Cadence: domain.CadenceMonthly, NextDue: day(22)}}
	scopedAccounts := ScopeLiabilityAccounts(accounts, txnfilter.Criteria{Member: "household"})
	cases := []struct {
		name             string
		recurring        []domain.Recurring
		wantLabel        string
		wantAccountGhost bool
	}{
		{name: "statement due date", wantLabel: "Rewards Credit Card", wantAccountGhost: true},
		{name: "matching recurring payment", recurring: recurring, wantLabel: "Card payment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			month, err := MonthCashflowWithAccounts(day(1), time.Sunday, nil, tc.recurring, scopedAccounts, currency.Rates{Base: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			var found []Ghost
			for _, week := range month.Weeks {
				for _, cell := range week.Days {
					if DayKey(cell.Date) == "2026-10-22" {
						found = cell.Ghosts
					}
				}
			}
			if len(found) != 1 {
				t.Fatalf("Oct 22 scheduled items = %#v, want one payment", found)
			}
			if found[0].Label != tc.wantLabel || found[0].Amount != -22000 || found[0].Liability != tc.wantAccountGhost {
				t.Fatalf("Oct 22 item = %+v, want label %q, -22000 USD, liability=%v", found[0], tc.wantLabel, tc.wantAccountGhost)
			}
		})
	}
}

func TestTV7CScopeLiabilityAccountsHonorsAccountAndMemberFilters(t *testing.T) {
	accounts := []domain.Account{
		{ID: "shared", OwnerID: "group", Scope: domain.ScopeShared, Class: domain.ClassLiability,
			OwnershipShares: map[string]int{"m1": 50, "m2": 50}},
		{ID: "private", OwnerID: "m2", Scope: domain.ScopeIndividual, Class: domain.ClassLiability},
		{ID: "archived", OwnerID: "m1", Class: domain.ClassLiability, Archived: true},
	}
	got := ScopeLiabilityAccounts(accounts, txnfilter.Criteria{Account: "shared,private", Member: "m1"})
	if len(got) != 1 || got[0].ID != "shared" {
		t.Fatalf("scoped liability accounts = %#v, want shared only", got)
	}
}

func TestTV7CFoldSettledHidesPaidLiabilityOccurrence(t *testing.T) {
	due := time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC)
	weeks := []CashflowWeek{{Days: []CashflowCell{{Date: due, InMonth: true, Ghosts: []Ghost{{
		RecurringID: "account:card", Label: "Rewards Credit Card", Amount: -22000, Currency: "USD", Date: due, Liability: true,
	}}}}}}
	got := FoldSettled(weeks, map[string]bool{domain.OccurrenceKey("account:card", due): true}, time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC))
	if len(got[0].Days[0].Ghosts) != 0 {
		t.Fatalf("settled liability payment remained scheduled: %#v", got[0].Days[0].Ghosts)
	}
}
