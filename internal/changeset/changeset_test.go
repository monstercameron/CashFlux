// SPDX-License-Identifier: MIT

package changeset

import (
	"encoding/json"
	"testing"
)

func TestAddAndEnabledCount(t *testing.T) {
	c := New("Set up a vacation fund")
	c.Add("create_goal", "Create goal \"Vacation\"", json.RawMessage(`{"name":"Vacation"}`))
	c.Add("create_budget", "Create budget \"Travel\"", nil)
	c.Add("create_rule", "Route Travel spend to the budget", nil)

	if c.Len() != 3 {
		t.Fatalf("Len = %d, want 3", c.Len())
	}
	if c.IsEmpty() {
		t.Fatal("IsEmpty = true, want false")
	}
	if got := c.EnabledCount(); got != 3 {
		t.Fatalf("EnabledCount = %d, want 3 (all enabled by default)", got)
	}
	c.SetEnabled(1, false)
	if got := c.EnabledCount(); got != 2 {
		t.Fatalf("EnabledCount after disable = %d, want 2", got)
	}
	// Out-of-range toggles are ignored, not panics.
	c.SetEnabled(99, false)
	c.SetEnabled(-1, true)
	if got := c.EnabledCount(); got != 2 {
		t.Fatalf("EnabledCount after no-op toggles = %d, want 2", got)
	}
}

func TestReceiptHelpers(t *testing.T) {
	ok := Receipt{Applied: []AppliedOp{
		{Index: 0, Kind: "add_transaction", Result: "Recorded $4.50", UndoID: "a"},
		{Index: 1, Kind: "add_transaction", Result: "Recorded $38.00", UndoID: "b"},
		{Index: 2, Kind: "create_category", Result: "Created Groceries", UndoID: "c"},
	}}
	if !ok.OK() {
		t.Fatal("OK = false, want true when Failed is nil")
	}
	if ok.AppliedCount() != 3 {
		t.Fatalf("AppliedCount = %d, want 3", ok.AppliedCount())
	}
	kinds := ok.Kinds()
	want := []string{"add_transaction", "add_transaction", "create_category"}
	if len(kinds) != len(want) {
		t.Fatalf("Kinds len = %d, want %d", len(kinds), len(want))
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("Kinds[%d] = %q, want %q", i, kinds[i], want[i])
		}
	}

	failed := Receipt{
		Applied: []AppliedOp{{Index: 0, Kind: "add_task"}},
		Failed:  &FailedOp{Index: 1, Kind: "add_account", Err: "boom"},
	}
	if failed.OK() {
		t.Fatal("OK = true, want false when Failed is set")
	}
	if failed.AppliedCount() != 1 {
		t.Fatalf("AppliedCount = %d, want 1 (partial before failure)", failed.AppliedCount())
	}
}

func TestUndoOrderIncludesPartialFailureAndTracksPartialProgress(t *testing.T) {
	r := Receipt{
		Applied: []AppliedOp{{UndoID: "first"}, {UndoID: "second"}},
		Failed:  &FailedOp{UndoID: "partial"},
	}
	want := []string{"partial", "second", "first"}
	if got := r.UndoOrderIDs(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("UndoOrderIDs = %v, want %v", got, want)
	}
	r.MarkUndone("partial")
	r.MarkUndone("second")
	r.MarkUndone("second")
	wantRemaining := []string{"first"}
	if got := r.UndoRemainingIDs(); len(got) != len(wantRemaining) || got[0] != wantRemaining[0] {
		t.Fatalf("UndoRemainingIDs = %v, want %v", got, wantRemaining)
	}
}

func TestZeroValueUsable(t *testing.T) {
	var c Changeset
	if !c.IsEmpty() {
		t.Fatal("zero Changeset should be empty")
	}
	if c.EnabledCount() != 0 {
		t.Fatal("zero Changeset EnabledCount should be 0")
	}
}
