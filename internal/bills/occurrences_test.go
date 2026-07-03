// SPDX-License-Identifier: MIT

package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestOccurrencesWithinProjectsMonthlyLiability(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	accts := []domain.Account{{
		ID: "card", Name: "Card", Class: domain.ClassLiability,
		DueDayOfMonth: 15, MinPayment: money.New(-5000, "USD"),
	}}
	got := OccurrencesWithin(accts, nil, now, now.AddDate(0, 0, 60))
	if len(got) != 2 {
		t.Fatalf("60-day window should hold 2 monthly occurrences, got %d", len(got))
	}
	want := []time.Time{
		time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
	}
	for i, w := range want {
		if !got[i].DueDate.Equal(w) {
			t.Errorf("occurrence %d = %v, want %v", i, got[i].DueDate, w)
		}
	}
}

func TestOccurrencesWithinClampsMonthEnd(t *testing.T) {
	// A due-day of 31 must land on Feb 28 in a non-leap February, then return
	// to the 31st in March — the classic clamping round-trip.
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	accts := []domain.Account{{
		ID: "loan", Name: "Loan", Class: domain.ClassLiability,
		DueDayOfMonth: 31, MinPayment: money.New(-10000, "USD"),
	}}
	got := OccurrencesWithin(accts, nil, now, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	want := []time.Time{
		time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	}
	if len(got) != len(want) {
		t.Fatalf("got %d occurrences, want %d", len(got), len(want))
	}
	for i, w := range want {
		if !got[i].DueDate.Equal(w) {
			t.Errorf("occurrence %d = %v, want %v", i, got[i].DueDate, w)
		}
	}
}

func TestOccurrencesWithinStepsRecurringCadence(t *testing.T) {
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	recs := []domain.Recurring{
		{ID: "gym", Label: "Gym", Amount: money.New(-3000, "USD"), Cadence: domain.CadenceWeekly,
			NextDue: time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)},
		{ID: "pay", Label: "Paycheck", Amount: money.New(250000, "USD"), Cadence: domain.CadenceBiweekly,
			NextDue: time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)}, // income — skipped
	}
	got := OccurrencesWithin(nil, recs, now, now.AddDate(0, 0, 28))
	if len(got) != 4 {
		t.Fatalf("4 weekly occurrences fit in 28 days (Jul 6/13/20/27), got %d", len(got))
	}
	for i, b := range got {
		want := time.Date(2026, 7, 6+7*i, 0, 0, 0, 0, time.UTC)
		if !b.DueDate.Equal(want) || b.Autopay {
			t.Errorf("occurrence %d = %v (autopay=%v), want %v", i, b.DueDate, b.Autopay, want)
		}
	}
}

func TestOccurrencesWithinFirstMatchesUpcomingAll(t *testing.T) {
	// The first occurrence per bill must carry the same (account, date, name)
	// identity as UpcomingAll's single occurrence, so plan lookups keyed by
	// occurrence ID work across both views.
	now := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	accts := []domain.Account{{
		ID: "card", Name: "Card", Class: domain.ClassLiability,
		DueDayOfMonth: 20, MinPayment: money.New(-5000, "USD"),
	}}
	recs := []domain.Recurring{
		{ID: "rent", Label: "Rent", Amount: money.New(-90000, "USD"), Cadence: domain.CadenceMonthly,
			NextDue: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), Autopay: true},
	}
	single := UpcomingAll(accts, recs, now)
	multi := OccurrencesWithin(accts, recs, now, now.AddDate(0, 0, 60))
	for _, s := range single {
		found := false
		for _, m := range multi {
			if m.AccountID == s.AccountID && m.Name == s.Name && m.DueDate.Equal(s.DueDate) && m.Autopay == s.Autopay {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("UpcomingAll occurrence %s@%v missing from OccurrencesWithin", s.Name, s.DueDate)
		}
	}
}
