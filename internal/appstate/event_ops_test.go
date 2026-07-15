// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func evDay(d int) time.Time { return time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC) }

func TestEventCRUDAndAutoAssociate(t *testing.T) {
	a := newApp(t, false)

	// Seed transactions in and out of range.
	for _, tx := range []domain.Transaction{
		{ID: "t1", AccountID: "a1", Desc: "Dinner", Date: evDay(2), Amount: money.New(-1000, "USD"), CategoryID: "food"},
		{ID: "t2", AccountID: "a1", Desc: "Hotel", Date: evDay(5), Amount: money.New(-2000, "USD"), CategoryID: "lodging"},
		{ID: "t3", AccountID: "a1", Desc: "Later", Date: evDay(20), Amount: money.New(-3000, "USD"), CategoryID: "food"}, // out of range
	} {
		if err := a.PutTransaction(tx); err != nil {
			t.Fatalf("seed txn: %v", err)
		}
	}

	// Name is required.
	if _, err := a.PutEvent(domain.Event{Start: evDay(1)}); err == nil {
		t.Fatalf("expected error for empty name")
	}

	ev, err := a.PutEvent(domain.Event{Name: "Trip", Start: evDay(1), End: evDay(10)})
	if err != nil {
		t.Fatalf("PutEvent: %v", err)
	}
	if ev.ID == "" {
		t.Fatalf("expected assigned id")
	}

	n, err := a.AutoAssociateEvent(ev.ID)
	if err != nil {
		t.Fatalf("AutoAssociate: %v", err)
	}
	if n != 2 {
		t.Fatalf("tagged %d want 2", n)
	}
	m := a.EventMembers(ev.ID)
	if !m["t1"] || !m["t2"] || m["t3"] {
		t.Fatalf("members=%v", m)
	}

	// Idempotent re-run creates nothing.
	if n2, _ := a.AutoAssociateEvent(ev.ID); n2 != 0 {
		t.Fatalf("re-run tagged %d want 0", n2)
	}

	// Unmap one.
	if err := a.UnmapTxnFromEvent("t1", ev.ID); err != nil {
		t.Fatalf("Unmap: %v", err)
	}
	if a.EventMembers(ev.ID)["t1"] {
		t.Fatalf("t1 still mapped")
	}

	// Delete event cascades link removal, keeps txns.
	if err := a.DeleteEvent(ev.ID); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if len(a.Events()) != 0 {
		t.Fatalf("event not deleted")
	}
	for _, l := range a.TxnLinks() {
		if l.Kind == domain.TxnLinkEventTxn {
			t.Fatalf("event link leaked: %+v", l)
		}
	}
	if txns := a.Transactions(); len(txns) != 3 {
		t.Fatalf("transactions should survive, got %d", len(txns))
	}
}
