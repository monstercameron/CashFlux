// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"io"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/store"
)

func assistantUndoFixture(t *testing.T) *appstate.App {
	t.Helper()
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	data := store.SampleDataset()
	if err := a.Store().Load(data); err != nil {
		t.Fatal(err)
	}
	oldDefault := appstate.Default
	oldStack, oldSnap := undoStack, lastUndoSnap
	oldApp, oldRev, oldCursor, oldPending := lastUndoCaptureApp, lastUndoCaptureRev, undoCaptureCursor, undoPending
	oldApplying, oldPersistence, oldDurableCursor := applyingUndo, undoPersistencePending, undoDurableCursor
	oldFeed := auditview.Feed
	appstate.Default = a
	undoStack = history.NewStack(4 * 1024 * 1024)
	undoPending = nil
	applyingUndo, undoPersistencePending, undoDurableCursor = false, false, 0
	auditview.Feed = auditlog.New(500)
	initUndo()
	t.Cleanup(func() {
		appstate.Default = oldDefault
		undoStack, lastUndoSnap = oldStack, oldSnap
		lastUndoCaptureApp, lastUndoCaptureRev, undoCaptureCursor, undoPending = oldApp, oldRev, oldCursor, oldPending
		applyingUndo, undoPersistencePending, undoDurableCursor = oldApplying, oldPersistence, oldDurableCursor
		auditview.Feed = oldFeed
		_ = a.Store().Close()
	})
	return a
}

func TestAssistantCaptureSeparatesManualBaselineAndSkipsNoop(t *testing.T) {
	a := assistantUndoFixture(t)
	manual := domain.Task{ID: "manual-before-agent", Title: "Manual task", Status: domain.StatusOpen, Priority: domain.PriorityMedium}
	if err := a.PutTask(manual); err != nil {
		t.Fatal(err)
	}
	captureAssistantBaseline()
	if len(undoPending) != 1 || undoPending[0].change.UndoID == "" {
		t.Fatalf("manual baseline pending=%d, id=%q; want one identified point", len(undoPending), pendingID(0))
	}
	manualID := undoPending[0].change.UndoID
	auditview.SetSessionActor(auditview.ActorAssistant)
	assistant := domain.Task{ID: "assistant-task", Title: "Assistant task", Status: domain.StatusOpen, Priority: domain.PriorityMedium}
	if err := a.PutTask(assistant); err != nil {
		t.Fatal(err)
	}
	agentID, changed := captureAssistantReceipt()
	auditview.SetSessionActor("")
	if !changed || agentID == "" || agentID == manualID {
		t.Fatalf("assistant receipt = (%q, %v), manual ID %q", agentID, changed, manualID)
	}
	if len(undoPending) != 2 || undoPending[1].change.UndoID != agentID {
		t.Fatalf("pending points=%d, assistant point ID=%q", len(undoPending), pendingID(1))
	}
	actors := map[string]string{}
	for _, entry := range auditview.Feed.Recent(10) {
		actors[entry.ID] = entry.Actor
	}
	if actors[manualID] != "user" || actors[agentID] != auditview.ActorAssistant {
		t.Fatalf("captured actors = manual:%q assistant:%q", actors[manualID], actors[agentID])
	}
	if id, changed := captureAssistantReceipt(); changed || id != "" {
		t.Fatalf("no-op capture = (%q, %v), want empty receipt", id, changed)
	}
	if len(undoPending) != 2 {
		t.Fatalf("no-op capture added a pending point: %d", len(undoPending))
	}
}

func TestUndoExpectedRequiresPublishedTopID(t *testing.T) {
	a := assistantUndoFixture(t)
	task := domain.Task{ID: "assistant-task", Title: "Assistant task", Status: domain.StatusOpen, Priority: domain.PriorityMedium}
	if err := a.PutTask(task); err != nil {
		t.Fatal(err)
	}
	auditview.SetSessionActor(auditview.ActorAssistant)
	id, changed := captureAssistantReceipt()
	auditview.SetSessionActor("")
	if !changed || id == "" {
		t.Fatal("assistant mutation did not create a receipt")
	}
	through := undoPending[0].through
	publishDurableUndo(through)
	if undoStack.UndoID() != id {
		t.Fatalf("published top ID=%q, want %q", undoStack.UndoID(), id)
	}

	manual := domain.Task{ID: "manual-after-agent", Title: "Manual task", Status: domain.StatusOpen, Priority: domain.PriorityMedium}
	if err := a.PutTask(manual); err != nil {
		t.Fatal(err)
	}
	captureAssistantBaseline()
	manualID := undoPending[len(undoPending)-1].change.UndoID
	publishDurableUndo(undoPending[len(undoPending)-1].through)
	if manualID == "" || undoStack.UndoID() != manualID {
		t.Fatalf("manual top ID=%q, want %q", undoStack.UndoID(), manualID)
	}
	called := false
	accepted := undoExpected(id, func(err error) {
		called = true
		if err == nil {
			t.Error("undo of a no-longer-top Assistant action unexpectedly succeeded")
		}
	})
	if !accepted || !called {
		t.Fatalf("mismatched undo accepted=%v callback=%v", accepted, called)
	}
	if undoStack.UndoID() != manualID {
		t.Fatalf("mismatch moved the undo cursor; top ID=%q, want %q", undoStack.UndoID(), manualID)
	}

	// Remove the intervening manual action through the ordinary stack path, then
	// verify the exact Assistant receipt is accepted and durably replayed.
	a.SetDurableWriter(func(revision uint64, done func(appstate.DurableResult)) {
		cursor, _ := a.Store().ChangeCursor()
		publishDurableUndo(cursor)
		done(appstate.DurableResult{Revision: revision})
	})
	completed := make(chan error, 1)
	undoLastChangeThen(false, func(err error) { completed <- err })
	if err := <-completed; err != nil {
		t.Fatalf("undo intervening manual point: %v", err)
	}
	if undoStack.UndoID() != id {
		t.Fatalf("top after manual undo=%q, want Assistant ID %q", undoStack.UndoID(), id)
	}
	completed = make(chan error, 1)
	if !undoExpected(id, func(err error) { completed <- err }) {
		t.Fatal("expected Assistant undo was not accepted")
	}
	if err := <-completed; err != nil {
		t.Fatalf("expected Assistant undo: %v", err)
	}
	if _, ok, err := a.Store().GetTask(task.ID); err != nil || ok {
		t.Fatalf("Assistant task remains after undo: exists=%v err=%v", ok, err)
	}
}

func pendingID(index int) string {
	if index < 0 || index >= len(undoPending) {
		return ""
	}
	return undoPending[index].change.UndoID
}
