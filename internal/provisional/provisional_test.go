// SPDX-License-Identifier: MIT

package provisional

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func on(day int) time.Time { return time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC) }

func checkpoint(id, acct string, minor int64, asOf time.Time) domain.Transaction {
	t, ok := New(id, acct, "checkpoint", 0, minor, "USD", asOf, asOf)
	if !ok {
		panic("fixture produced no checkpoint")
	}
	return t
}

// A checkpoint is real for the balance and invisible to reporting. Both halves
// matter: drop the first and the account is wrong, drop the second and the app
// invents a payday.
func TestACheckpointCountsInBalancesAndNotInReports(t *testing.T) {
	cp, ok := New("c1", "chk", "Balance checkpoint", 100000, 125000, "USD", on(15), on(15))
	if !ok {
		t.Fatal("New reported nothing to record for a $250 gap")
	}
	if cp.Amount.Amount != 25000 {
		t.Errorf("amount = %d, want the 25000 difference", cp.Amount.Amount)
	}
	if !cp.ExcludeFromReports {
		t.Errorf("a checkpoint counts as income or spending — that is the bug it replaces")
	}
	if !cp.IsBalanceCheckpoint() {
		t.Errorf("the row does not identify itself as a checkpoint")
	}
	if cp.CountsInReports() {
		t.Errorf("CountsInReports = true")
	}

	acc := domain.Account{ID: "chk", Currency: "USD", Class: domain.ClassAsset,
		OpeningBalance: money.New(100000, "USD")}
	bal, err := ledger.Balance(acc, []domain.Transaction{cp})
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal.Amount != 125000 {
		t.Errorf("balance = %d, want the account to actually reach the entered figure", bal.Amount)
	}
}

// A balance read on Friday and typed on Monday belongs to Friday, or every figure
// computed over a date range files it in the wrong one.
func TestTheRowIsDatedAsOfNotAsEntered(t *testing.T) {
	asOf, posted := on(14), on(17)
	cp, _ := New("c1", "chk", "x", 0, 5000, "USD", asOf, posted)
	if !cp.Date.Equal(asOf) {
		t.Errorf("Date = %v, want the as-of date %v", cp.Date, asOf)
	}
	if !cp.BalanceCheckpointAt.Equal(asOf) {
		t.Errorf("BalanceCheckpointAt = %v, want %v", cp.BalanceCheckpointAt, asOf)
	}
}

func TestNoGapMeansNoRow(t *testing.T) {
	if _, ok := New("c1", "chk", "x", 125000, 125000, "USD", on(15), on(15)); ok {
		t.Errorf("wrote a zero-amount row, which is a permanent artefact saying nothing")
	}
}

func TestAMissingAsOfFallsBackToTheEntryDate(t *testing.T) {
	posted := on(17)
	cp, _ := New("c1", "chk", "x", 0, 5000, "USD", time.Time{}, posted)
	if !cp.Date.Equal(posted) {
		t.Errorf("Date = %v, want the entry date when no as-of was given", cp.Date)
	}
}

func TestCheckpointsAreTaggedAndFindable(t *testing.T) {
	cp, _ := New("c1", "chk", "x", 0, 5000, "USD", on(15), on(15))
	found := false
	for _, tag := range cp.Tags {
		if tag == Tag {
			found = true
		}
	}
	if !found {
		t.Errorf("tags = %v, want %q so the ledger's own filters can find it", cp.Tags, Tag)
	}
}

// Only one guess per account stands at a time; checking a balance twice must not
// leave two adjustments both claiming to explain the same gap.
func TestOpenIsTheNewestAndTheRestAreStale(t *testing.T) {
	txns := []domain.Transaction{
		checkpoint("old", "chk", 1000, on(3)),
		checkpoint("new", "chk", 2000, on(19)),
		checkpoint("mid", "chk", 1500, on(11)),
		checkpoint("other", "sav", 9999, on(20)),
	}
	got, ok := Open("chk", txns)
	if !ok || got.ID != "new" {
		t.Fatalf("Open = %+v ok=%v, want the newest", got.ID, ok)
	}
	stale := Stale("chk", txns)
	if len(stale) != 2 {
		t.Fatalf("Stale = %d, want the two older ones", len(stale))
	}
	if stale[0].ID != "old" || stale[1].ID != "mid" {
		t.Errorf("stale order = %s,%s, want oldest first", stale[0].ID, stale[1].ID)
	}
	// Another account's checkpoint is not this account's business.
	if o, _ := Open("sav", txns); o.ID != "other" {
		t.Errorf("accounts are not kept apart")
	}
}

