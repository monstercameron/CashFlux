// SPDX-License-Identifier: MIT

package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// onDay builds a date for the test table.
func onDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// TestDedupeObligations guards the dual-bill-identity invariant: a liability
// account's statement bill and the monthly recurring flow that pays it are ONE
// obligation and must not both reach a surface (the agenda double-counted them
// before this existed).
func TestDedupeObligations(t *testing.T) {
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	monthly := domain.Recurring{ID: "rec-carpay-m", Cadence: domain.CadenceMonthly}
	weekly := domain.Recurring{ID: "rec-weekly", Cadence: domain.CadenceWeekly}

	tests := []struct {
		name      string
		bills     []Bill
		recurring []domain.Recurring
		wantNames []string
		// wantAnchor maps a surviving bill's AccountID to the anchor it absorbed.
		wantAnchor map[string]string
	}{
		{
			name: "liability statement and its monthly flow collapse to one row",
			bills: []Bill{
				{AccountID: "acct-carloan-marcus", Name: "Marcus's Car Loan", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
				{AccountID: "recurring:rec-carpay-m", Name: "Car payment (Marcus)", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
			},
			recurring:  []domain.Recurring{monthly},
			wantNames:  []string{"Car payment (Marcus)"},
			wantAnchor: map[string]string{"recurring:rec-carpay-m": "acct-carloan-marcus"},
		},
		{
			name: "every occurrence in the window is deduped, not just the first",
			bills: []Bill{
				{AccountID: "acct-carloan-marcus", Name: "Marcus's Car Loan", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
				{AccountID: "recurring:rec-carpay-m", Name: "Car payment (Marcus)", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
				{AccountID: "acct-carloan-marcus", Name: "Marcus's Car Loan", Amount: usd(62000), DueDate: onDay(2026, 8, 15)},
				{AccountID: "recurring:rec-carpay-m", Name: "Car payment (Marcus)", Amount: usd(62000), DueDate: onDay(2026, 8, 15)},
			},
			recurring: []domain.Recurring{monthly},
			wantNames: []string{"Car payment (Marcus)", "Car payment (Marcus)"},
		},
		{
			name: "a different amount on the same day is a different obligation",
			bills: []Bill{
				{AccountID: "acct-carloan-marcus", Name: "Marcus's Car Loan", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
				{AccountID: "recurring:rec-carpay-m", Name: "Car payment (Marcus)", Amount: usd(48000), DueDate: onDay(2026, 7, 15)},
			},
			recurring: []domain.Recurring{monthly},
			wantNames: []string{"Marcus's Car Loan", "Car payment (Marcus)"},
		},
		{
			name: "a different day is a different obligation",
			bills: []Bill{
				{AccountID: "acct-carloan-marcus", Name: "Marcus's Car Loan", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
				{AccountID: "recurring:rec-carpay-m", Name: "Car payment (Marcus)", Amount: usd(62000), DueDate: onDay(2026, 7, 17)},
			},
			recurring: []domain.Recurring{monthly},
			wantNames: []string{"Marcus's Car Loan", "Car payment (Marcus)"},
		},
		{
			name: "a non-monthly flow coinciding once is a coincidence, not a duplicate",
			bills: []Bill{
				{AccountID: "acct-carloan-marcus", Name: "Marcus's Car Loan", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
				{AccountID: "recurring:rec-weekly", Name: "Weekly thing", Amount: usd(62000), DueDate: onDay(2026, 7, 15)},
			},
			recurring: []domain.Recurring{weekly},
			wantNames: []string{"Marcus's Car Loan", "Weekly thing"},
		},
		{
			name: "two unrelated statement bills are both kept",
			bills: []Bill{
				{AccountID: "acct-card", Name: "Rewards Credit Card", Amount: usd(22000), DueDate: onDay(2026, 7, 22)},
				{AccountID: "acct-mortgage", Name: "Mortgage", Amount: usd(148000), DueDate: onDay(2026, 7, 1)},
			},
			wantNames: []string{"Rewards Credit Card", "Mortgage"},
		},
		{
			name:      "empty input is empty output",
			bills:     nil,
			wantNames: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DedupeObligations(tc.bills, tc.recurring)
			if len(got) != len(tc.wantNames) {
				t.Fatalf("got %d bills, want %d: %+v", len(got), len(tc.wantNames), got)
			}
			for i, want := range tc.wantNames {
				if got[i].Name != want {
					t.Errorf("bill %d: got name %q, want %q", i, got[i].Name, want)
				}
			}
			for acct, wantAnchor := range tc.wantAnchor {
				found := false
				for _, b := range got {
					if b.AccountID != acct {
						continue
					}
					found = true
					if b.AnchorAccountID != wantAnchor {
						t.Errorf("bill %q: got anchor %q, want %q", acct, b.AnchorAccountID, wantAnchor)
					}
				}
				if !found {
					t.Errorf("expected a surviving bill with AccountID %q", acct)
				}
			}
		})
	}
}

// TestDedupeObligationsPreservesUnmergedFields checks the merge only annotates
// the surviving row and leaves everything else untouched.
func TestDedupeObligationsPreservesUnmergedFields(t *testing.T) {
	in := []Bill{
		{AccountID: "acct-carloan-marcus", Name: "Marcus's Car Loan", Amount: money.New(62000, "USD"), DueDate: onDay(2026, 7, 15)},
		{AccountID: "recurring:rec-carpay-m", Name: "Car payment (Marcus)", Amount: money.New(62000, "USD"), DueDate: onDay(2026, 7, 15), Autopay: true, DaysUntil: 14},
	}
	got := DedupeObligations(in, []domain.Recurring{{ID: "rec-carpay-m", Cadence: domain.CadenceMonthly}})
	if len(got) != 1 {
		t.Fatalf("got %d bills, want 1", len(got))
	}
	if !got[0].Autopay || got[0].DaysUntil != 14 {
		t.Errorf("surviving bill lost its fields: %+v", got[0])
	}
	if got[0].AnchorAccountID != "acct-carloan-marcus" {
		t.Errorf("got anchor %q, want acct-carloan-marcus", got[0].AnchorAccountID)
	}
}

// TestRecurringIDFromAccount covers the "recurring:<id>" convention both ways.
func TestRecurringIDFromAccount(t *testing.T) {
	if id, ok := RecurringIDFromAccount("recurring:rec-gym"); !ok || id != "rec-gym" {
		t.Errorf("got (%q,%v), want (rec-gym,true)", id, ok)
	}
	if _, ok := RecurringIDFromAccount("acct-card"); ok {
		t.Error("a real account id must not read as recurring-derived")
	}
}
