// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestRunAssistantWriteResultRecordsOnlyCapturedChanges(t *testing.T) {
	oldBaseline, oldCapture := auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt
	oldActor := auditview.SessionActor()
	t.Cleanup(func() {
		auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt = oldBaseline, oldCapture
		auditview.SetSessionActor(oldActor)
	})
	baselines := 0
	auditview.CaptureAssistantBaseline = func() bool { baselines++; return true }
	auditview.CaptureAssistantReceipt = func() (string, bool) { return "", false }
	actionCount := len(uistate.AssistantActions())

	noop := runAssistantWriteResult("noop_tool", func() string { return "Nothing changed." })
	if noop.Text != "Nothing changed." || noop.Changed || noop.UndoID != "" {
		t.Fatalf("no-op result = %+v", noop)
	}
	if len(uistate.AssistantActions()) != actionCount {
		t.Fatal("no-op created an Assistant action history item")
	}

	auditview.CaptureAssistantReceipt = func() (string, bool) { return "receipt-42", true }
	changed := runAssistantWriteResult("change_tool", func() string { return "Saved." })
	if changed.Text != "Saved." || !changed.Changed || changed.UndoID != "receipt-42" {
		t.Fatalf("changed result = %+v", changed)
	}
	last, ok := uistate.LastAssistantAction()
	if !ok || last.Tool != "change_tool" || last.UndoID != changed.UndoID {
		t.Fatalf("last Assistant action = %+v/%v", last, ok)
	}
	uistate.DropLastAssistantActionFor(changed.UndoID)
	if baselines != 2 {
		t.Fatalf("baseline captures=%d, want one per call", baselines)
	}
}

func TestRunAssistantWriteDoesNotRunWithoutAnIsolatedBaseline(t *testing.T) {
	oldBaseline, oldCapture := auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt
	oldActor := auditview.SessionActor()
	t.Cleanup(func() {
		auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt = oldBaseline, oldCapture
		auditview.SetSessionActor(oldActor)
	})
	auditview.SetSessionActor("prior-actor")
	auditview.CaptureAssistantBaseline = func() bool { return false }
	captured := false
	auditview.CaptureAssistantReceipt = func() (string, bool) {
		captured = true
		return "should-not-exist", true
	}
	ran := false
	result := runAssistantWriteResult("blocked_tool", func() string {
		ran = true
		return "changed"
	})
	if ran || captured || result.Changed || result.UndoID != "" {
		t.Fatalf("unisolated write ran or captured: ran=%v captured=%v result=%+v", ran, captured, result)
	}
	if auditview.SessionActor() != "prior-actor" {
		t.Fatalf("actor after refusal = %q", auditview.SessionActor())
	}
}

func TestRunAssistantWriteResultCapturesPartialPanicAndRestoresActor(t *testing.T) {
	oldBaseline, oldCapture := auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt
	oldActor := auditview.SessionActor()
	auditview.SetSessionActor("prior-actor")
	t.Cleanup(func() {
		auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt = oldBaseline, oldCapture
		auditview.SetSessionActor(oldActor)
	})
	auditview.CaptureAssistantBaseline = func() bool { return true }
	captured := false
	auditview.CaptureAssistantReceipt = func() (string, bool) {
		captured = auditview.SessionActor() == auditview.ActorAssistant
		return "partial-receipt", true
	}
	result := runAssistantWriteResult("panic_tool", func() string { panic("after mutation") })
	if !strings.Contains(result.Text, "panic_tool tool failed: after mutation") || !result.Changed || result.UndoID != "partial-receipt" {
		t.Fatalf("panic result = %+v", result)
	}
	if !captured || auditview.SessionActor() != "prior-actor" {
		t.Fatalf("captureActor=%v restoredActor=%q", captured, auditview.SessionActor())
	}
	uistate.DropLastAssistantActionFor(result.UndoID)
}
