// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "testing"

func TestTV8HNoticeFlagsAreExplicitAndDoNotLeak(t *testing.T) {
	n := Notice{Seq: 4}
	for _, tc := range []struct {
		name                     string
		notice                   Notice
		undo, durable, transient bool
	}{
		{"write", n.With("Updated", false), false, false, false},
		{"undoable write", n.WithUndoable("Filed under Dining"), true, false, false},
		{"committed write", n.WithDurable("Filed under Dining", true), true, true, false},
		{"undo", n.WithDurable("Change undone.", false), false, true, false},
		{"information", n.WithTransient("Nothing to undo"), false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.notice.MarkLeaving()
			if got.Seq != 5 || !got.Leaving || got.Undoable != tc.undo || got.Durable != tc.durable || got.NonDataset != tc.transient {
				t.Fatalf("flags lost: %+v", got)
			}
			next := got.With("Failed", true)
			if next.Seq != 6 || !next.Err || next.Undoable || next.Durable || next.NonDataset || next.Leaving {
				t.Fatalf("flags leaked: %+v", next)
			}
			clear := got.Cleared()
			if clear != (Notice{Seq: 5}) {
				t.Fatalf("clear retained flags: %+v", clear)
			}
		})
	}
}
