// SPDX-License-Identifier: MIT

package freshness

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

var now = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func daysAgo(n int) time.Time { return now.AddDate(0, 0, -n) }

func acct(t domain.AccountType, asOf time.Time) domain.Account {
	return domain.Account{ID: "a", Type: t, BalanceAsOf: asOf}
}

func TestDefaultWindows(t *testing.T) {
	w := DefaultWindows()
	if d, ok := w.WindowDays(domain.TypeCreditCard); !ok || d != 14 {
		t.Errorf("credit card window = %d ok=%v, want 14", d, ok)
	}
	if d, _ := w.WindowDays(domain.TypeSavings); d != 45 {
		t.Errorf("savings window = %d, want 45", d)
	}
	// C222/C226: slow-moving asset values get long windows so we don't nag them
	// like a reconciled cash balance.
	if d, _ := w.WindowDays(domain.TypeInvestment); d != 120 {
		t.Errorf("investment window = %d, want 120", d)
	}
	if d, _ := w.WindowDays(domain.TypeOther); d != 180 {
		t.Errorf("other-asset window = %d, want 180", d)
	}
}

func TestIsStale(t *testing.T) {
	w := DefaultWindows()
	tests := []struct {
		name string
		acc  domain.Account
		want bool
	}{
		{"credit card 20d old (window 14)", acct(domain.TypeCreditCard, daysAgo(20)), true},
		{"credit card 10d old (window 14)", acct(domain.TypeCreditCard, daysAgo(10)), false},
		{"checking 20d old (window 30)", acct(domain.TypeChecking, daysAgo(20)), false},
		{"checking 40d old (window 30)", acct(domain.TypeChecking, daysAgo(40)), true},
		// C222/C226: slow-moving asset values aren't nagged like cash.
		{"investment 90d old (window 120)", acct(domain.TypeInvestment, daysAgo(90)), false},
		{"investment 130d old (window 120)", acct(domain.TypeInvestment, daysAgo(130)), true},
		{"other-asset 120d old (window 180)", acct(domain.TypeOther, daysAgo(120)), false},
		{"other-asset 200d old (window 180)", acct(domain.TypeOther, daysAgo(200)), true},
		{"never confirmed", acct(domain.TypeSavings, time.Time{}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStale(tt.acc, w, now); got != tt.want {
				t.Errorf("IsStale = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsStaleArchivedAndExempt(t *testing.T) {
	w := DefaultWindows()
	archived := acct(domain.TypeCreditCard, daysAgo(100))
	archived.Archived = true
	if IsStale(archived, w, now) {
		t.Error("archived account should never be stale")
	}

	// Exempt a type by overriding its window to 0.
	exempt := w.Merge(Windows{domain.TypeCreditCard: 0})
	if IsStale(acct(domain.TypeCreditCard, daysAgo(100)), exempt, now) {
		t.Error("window 0 should exempt the type")
	}

	// Untracked type is never stale.
	untracked := Windows{}
	if IsStale(acct(domain.TypeChecking, daysAgo(100)), untracked, now) {
		t.Error("untracked type should never be stale")
	}
}

func TestIsStalePerAccountExemptAndSnooze(t *testing.T) {
	w := DefaultWindows()

	// A per-account exemption beats any window.
	ex := acct(domain.TypeCreditCard, daysAgo(100))
	ex.FreshnessExempt = true
	if IsStale(ex, w, now) {
		t.Error("freshness-exempt account should never be stale")
	}

	// A snooze suppresses staleness until its date…
	sn := acct(domain.TypeCreditCard, daysAgo(100))
	sn.FreshnessSnoozeUntil = now.AddDate(0, 0, 7)
	if IsStale(sn, w, now) {
		t.Error("snoozed account should not be stale before the snooze date")
	}
	// …and expires: at/after the date the account is stale again.
	if !IsStale(sn, w, now.AddDate(0, 0, 7)) {
		t.Error("snooze should expire on its date")
	}
	// A past snooze changes nothing.
	past := acct(domain.TypeCreditCard, daysAgo(100))
	past.FreshnessSnoozeUntil = daysAgo(1)
	if !IsStale(past, w, now) {
		t.Error("expired snooze should leave the account stale")
	}
}

func TestDaysSinceUpdate(t *testing.T) {
	if got := DaysSinceUpdate(acct(domain.TypeChecking, daysAgo(7)), now); got != 7 {
		t.Errorf("DaysSinceUpdate = %d, want 7", got)
	}
	if got := DaysSinceUpdate(acct(domain.TypeChecking, time.Time{}), now); got != -1 {
		t.Errorf("DaysSinceUpdate(never) = %d, want -1", got)
	}
}

func TestStaleAccounts(t *testing.T) {
	w := DefaultWindows()
	accounts := []domain.Account{
		acct(domain.TypeCreditCard, daysAgo(20)), // stale
		acct(domain.TypeChecking, daysAgo(5)),    // fresh
		acct(domain.TypeLoan, daysAgo(30)),       // stale
	}
	got := StaleAccounts(accounts, w, now)
	if len(got) != 2 {
		t.Fatalf("stale count = %d, want 2", len(got))
	}
}

func TestDismissalsHideOnlyCurrentStaleState(t *testing.T) {
	w := DefaultWindows()
	stale := domain.Account{ID: "cc", Type: domain.TypeCreditCard, BalanceAsOf: daysAgo(20)}
	other := domain.Account{ID: "checking", Type: domain.TypeChecking, BalanceAsOf: daysAgo(40)}

	d := Dismissals{}.Dismiss([]domain.Account{stale}, now)
	got := VisibleStaleAccounts([]domain.Account{stale, other}, w, d, now)
	if len(got) != 1 || got[0].ID != "checking" {
		t.Fatalf("visible stale after dismissal = %+v, want only checking", got)
	}

	updated := stale
	updated.BalanceAsOf = now.Add(time.Hour)
	if d.IsDismissed(updated) {
		t.Fatal("a later balance update should clear the old dismissal")
	}

	later := updated
	future := updated.BalanceAsOf.AddDate(0, 0, 20)
	got = VisibleStaleAccounts([]domain.Account{later}, w, d, future)
	if len(got) != 1 || got[0].ID != "cc" {
		t.Fatalf("later stale state should be visible again, got %+v", got)
	}
}

func TestMergeDoesNotMutate(t *testing.T) {
	base := DefaultWindows()
	_ = base.Merge(Windows{domain.TypeChecking: 99})
	if d, _ := base.WindowDays(domain.TypeChecking); d != 30 {
		t.Errorf("base mutated: checking = %d, want 30", d)
	}
}

// EC-20: a page derived from balances nobody has confirmed in two months looks
// exactly like one confirmed this morning. The numbers are equally crisp and one
// of them is fiction.
func TestCoverageStandingHasFourStates(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	w := DefaultWindows()
	fresh := func(id string) domain.Account {
		return domain.Account{ID: id, Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -1)}
	}
	stale := func(id string) domain.Account {
		return domain.Account{ID: id, Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -120)}
	}
	all := Measure([]domain.Account{fresh("a"), fresh("b"), fresh("c")}, w, now)
	if got := all.Standing(); got != StandingCurrent {
		t.Errorf("all fresh = %q, want %q", got, StandingCurrent)
	}
	// One stale in three must not turn a page into an alarm.
	mostly := Measure([]domain.Account{fresh("a"), fresh("b"), fresh("c"), stale("d")}, w, now)
	if got := mostly.Standing(); got != StandingMostly {
		t.Errorf("3 of 4 fresh = %q, want %q", got, StandingMostly)
	}
	if !mostly.Trustworthy() {
		t.Error("a mostly-current page was called untrustworthy")
	}
	half := Measure([]domain.Account{fresh("a"), stale("b"), stale("c")}, w, now)
	if got := half.Standing(); got != StandingIncomplete {
		t.Errorf("1 of 3 fresh = %q, want %q", got, StandingIncomplete)
	}
	worst := Measure([]domain.Account{fresh("a"), stale("b"), stale("c"), stale("d")}, w, now)
	if got := worst.Standing(); got != StandingStale {
		t.Errorf("1 of 4 fresh = %q, want %q", got, StandingStale)
	}
	if worst.Trustworthy() {
		t.Error("a mostly-stale page was called trustworthy")
	}
}

// "Nothing is stale" and "there is nothing" are different statements, and only
// the first is reassuring.
func TestNoAccountsIsUnknownNotCurrent(t *testing.T) {
	c := Measure(nil, DefaultWindows(), time.Now())
	if got := c.Standing(); got != StandingUnknown {
		t.Errorf("no accounts = %q, want %q", got, StandingUnknown)
	}
	if c.Trustworthy() {
		t.Error("an empty page was reported as trustworthy")
	}
}

// "We have not checked since March" and "we have never checked" call for
// different actions.
func TestNeverConfirmedIsCountedApartFromStale(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	c := Measure([]domain.Account{
		{ID: "never", Type: domain.TypeChecking},
		{ID: "old", Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -200)},
	}, DefaultWindows(), now)
	if c.Unconfirmed != 1 || c.Stale != 1 {
		t.Errorf("coverage = %+v, want one of each", c)
	}
}

// Archived accounts are not part of any page's figures, and counting them would
// drag every household's standing down for accounts they deliberately closed.
func TestArchivedAccountsAreNotCounted(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	c := Measure([]domain.Account{
		{ID: "live", Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -1)},
		{ID: "gone", Type: domain.TypeChecking, Archived: true},
	}, DefaultWindows(), now)
	if c.Total != 1 || c.Standing() != StandingCurrent {
		t.Errorf("coverage = %+v standing = %q", c, c.Standing())
	}
}

func TestReconciledIsCounted(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	c := Measure([]domain.Account{
		{ID: "a", Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -1),
			Reconciliations: []domain.Reconciliation{{At: now.AddDate(0, 0, -3)}}},
		{ID: "b", Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -1)},
	}, DefaultWindows(), now)
	if c.Reconciled != 1 {
		t.Errorf("reconciled = %d, want 1", c.Reconciled)
	}
}