func TestNoCheckpointsIsNotAnError(t *testing.T) {
	if _, ok := Open("chk", nil); ok {
		t.Errorf("Open reported a checkpoint on an empty ledger")
	}
	if got := Stale("chk", []domain.Transaction{checkpoint("a", "chk", 1, on(2))}); got != nil {
		t.Errorf("Stale = %+v, want none when only one stands", got)
	}
}

// The half that was missing entirely: reconciling retires the guess it replaces.
func TestReconcilingRetiresTheCheckpointsItCovers(t *testing.T) {
	txns := []domain.Transaction{
		checkpoint("july", "chk", 1000, on(5)),
		checkpoint("aug", "chk", 2000, on(25)),
		checkpoint("other", "sav", 3000, on(5)),
	}
	got := SupersededBy("chk", txns, on(15))
	if len(got) != 1 || got[0] != "july" {
		t.Fatalf("superseded = %v, want only the checkpoint the statement covers", got)
	}
}

// A checkpoint dated ON the statement's closing date is covered by it. Statement
// dates are date-only and transaction dates carry a time, so a mid-afternoon
// checkpoint on the closing day must not survive by six hours.
func TestACheckpointOnTheClosingDateIsCovered(t *testing.T) {
	afternoon := time.Date(2026, 8, 15, 16, 30, 0, 0, time.UTC)
	txns := []domain.Transaction{checkpoint("c", "chk", 1000, afternoon)}
	if got := SupersededBy("chk", txns, on(15)); len(got) != 1 {
		t.Errorf("superseded = %v, want the same-day checkpoint retired", got)
	}
}

// A checkpoint for a period the statement has not reached is still needed.
func TestALaterCheckpointSurvivesReconciliation(t *testing.T) {
	txns := []domain.Transaction{checkpoint("future", "chk", 1000, on(28))}
	if got := SupersededBy("chk", txns, on(15)); len(got) != 0 {
		t.Errorf("superseded = %v, want the later guess kept", got)
	}
}

// The figure that explains why a balance and a period's activity do not add up.
func TestTotalMinorSumsOnlyCheckpoints(t *testing.T) {
	ordinary := domain.Transaction{ID: "x", AccountID: "chk", Amount: money.New(-9999, "USD"), Date: on(4)}
	txns := []domain.Transaction{
		checkpoint("a", "chk", 1000, on(3)),
		checkpoint("b", "chk", 2500, on(9)),
		ordinary,
	}
	if got := TotalMinor(txns); got != 3500 {
		t.Errorf("TotalMinor = %d, want 3500 — ordinary spending must not be counted", got)
	}
}

// Caught by the component test that first used this package: a transaction must
// describe itself or the store refuses it, so New must never hand back a row
// that cannot be written.
func TestNewAlwaysProducesAWritableRow(t *testing.T) {
	for _, desc := range []string{"", "   ", "\t"} {
		cp, ok := New("c1", "chk", desc, 0, 5000, "USD", on(15), on(15))
		if !ok {
			t.Fatalf("New(%q) reported nothing to record", desc)
		}
		if cp.Desc == "" {
			t.Errorf("New(%q) produced a row with no description; the store rejects it", desc)
		}
		if cp.Desc != FallbackDesc {
			t.Errorf("New(%q) desc = %q, want the fallback", desc, cp.Desc)
		}
	}
	cp, _ := New("c1", "chk", "Balance checkpoint (awaiting statement)", 0, 5000, "USD", on(15), on(15))
	if cp.Desc != "Balance checkpoint (awaiting statement)" {
		t.Errorf("a supplied description was overwritten: %q", cp.Desc)
	}
}
