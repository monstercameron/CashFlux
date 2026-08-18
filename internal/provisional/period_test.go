// SPDX-License-Identifier: MIT

package provisional

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func reconciled(id string, ty domain.AccountType, through time.Time) domain.Account {
	a := domain.Account{ID: id, Name: id, Type: ty, Class: ty.Class(), Currency: "USD"}
	if !through.IsZero() {
		a.Reconciliations = []domain.Reconciliation{{At: through, StatementDate: through,
			StatementBalance: money.New(1000, "USD")}}
	}
	return a
}

// The household's books are closed through the EARLIEST account, not the latest:
// one well-kept account must not certify months the others have not closed.
func TestClosedThroughTakesTheEarliestAccount(t *testing.T) {
	accs := []domain.Account{
		reconciled("chk", domain.TypeChecking, d(2026, time.July, 31)),
		reconciled("sav", domain.TypeSavings, d(2026, time.August, 15)),
		reconciled("card", domain.TypeCreditCard, d(2026, time.June, 30)),
	}
	got, ok := ClosedThrough(accs)
	if !ok {
		t.Fatal("ok = false with three reconciled accounts")
	}
	if !got.Equal(d(2026, time.June, 30)) {
		t.Errorf("ClosedThrough = %v, want the earliest (June 30)", got.Format("2006-01-02"))
	}
}

// One never-reconciled account leaves the books open, and says so at once.
func TestOneUnreconciledAccountLeavesTheBooksOpen(t *testing.T) {
	accs := []domain.Account{
		reconciled("chk", domain.TypeChecking, d(2026, time.July, 31)),
		reconciled("sav", domain.TypeSavings, time.Time{}),
	}
	if _, ok := ClosedThrough(accs); ok {
		t.Errorf("ok = true while an account has never been reconciled")
	}
}

// Accounts that never receive a statement must not hold the books open forever.
func TestUnreconcilableAndArchivedAccountsDoNotHoldTheBooksOpen(t *testing.T) {
	util := reconciled("fpl", domain.TypeUtilities, time.Time{})
	prop := reconciled("house", domain.TypeProperty, time.Time{})
	archived := reconciled("old", domain.TypeChecking, time.Time{})
	archived.Archived = true

	accs := []domain.Account{
		reconciled("chk", domain.TypeChecking, d(2026, time.July, 31)),
		util, prop, archived,
	}
	got, ok := ClosedThrough(accs)
	if !ok {
		t.Fatalf("a utility shell held the books open, waiting on a statement that never comes")
	}
	if !got.Equal(d(2026, time.July, 31)) {
		t.Errorf("ClosedThrough = %v, want July 31", got.Format("2006-01-02"))
	}
}

func TestNoAccountsAtAllIsNotClosed(t *testing.T) {
	if _, ok := ClosedThrough(nil); ok {
		t.Errorf("ok = true with no accounts")
	}
}

// "Closed through July" means a month ending on 1 August is settled. Comparing
// against the exclusive end instant instead would report July as provisional
// for exactly one day's worth of reasoning nobody would agree with.
func TestAMonthIsClosedByAStatementThroughItsLastDay(t *testing.T) {
	closedThrough := d(2026, time.July, 31)

	july := StatusOf(d(2026, time.August, 1), closedThrough, true)
	if july != Closed {
		t.Errorf("July = %s, want closed — the statement covers through July 31", july)
	}
	august := StatusOf(d(2026, time.September, 1), closedThrough, true)
	if august != Provisional {
		t.Errorf("August = %s, want provisional", august)
	}
	june := StatusOf(d(2026, time.July, 1), closedThrough, true)
	if june != Closed {
		t.Errorf("June = %s, want closed", june)
	}
}

func TestNothingIsClosedWithoutAReconciliation(t *testing.T) {
	if got := StatusOf(d(2026, time.August, 1), time.Time{}, false); got != Provisional {
		t.Errorf("status = %s, want provisional when nothing has been reconciled", got)
	}
}

// A month-to-date figure resting partly on a guess is the case worth captioning:
// it is the difference between "you spent less" and "the month is not over".
func TestStandingReportsWhatRestsOnAGuess(t *testing.T) {
	accs := []domain.Account{reconciled("chk", domain.TypeChecking, d(2026, time.July, 31))}
	augStart, augEnd := d(2026, time.August, 1), d(2026, time.September, 1)

	txns := []domain.Transaction{
		checkpoint("cp1", "chk", 25000, d(2026, time.August, 15)),
		checkpoint("cp2", "chk", 10000, d(2026, time.August, 20)),
		// July's checkpoint is outside the window.
		checkpoint("cp0", "chk", 99999, d(2026, time.July, 5)),
		{ID: "spend", AccountID: "chk", Amount: money.New(-4000, "USD"), Date: d(2026, time.August, 3)},
	}

	got := StandingOf(txns, augStart, augEnd, accs)
	if got.Status != Provisional {
		t.Errorf("status = %s, want provisional", got.Status)
	}
	if got.Checkpoints != 2 {
		t.Errorf("Checkpoints = %d, want the two inside August", got.Checkpoints)
	}
	if got.CheckpointMinor != 35000 {
		t.Errorf("CheckpointMinor = %d, want 35000", got.CheckpointMinor)
	}
	if !got.RestsOnAGuess() {
		t.Errorf("RestsOnAGuess = false")
	}
}

func TestAClosedMonthRestingOnNothingSaysSo(t *testing.T) {
	accs := []domain.Account{reconciled("chk", domain.TypeChecking, d(2026, time.July, 31))}
	got := StandingOf(
		[]domain.Transaction{{ID: "s", AccountID: "chk", Amount: money.New(-4000, "USD"), Date: d(2026, time.July, 3)}},
		d(2026, time.July, 1), d(2026, time.August, 1), accs)

	if got.Status != Closed {
		t.Errorf("status = %s, want closed", got.Status)
	}
	if got.RestsOnAGuess() {
		t.Errorf("RestsOnAGuess = true for a month with no checkpoints")
	}
	if got.CheckpointMinor != 0 {
		t.Errorf("CheckpointMinor = %d, want 0", got.CheckpointMinor)
	}
}
