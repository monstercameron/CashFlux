// SPDX-License-Identifier: MIT

package app

import "testing"

// The case C584 was filed for, and the one C598's journey reloads into: one edit,
// then an immediate refresh. The write has to be issued on the FIRST tick that
// sees the change, because the second tick may never arrive before the reload.
func TestRevisionWatcherWritesOnTheFirstTickAfterAnEdit(t *testing.T) {
	var w revisionWatcher

	if w.tick(0) {
		t.Error("wrote with nothing changed")
	}
	if !w.tick(1) {
		t.Fatal("an edit did not write on the tick that noticed it")
	}
	if !w.tick(1) {
		t.Error("the trailing write after the burst ended did not happen")
	}
	if w.tick(1) {
		t.Error("kept writing after the dataset went quiet")
	}
}

// A burst costs at most one write per tick — not one per edit, and not one per
// tick forever after.
func TestRevisionWatcherCoalescesABurst(t *testing.T) {
	var w revisionWatcher
	writes := 0
	// Six edits arriving across three ticks (the revision jumps by more than one
	// between observations), then silence.
	for _, rev := range []int{2, 4, 6, 6, 6, 6} {
		if w.tick(rev) {
			writes++
		}
	}
	// Three leading writes (one per tick that moved) plus one trailing write.
	if writes != 4 {
		t.Errorf("a six-edit burst cost %d writes, want 4", writes)
	}
}

// Quiet is quiet: an idle app must not write at all.
func TestRevisionWatcherIsSilentWhenNothingChanges(t *testing.T) {
	w := revisionWatcher{seen: 7}
	for i := range 50 {
		if w.tick(7) {
			t.Fatalf("wrote on idle tick %d", i)
		}
	}
}

// The revision is a counter the UI bumps; a watcher started mid-session seeds
// from wherever it is and must not mistake that for a change.
func TestRevisionWatcherSeedsWithoutWriting(t *testing.T) {
	w := revisionWatcher{seen: 42}
	if w.tick(42) {
		t.Error("wrote immediately at startup with no edit behind it")
	}
	if !w.tick(43) {
		t.Error("missed the first real edit after startup")
	}
}
