// SPDX-License-Identifier: MIT

package appstate

import (
	"io"
	"testing"
)

func TestTransactionDurabilityIsPendingUntilCommitAndScopeCancelable(t *testing.T) {
	a, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Store().Close()
	var complete func(DurableResult)
	var revision uint64
	a.SetDurableWriter(func(r uint64, done func(DurableResult)) { revision = r; complete = done })
	var events []TransactionDurability
	a.OnTransactionDurability(func(event TransactionDurability) { event.TransactionIDs[0] = "observer-owned copy" })
	cancel := a.OnTransactionDurability(func(event TransactionDurability) { events = append(events, event) })
	ids := []string{"row"}
	done := 0
	a.RequestTransactionsDurable(ids, func(DurableResult) { done++ })
	ids[0] = "caller-owned copy"
	if len(events) != 1 || !events[0].Pending || events[0].TransactionIDs[0] != "row" || done != 0 {
		t.Fatal("pending event acknowledged a write or aliased IDs")
	}
	complete(DurableResult{Revision: revision})
	if len(events) != 2 || events[1].Pending || events[1].Operation != events[0].Operation || events[1].Result.Err != nil || done != 1 {
		t.Fatal("durable completion event missing")
	}
	cancel()
	a.RequestTransactionsDurable([]string{"row"}, nil)
	complete(DurableResult{Revision: revision})
	if len(events) != 2 {
		t.Fatal("cancelled UI scope kept observing another operation")
	}
}
