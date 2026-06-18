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
