// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestJumpMonthCadencePreservesFebruaryForEndOfMonthBills(t *testing.T) {
	rec := []domain.Recurring{{
		ID:      "rent",
		Label:   "Month-End Bill",
		Cadence: domain.CadenceMonthly,
		Amount:  money.New(-10000, "USD"),
		NextDue: time.Date(2025, time.May, 31, 0, 0, 0, 0, time.UTC),
	}}
	from := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	got := Ghosts(rec, from, to)
	if len(got) != 1 || got[0].Date.Month() != time.February || got[0].Date.Day() != 28 {
		t.Fatalf("Feb 2026 projection = %+v, want single ghost on 2026-02-28", got)
	}
}

func TestSettledLiabilityOccurrencesIgnoresCreditCardRefunds(t *testing.T) {
	due := time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC)
	weeks := []CashflowWeek{{Days: []CashflowCell{{Date: due, Ghosts: []Ghost{{RecurringID: "account:cc", Date: due, Liability: true}}}}}}
	accounts := []domain.Account{{ID: "cc", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed}}
	// Ordinary merchant refund on a positive-owed card (NOT a transfer, NOT BillAccountID).
	refund := domain.Transaction{ID: "ref", AccountID: "cc", Date: time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(-2500, "USD")}
	got := SettledLiabilityOccurrences(weeks, []domain.Transaction{refund}, accounts, time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC))
	if got[domain.OccurrenceKey("account:cc", due)] {
		t.Fatal("non-transfer credit card refund falsely settled monthly bill occurrence")
	}
}

func TestMonthCashflowNonUTCBoundaryAndPositiveOwedLiabilityTransfer(t *testing.T) {
	west := time.FixedZone("UTC-5", -5*3600)
	octLate := time.Date(2026, time.October, 31, 23, 30, 0, 0, west) // 2026-11-01 04:30 UTC
	txns := []domain.Transaction{
		{ID: "late", Date: octLate, Amount: money.New(-4200, "USD")},
	}
	oct, err := MonthCashflow(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), time.Sunday, txns, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("oct MonthCashflow: %v", err)
	}
	var octSpent int64
	for _, w := range oct.Weeks {
		octSpent += w.Total.Spent
	}
	if octSpent != 4200 {
		t.Fatalf("Oct spent for non-UTC Oct 31 23:30 = %d, want 4200", octSpent)
	}

	// Positive-owed liability payment in MonthCashflowWithAccounts must not double-count.
	d := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "chk", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "cc", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed, Currency: "USD", OpeningBalance: money.New(50000, "USD")},
	}
	pair := []domain.Transaction{
		{ID: "out", AccountID: "chk", TransferAccountID: "cc", Date: d, Amount: money.New(-10000, "USD")},
		{ID: "in", AccountID: "cc", TransferAccountID: "chk", Date: d, Amount: money.New(-10000, "USD")},
	}
	res, err := MonthCashflowWithAccounts(d, time.Sunday, pair, nil, accounts, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("MonthCashflowWithAccounts: %v", err)
	}
	var moved int64
	for _, w := range res.Weeks {
		moved += w.Total.Moved
	}
	if moved != 10000 {
		t.Fatalf("positive-owed pair Moved = %d, want 10000", moved)
	}
}
