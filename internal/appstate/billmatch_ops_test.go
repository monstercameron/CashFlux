// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/billmatch"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAutoMatchBillsOnPost(t *testing.T) {
	a := newApp(t, false)
	today := time.Now()
	due := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	if err := a.PutRecurring(domain.Recurring{
		ID: "r-net", Label: "Netflix", Cadence: domain.CadenceMonthly,
		NextDue: due, Amount: money.New(-1599, "USD"),
	}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}

	// A matching payment posts — should auto-create a bill-match link.
	txn := domain.Transaction{
		ID: "t1", AccountID: "acc", Date: due, Payee: "Netflix", Desc: "Netflix",
		Amount: money.New(-1650, "USD"),
	}
	if err := a.PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	l, ok := a.BillMatchForOccurrence("r-net", due)
	if !ok {
		t.Fatalf("expected bill-match link for occurrence; links=%+v", a.TxnLinks())
	}
	if l.Primary() != "t1" {
		t.Fatalf("matched txn = %q, want t1", l.Primary())
	}

	// Variance: paid $16.50 for an expected $15.99 = +51 over.
	v, ok := a.BillMatchVariance("r-net", due, 1599)
	if !ok || v != 51 {
		t.Fatalf("variance = %d ok=%v, want 51 true", v, ok)
	}

	// Paid-occurrence set carries the key.
	if !a.BillMatchPaidOccurrences()[billmatch.Key("r-net", due)] {
		t.Fatal("occurrence not in paid set")
	}

	// Unmatch releases it.
	if err := a.UnlinkBill("t1"); err != nil {
		t.Fatalf("UnlinkBill: %v", err)
	}
	if _, ok := a.BillMatchForOccurrence("r-net", due); ok {
		t.Fatal("occurrence still matched after unlink")
	}
}

func TestAutoMatchBillsAmbiguousLeftAlone(t *testing.T) {
	a := newApp(t, false)
	today := time.Now()
	due := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if err := a.PutRecurring(domain.Recurring{
		ID: "r-gym", Label: "Gym", Cadence: domain.CadenceMonthly,
		NextDue: due, Amount: money.New(-3000, "USD"),
	}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	// Two identical charges post in sequence. When the first posts it is the sole
	// candidate — genuinely unambiguous at that moment — so it matches. The second
	// finds the occurrence already settled and creates no second link (a txn
	// matches at most one occurrence; an occurrence at most one txn).
	for _, id := range []string{"g1", "g2"} {
		if err := a.PutTransaction(domain.Transaction{
			ID: id, AccountID: "acc", Date: due, Payee: "Gym", Desc: "Gym", Amount: money.New(-3000, "USD"),
		}); err != nil {
			t.Fatalf("PutTransaction %s: %v", id, err)
		}
	}
	l, ok := a.BillMatchForOccurrence("r-gym", due)
	if !ok || l.Primary() != "g1" {
		t.Fatalf("first charge should settle the occurrence, got %+v ok=%v", l, ok)
	}
	var billLinks int
	for _, lk := range a.TxnLinks() {
		if lk.Kind == domain.TxnLinkBillMatch {
			billLinks++
		}
	}
	if billLinks != 1 {
		t.Fatalf("want exactly 1 bill-match link, got %d", billLinks)
	}
}
