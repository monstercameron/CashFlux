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

// ─── C340: the dual bill identity must not escape any surface ────────────────

// sampleObligation builds the shape the V-sweep found double-counted: a student
// loan whose statement bill and monthly recurring flow are the same $320 on the
// 5th.
func sampleObligation() ([]domain.Account, []domain.Recurring) {
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accounts := []domain.Account{{
		ID: "sloan", Name: "Priya's Student Loan", Class: domain.ClassLiability,
		Type: domain.TypeLoan, Currency: "USD", OpeningBalance: usd(-3_800_000),
		DueDayOfMonth: 5, MinPayment: usd(32_000),
	}}
	recurring := []domain.Recurring{{
		ID: "rec-studentloan", Label: "Student loan payment", Amount: usd(-32_000),
		Cadence: domain.CadenceMonthly, NextDue: onDay(2026, time.July, 5),
		AccountID: "checking",
	}}
	return accounts, recurring
}

// TestOccurrencesWithinDedupesByDefault is the fix for C340.
//
// The dedupe existed but was an opt-in wrapper, and three of its four callers —
// the bills calendar, the pay-ahead planner, the payday preflight — had simply
// forgotten to apply it. "Total due soon" read $8,814.00, the calendar showed
// two badges on the 5th, and the counts were inflated. A correctness rule a
// caller can forget is not a rule, so the projection dedupes itself.
func TestOccurrencesWithinDedupesByDefault(t *testing.T) {
	accounts, recurring := sampleObligation()
	now := onDay(2026, time.July, 1)
	until := onDay(2026, time.September, 30)

	got := OccurrencesWithin(accounts, recurring, now, until)

	byDate := map[string]int{}
	var total int64
	for _, b := range got {
		byDate[b.DueDate.Format("2006-01-02")]++
		total += b.Amount.Amount
	}
	for date, n := range byDate {
		if n != 1 {
			t.Errorf("%s has %d obligations, want 1 — the statement bill and the flow that "+
				"pays it are one payment (C340)", date, n)
		}
	}
	// Jul 5, Aug 5, Sep 5 — three occurrences, $320 each, not six.
	if len(got) != 3 {
		t.Fatalf("got %d occurrences, want 3", len(got))
	}
	if want := int64(96_000); total != want {
		t.Errorf("window total = %d, want %d (a double-count inflates every headline "+
			"built on this list)", total, want)
	}
	for _, b := range got {
		if b.AnchorAccountID != "sloan" {
			t.Errorf("occurrence %s has anchor %q, want \"sloan\" — the merged row must keep "+
				"the liability it absorbed", b.DueDate.Format("2006-01-02"), b.AnchorAccountID)
		}
		if b.Name != "Student loan payment" {
			t.Errorf("survivor name = %q, want the household's own label", b.Name)
		}
	}
}

// TestUpcomingAllUsesTheSameMergeRule pins the survivor.
//
// UpcomingAll used to run its own dedupe with the OPPOSITE survivor: it dropped
// the recurring flow and kept the liability's statement row, recording nothing
// about what it had absorbed. So /bills and the recurring agenda disagreed about
// which identity a merged obligation wears, and only one of them could say what
// it covered.
func TestUpcomingAllUsesTheSameMergeRule(t *testing.T) {
	accounts, recurring := sampleObligation()
	now := onDay(2026, time.July, 1)

	got := UpcomingAll(accounts, recurring, now)
	if len(got) != 1 {
		t.Fatalf("got %d bills, want 1", len(got))
	}
	b := got[0]
	if _, isRecurring := RecurringIDFromAccount(b.AccountID); !isRecurring {
		t.Errorf("survivor is %q, want the recurring flow — it carries the household's "+
			"label, its posting mode, and the schedule \"mark paid\" advances (C340)", b.AccountID)
	}
	if b.AnchorAccountID != "sloan" {
		t.Errorf("anchor = %q, want \"sloan\"", b.AnchorAccountID)
	}

	// And the same merge the agenda sees.
	merged := OccurrencesWithin(accounts, recurring, now, onDay(2026, time.July, 31))
	if len(merged) != 1 || merged[0].AccountID != b.AccountID {
		t.Errorf("UpcomingAll and OccurrencesWithin disagree about the surviving identity")
	}
}

// TestIdentitiesCoverBothSidesOfAMerge protects paid marks across the change.
//
// Marks are keyed by (bill id, due date). Flipping which identity survives would
// have made every mark recorded under the old one vanish — a bill the household
// had ticked off silently reappearing as unpaid.
func TestIdentitiesCoverBothSidesOfAMerge(t *testing.T) {
	accounts, recurring := sampleObligation()
	got := UpcomingAll(accounts, recurring, onDay(2026, time.July, 1))
	ids := got[0].Identities()
	if len(ids) != 2 {
		t.Fatalf("Identities() = %v, want both the flow and the liability it absorbed", ids)
	}
	want := map[string]bool{"recurring:rec-studentloan": true, "sloan": true}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected identity %q", id)
		}
	}

	// An unmerged bill has exactly one identity — no phantom second lookup.
	plain := Bill{AccountID: "card"}
	if ids := plain.Identities(); len(ids) != 1 || ids[0] != "card" {
		t.Errorf("Identities() on an unmerged bill = %v, want [card]", ids)
	}
}

// TestUnrelatedBillsOnTheSameDayStaySeparate is the false-positive guard.
//
// The merge key is deliberately exact — same currency, same amount, same date,
// and the flow must repeat monthly. A weekly flow that coincides once, or a
// different amount, is a coincidence and must survive as its own obligation.
func TestUnrelatedBillsOnTheSameDayStaySeparate(t *testing.T) {
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accounts := []domain.Account{{
		ID: "card", Name: "Rewards Card", Class: domain.ClassLiability,
		Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(-380_000),
		DueDayOfMonth: 5, MinPayment: usd(22_000),
	}}
	recurring := []domain.Recurring{
		// Same day, different amount — a real second obligation.
		{ID: "rec-subs", Label: "Streaming & apps", Amount: usd(-3_800),
			Cadence: domain.CadenceMonthly, NextDue: onDay(2026, time.July, 5)},
		// Same day, same amount, but weekly — coincidence, not a statement mirror.
		{ID: "rec-weekly", Label: "Weekly something", Amount: usd(-22_000),
			Cadence: domain.CadenceWeekly, NextDue: onDay(2026, time.July, 5)},
	}
	got := UpcomingAll(accounts, recurring, onDay(2026, time.July, 1))
	if len(got) != 3 {
		t.Fatalf("got %d bills, want 3 — merging unrelated bills that share a date is a "+
			"worse failure than listing two", len(got))
	}
}
