// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/changeset"
)

func TestUndoChangesetReceiptUsesExactReverseOrderAndWaits(t *testing.T) {
	r := changeset.Receipt{
		Applied: []changeset.AppliedOp{{UndoID: "one"}, {UndoID: "two"}},
		Failed:  &changeset.FailedOp{UndoID: "partial"},
	}
	stack := []string{"partial", "two", "one"}
	var started []string
	var pending func(error)
	finished := false
	undoChangesetReceipt(&r,
		func() string {
			if len(stack) == 0 {
				return ""
			}
			return stack[0]
		},
		func(id string, done func(error)) bool {
			started = append(started, id)
			pending = done
			return true
		},
		func() {},
		func(err error) {
			if err != nil {
				t.Errorf("undo failed: %v", err)
			}
			finished = true
		},
	)
	if !reflect.DeepEqual(started, []string{"partial"}) {
		t.Fatalf("started before durable completion = %v", started)
	}
	for _, want := range []string{"partial", "two", "one"} {
		if pending == nil || started[len(started)-1] != want {
			t.Fatalf("pending undo order = %v, want current %q", started, want)
		}
		stack = stack[1:]
		complete := pending
		pending = nil
		complete(nil)
	}
	if !finished || len(r.UndoRemainingIDs()) != 0 {
		t.Fatalf("finished=%v remaining=%v", finished, r.UndoRemainingIDs())
	}
	if !reflect.DeepEqual(started, []string{"partial", "two", "one"}) {
		t.Fatalf("undo sequence = %v", started)
	}
}

func TestUndoChangesetReceiptStopsOnMismatchAndError(t *testing.T) {
	t.Run("mismatch", func(t *testing.T) {
		r := changeset.Receipt{Applied: []changeset.AppliedOp{{UndoID: "expected"}}}
		called := false
		var gotErr error
		undoChangesetReceipt(&r, func() string { return "other" }, func(string, func(error)) bool {
			called = true
			return true
		}, func() {}, func(err error) { gotErr = err })
		if called || gotErr == nil || len(r.UndoneIDs) != 0 {
			t.Fatalf("mismatch called=%v err=%v undone=%v", called, gotErr, r.UndoneIDs)
		}
	})

	t.Run("new top after partial progress is preserved", func(t *testing.T) {
		r := changeset.Receipt{Applied: []changeset.AppliedOp{{UndoID: "one"}, {UndoID: "two"}}}
		top := "two"
		var pending func(error)
		started := []string{}
		var gotErr error
		undoChangesetReceipt(&r, func() string { return top }, func(id string, done func(error)) bool {
			started = append(started, id)
			pending = done
			return true
		}, func() {}, func(err error) { gotErr = err })
		completeFirst := pending
		pending = nil
		top = "manual-change"
		completeFirst(nil)
		if len(started) != 1 || started[0] != "two" || gotErr == nil {
			t.Fatalf("started=%v err=%v, expected to stop before unrelated top", started, gotErr)
		}
		if got := r.UndoRemainingIDs(); len(got) != 1 || got[0] != "one" {
			t.Fatalf("remaining IDs = %v, want [one]", got)
		}
	})

	t.Run("durable error preserves partial progress", func(t *testing.T) {
		r := changeset.Receipt{Applied: []changeset.AppliedOp{{UndoID: "one"}, {UndoID: "two"}}}
		stack := []string{"two", "one"}
		var pending func(error)
		started := []string{}
		var gotErr error
		undoChangesetReceipt(&r, func() string { return stack[0] }, func(id string, done func(error)) bool {
			started = append(started, id)
			pending = done
			return true
		}, func() {}, func(err error) { gotErr = err })
		stack = stack[1:]
		first := pending
		pending = nil
		first(nil)
		second := pending
		if second == nil || !reflect.DeepEqual(started, []string{"two", "one"}) {
			t.Fatalf("second undo not started after first completion: %v", started)
		}
		failure := errors.New("storage rejected undo")
		second(failure)
		if !errors.Is(gotErr, failure) || !reflect.DeepEqual(r.UndoRemainingIDs(), []string{"one"}) {
			t.Fatalf("error=%v remaining=%v", gotErr, r.UndoRemainingIDs())
		}
	})
}
