// SPDX-License-Identifier: MIT

package undosnap_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

// normalize unmarshals JSON into an interface{} so byte-level differences
// (key order, whitespace) don't cause spurious failures.
func normalize(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return v
}

func TestToSnapshot_Arrays(t *testing.T) {
	input := []byte(`{
		"accounts": [
			{"id":"a1","name":"Checking","balance":1000},
			{"id":"a2","name":"Savings","balance":5000}
		],
		"transactions": [
			{"id":"t1","amount":-50,"account":"a1"}
		]
	}`)

	snap, err := undosnap.ToSnapshot(input)
	if err != nil {
		t.Fatalf("ToSnapshot: %v", err)
	}
	if len(snap["accounts"]) != 2 {
		t.Errorf("accounts: want 2 rows, got %d", len(snap["accounts"]))
	}
	if _, ok := snap["accounts"]["a1"]; !ok {
		t.Error("accounts[a1] missing")
	}
	if _, ok := snap["accounts"]["a2"]; !ok {
		t.Error("accounts[a2] missing")
	}
	if len(snap["transactions"]) != 1 {
		t.Errorf("transactions: want 1 row, got %d", len(snap["transactions"]))
	}
}

func TestToSnapshot_Scalars(t *testing.T) {
	input := []byte(`{
		"schemaVersion": 3,
		"settings": {"currency":"USD","openAIKey":""},
		"accounts": []
	}`)

	snap, err := undosnap.ToSnapshot(input)
	if err != nil {
		t.Fatalf("ToSnapshot: %v", err)
	}
	// Scalars land in _meta:* collections.
	if _, ok := snap["_meta:schemaVersion"]["schemaVersion"]; !ok {
		t.Error("_meta:schemaVersion missing")
	}
	if _, ok := snap["_meta:settings"]["settings"]; !ok {
		t.Error("_meta:settings missing")
	}
	// Empty array produces an empty map (not nil) collection.
	if snap["accounts"] == nil {
		t.Error("accounts collection should be non-nil (empty map)")
	}
	if len(snap["accounts"]) != 0 {
		t.Errorf("accounts: want 0 rows, got %d", len(snap["accounts"]))
	}
}

func TestToSnapshot_ReservedKeyError(t *testing.T) {
	input := []byte(`{"_meta:evil": "bad"}`)
	_, err := undosnap.ToSnapshot(input)
	if err == nil {
		t.Fatal("expected error for reserved _meta: key in export JSON")
	}
}

func TestToSnapshot_MissingIDError(t *testing.T) {
	input := []byte(`{"accounts": [{"name":"no id here"}]}`)
	_, err := undosnap.ToSnapshot(input)
	if err == nil {
		t.Fatal("expected error for element missing id")
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{
			name: "typical export",
			input: `{
				"schemaVersion": 2,
				"settings": {"currency":"USD","theme":"dark"},
				"accounts": [
					{"id":"acc1","name":"Checking","type":"checking"},
					{"id":"acc2","name":"Savings","type":"savings"}
				],
				"transactions": [
					{"id":"tx1","amount":-120,"account":"acc1","date":"2026-01-15"},
					{"id":"tx2","amount":5000,"account":"acc2","date":"2026-01-01"}
				],
				"categories": []
			}`,
		},
		{
			name:  "empty collections",
			input: `{"accounts":[],"transactions":[],"categories":[]}`,
		},
		{
			name:  "only scalars",
			input: `{"schemaVersion":1,"settings":{"currency":"EUR"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap, err := undosnap.ToSnapshot([]byte(tc.input))
			if err != nil {
				t.Fatalf("ToSnapshot: %v", err)
			}
			out, err := undosnap.ToJSON(snap)
			if err != nil {
				t.Fatalf("ToJSON: %v", err)
			}
			if !reflect.DeepEqual(normalize(t, []byte(tc.input)), normalize(t, out)) {
				t.Errorf("round-trip mismatch\ninput: %s\noutput: %s", tc.input, out)
			}
		})
	}
}

func TestDiffAndApplyIntegration(t *testing.T) {
	// Confirm that Diff / Invert / Apply work correctly when combined with
	// undosnap: forward diff moves forward, inverted diff moves back.
	before := []byte(`{"accounts":[{"id":"a1","name":"Checking","balance":1000}]}`)
	after := []byte(`{"accounts":[{"id":"a1","name":"Checking","balance":1500}]}`)

	snapBefore, err := undosnap.ToSnapshot(before)
	if err != nil {
		t.Fatal(err)
	}
	snapAfter, err := undosnap.ToSnapshot(after)
	if err != nil {
		t.Fatal(err)
	}

	cs := history.Diff(snapBefore, snapAfter)
	if cs.IsEmpty() {
		t.Fatal("Diff should detect the balance change")
	}

	// Apply forward: snapBefore + cs == snapAfter
	applied := cs.Apply(snapBefore)
	gotAfterJSON, err := undosnap.ToJSON(applied)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(t, gotAfterJSON), normalize(t, after)) {
		t.Errorf("forward apply wrong: got %s", gotAfterJSON)
	}

	// Apply inverse: snapAfter + cs.Invert() == snapBefore
	reverted := cs.Invert().Apply(snapAfter)
	gotBeforeJSON, err := undosnap.ToJSON(reverted)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(t, gotBeforeJSON), normalize(t, before)) {
		t.Errorf("inverse apply wrong: got %s", gotBeforeJSON)
	}
}

func TestStackUndoRedo(t *testing.T) {
	// Confirm Stack.Undo returns the inverse change set and Stack.Redo returns
	// the forward change set, so our undo controller uses them correctly.
	snap0 := history.Snapshot{
		"items": {"i1": json.RawMessage(`{"id":"i1","v":1}`)},
	}
	snap1 := history.Snapshot{
		"items": {"i1": json.RawMessage(`{"id":"i1","v":2}`)},
	}

	cs := history.Diff(snap0, snap1)
	if cs.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}

	stack := history.NewStack(0)
	stack.Push(cs)

	// Undo returns the INVERSE change set (moves backward).
	undoCS, ok := stack.Undo()
	if !ok {
		t.Fatal("Undo should succeed")
	}
	revertedSnap := undoCS.Apply(snap1)
	if string(revertedSnap["items"]["i1"]) != `{"id":"i1","v":1}` {
		t.Errorf("undo did not revert: got %s", revertedSnap["items"]["i1"])
	}

	// Redo returns the FORWARD change set (re-applies the mutation).
	redoCS, ok := stack.Redo()
	if !ok {
		t.Fatal("Redo should succeed")
	}
	reappliedSnap := redoCS.Apply(snap0)
	if string(reappliedSnap["items"]["i1"]) != `{"id":"i1","v":2}` {
		t.Errorf("redo did not re-apply: got %s", reappliedSnap["items"]["i1"])
	}
}
