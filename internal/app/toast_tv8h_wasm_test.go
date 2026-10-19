// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTV8HToastRequiresExplicitUndoAndPersistencePolicy(t *testing.T) {
	old := auditview.CanUndoFunc
	auditview.CanUndoFunc = func() bool { return true }
	defer func() { auditview.CanUndoFunc = old }()
	for _, tc := range []struct {
		n    uistate.Notice
		undo bool
	}{
		{uistate.Notice{}.With("Deleted updated removed archived", false), false},
		{uistate.Notice{}.WithUndoable("Filed under Dining"), true},
		{uistate.Notice{Text: "Failure", Err: true, Undoable: true}, false},
		{uistate.Notice{Text: "Information", NonDataset: true, Undoable: true}, false},
	} {
		if got := toastOffersUndo(tc.n); got != tc.undo {
			t.Fatalf("Undo=%v for %+v", got, tc.n)
		}
	}
	auditview.CanUndoFunc = func() bool { return false }
	if toastOffersUndo(uistate.Notice{}.WithUndoable("Filed")) {
		t.Fatal("offered unavailable inverse")
	}
	a, _ := bv6dJournal(t, false)
	oldApp := durableNoticeApp
	durableNoticeApp = a
	defer func() { durableNoticeApp = oldApp }()
	if !toastNeedsDurable(uistate.Notice{}.With("Saved", false), a) {
		t.Fatal("uncommitted success bypassed durability")
	}
	for _, n := range []uistate.Notice{uistate.Notice{}.WithTransient("Information"), uistate.Notice{}.WithDurable("Saved", true), uistate.Notice{}.With("Failed", true)} {
		if toastNeedsDurable(n, a) {
			t.Fatalf("unnecessary durable wait for %+v", n)
		}
	}
	if toastTimeoutMS != 8000 || toastErrTimeoutMS < 8000 {
		t.Fatal("toast lasts less than eight seconds")
	}
}

func TestTV8HToastUndoFailureIsRetryableWithoutPoppingAgain(t *testing.T) {
	for _, recoveredBeforeClick := range []bool{false, true} {
		name := "retry commits"
		if recoveredBeforeClick {
			name = "autosave commits before retry"
		}
		t.Run(name, func(t *testing.T) {
			a, commits := bv6dJournal(t, false)
			defer uistate.ClearPersistFailure("transaction-undo")
			f := render.New(t)
			defer f.Cleanup()
			var notice state.Atom[uistate.Notice]
			harness := func() ui.Node { notice = uistate.UseNotice(); return Div() }
			f.Render(ui.CreateElement(harness))
			notice.Set(uistate.Notice{})
			before := a.Transactions()[0]
			after := before
			after.Note += " TV8H"
			if err := screens.CommitUndoable(a, "Transaction updated", []string{after.ID}, func() error { return a.PutTransaction(after) }, nil); err != nil {
				t.Fatal(err)
			}
			bv6dFinish(t, waitSOLP2Commit(t, commits))
			if !notice.Get().Undoable {
				t.Fatal("durable write has no Undo")
			}
			completed := make(chan error, 2)
			commitToastUndo(notice, func(err error) { completed <- err })
			// Undo schedules notification catch-up independently of the save. Keep this
			// fixture focused on write feedback rather than sample-household reminders.
			notifySchedule.epoch++
			if notifySchedule.timer != nil {
				notifySchedule.timer.Stop()
			}
			commit := waitSOLP2Commit(t, commits)
			if notice.Get().Text == "Change undone." {
				t.Fatal("undo success before commit")
			}
			f.Rerender(ui.CreateElement(harness))
			commit.done(errors.New("quota exhausted"))
			if err := <-completed; err == nil {
				t.Fatal("failed Undo completed successfully")
			}
			if n := notice.Get(); !n.Err || n.Undoable {
				t.Fatalf("failed Undo = %+v", n)
			}
			if !undoPersistencePending {
				t.Fatal("lost pending restoration")
			}
			if recoveredBeforeClick {
				autosaved := make(chan error, 1)
				a.RequestDurable(func(result appstate.DurableResult) { autosaved <- result.Err })
				bv6dFinish(t, waitSOLP2Commit(t, commits))
				if err := <-autosaved; err != nil {
					t.Fatal(err)
				}
			}
			cursor, err := a.Store().ChangeCursor()
			if err != nil {
				t.Fatal(err)
			}
			f.Rerender(ui.CreateElement(harness))
			uistate.RetryPersistFailures()
			if !recoveredBeforeClick {
				bv6dFinish(t, waitSOLP2Commit(t, commits))
			}
			afterRetry, err := a.Store().ChangeCursor()
			if err != nil || afterRetry != cursor {
				t.Fatalf("retry replayed history: %d -> %d (%v)", cursor, afterRetry, err)
			}
			if err := <-completed; err != nil {
				t.Fatal(err)
			}
			if undoPersistencePending {
				t.Fatal("successful retry stayed pending")
			}
			got, _ := a.TransactionByID(before.ID)
			if got.Note != before.Note {
				t.Fatalf("undo did not restore transaction: %q", got.Note)
			}
			if n := notice.Get(); n.Err || n.Undoable || !n.Durable || n.Text != "Change undone." {
				t.Fatalf("undo recovery = %+v", n)
			}

		})
	}
}

func TestTV8HToastKeepsEightSecondsOfVisibleReadingTime(t *testing.T) {
	clock := toastReadingClock{}
	for _, step := range []struct {
		now                       float64
		visibility                string
		hovered, focused, expired bool
	}{
		{now: 3000, visibility: "visible"},
		{now: 15000, visibility: "visible", hovered: true},
		{now: 27000, visibility: "visible", focused: true},
		{now: 47000, visibility: "hidden"},
		{now: 51999, visibility: "visible"},
		{now: 52000, visibility: "visible", expired: true},
	} {
		reading := toastV5RowCountsTime(step.visibility, step.hovered, step.focused)
		if got := clock.expired(step.now, 8000, reading); got != step.expired {
			t.Fatalf("at %vms expired=%v elapsed=%v", step.now, got, clock.elapsed)
		}
	}
	// A visibility event resets the timestamp even if background intervals were
	// suspended for a minute; returning gives the user their remaining time.
	clock = toastReadingClock{last: 0, elapsed: 3000}
	clock.last = 60000
	if clock.expired(60100, 8000, true) || clock.elapsed != 3100 {
		t.Fatalf("hidden time charged: %+v", clock)
	}
}
