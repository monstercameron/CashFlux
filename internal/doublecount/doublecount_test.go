// SPDX-License-Identifier: MIT

package doublecount

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func tx(id, acct, payee string, when time.Time, minor int64) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Payee: payee, Desc: payee,
		Date: when, Amount: money.New(minor, "USD"),
	}
}

// Nothing left the household, but the ledger shows $400 of spending AND $400 of
// income — every report built on either is wrong.
func TestATransferEnteredAsTwoTransactionsIsFound(t *testing.T) {
	out := tx("a", "checking", "Transfer to savings", day(2026, time.August, 3), -40_000)
	in := tx("b", "savings", "From checking", day(2026, time.August, 4), 40_000)
	r := Find([]domain.Transaction{out, in})
	if len(r.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Kind != KindMirror {
		t.Errorf("kind = %q, want %q", f.Kind, KindMirror)
	}
	if f.AmountMinor != 40_000 || f.DaysApart != 1 {
		t.Errorf("finding = %+v, want 40000 one day apart", f)
	}
	if r.MirrorMinor != 40_000 {
		t.Errorf("mirror total = %d, want 40000", r.MirrorMinor)
	}
}

// A transfer is already modelled correctly — it is the FIX, not the problem.
func TestARealTransferIsNotFlagged(t *testing.T) {
	a := tx("a", "checking", "Move", day(2026, time.August, 3), -40_000)
	a.TransferAccountID = "savings"
	b := tx("b", "savings", "Move", day(2026, time.August, 3), 40_000)
	b.TransferAccountID = "checking"
	if r := Find([]domain.Transaction{a, b}); r.Any() {
		t.Errorf("a properly recorded transfer was flagged: %+v", r.Findings)
	}
}

// Repeated charges in ONE account are internal/dedupe's question. A second,
// slightly different rule here would eventually disagree with the canonical one
// about the same two rows, and two screens contradicting each other about
// whether something is a duplicate is worse than either rule alone.
func TestRepeatedChargesInOneAccountAreLeftToDedupe(t *testing.T) {
	a := tx("a", "card", "Coffee Shop", day(2026, time.August, 3), -1_500)
	b := tx("b", "card", "Coffee Shop", day(2026, time.August, 3), -1_500)
	if r := Find([]domain.Transaction{a, b}); r.Any() {
		t.Errorf("same-account duplicates were claimed here: %+v", r.Findings)
	}
}

func TestTwoOutflowsFromDifferentAccountsAreNotAMirror(t *testing.T) {
	// Both leaving is not money moving between them.
	a := tx("a", "checking", "Rent", day(2026, time.August, 3), -40_000)
	b := tx("b", "savings", "Rent", day(2026, time.August, 3), -40_000)
	if r := Find([]domain.Transaction{a, b}); r.Any() {
		t.Errorf("two outflows were paired: %+v", r.Findings)
	}
}

func TestBeyondTheWindowIsNotTheSameMoney(t *testing.T) {
	a := tx("a", "checking", "Move", day(2026, time.August, 1), -40_000)
	b := tx("b", "savings", "Move", day(2026, time.August, 10), 40_000)
	if r := Find([]domain.Transaction{a, b}); r.Any() {
		t.Errorf("nine days apart was treated as one movement: %+v", r.Findings)
	}
}

func TestTheWindowEdgeIsInclusive(t *testing.T) {
	a := tx("a", "checking", "Move", day(2026, time.August, 1), -40_000)
	b := tx("b", "savings", "Move", day(2026, time.August, 1+WindowDays), 40_000)
	if r := Find([]domain.Transaction{a, b}); !r.Any() {
		t.Error("a gap of exactly the window must still be considered")
	}
}

func TestDifferentCurrenciesAreNeverTheSameMoney(t *testing.T) {
	// Converting to compare would invent a match out of an exchange rate.
	a := domain.Transaction{ID: "a", AccountID: "x", Payee: "P",
		Date: day(2026, time.August, 3), Amount: money.New(-40_000, "USD")}
	b := domain.Transaction{ID: "b", AccountID: "y", Payee: "P",
		Date: day(2026, time.August, 3), Amount: money.New(40_000, "EUR")}
	if r := Find([]domain.Transaction{a, b}); r.Any() {
		t.Errorf("two currencies sharing a number were paired: %+v", r.Findings)
	}
}

// Without single-claiming, three mirrored movements would produce overlapping
// pairings of the same money and read as a catastrophe.
func TestEachTransactionIsClaimedOnce(t *testing.T) {
	a := tx("a", "checking", "Move", day(2026, time.August, 1), -5_000)
	b := tx("b", "savings", "Move", day(2026, time.August, 1), 5_000)
	c := tx("c", "savings", "Move", day(2026, time.August, 2), 5_000)
	r := Find([]domain.Transaction{a, b, c})
	if len(r.Findings) != 1 {
		t.Errorf("findings = %d, want 1 - one pair plus a leftover", len(r.Findings))
	}
	if r.MirrorMinor != 5_000 {
		t.Errorf("mirror total = %d, want a single 5000", r.MirrorMinor)
	}
}

func TestExcludedRowsAreLeftAlone(t *testing.T) {
	// Excluded from reports means somebody has already decided about it.
	a := tx("a", "checking", "Move", day(2026, time.August, 3), -1_500)
	b := tx("b", "savings", "Move", day(2026, time.August, 3), 1_500)
	b.ExcludeFromReports = true
	if r := Find([]domain.Transaction{a, b}); r.Any() {
		t.Errorf("an excluded row was paired: %+v", r.Findings)
	}
}

func TestTheEarlierTransactionIsAlwaysFirst(t *testing.T) {
	// So the same pair is always reported the same way round.
	later := tx("z", "savings", "Move", day(2026, time.August, 4), 40_000)
	earlier := tx("a", "checking", "Move", day(2026, time.August, 3), -40_000)
	r := Find([]domain.Transaction{later, earlier})
	if len(r.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(r.Findings))
	}
	if r.Findings[0].A.ID != "a" {
		t.Errorf("first = %q, want the earlier transaction", r.Findings[0].A.ID)
	}
}

func TestAnEmptyLedgerFindsNothing(t *testing.T) {
	if r := Find(nil); r.Any() {
		t.Error("an empty ledger produced findings")
	}
}

func TestResultsAreStableAcrossRuns(t *testing.T) {
	txns := []domain.Transaction{
		tx("b", "savings", "Move", day(2026, time.August, 1), 5_000),
		tx("a", "checking", "Move", day(2026, time.August, 1), -5_000),
	}
	for i := range 5 {
		r := Find(txns)
		if len(r.Findings) != 1 || r.Findings[0].A.ID != "a" {
			t.Fatalf("run %d: %+v", i, r.Findings)
		}
	}
}
