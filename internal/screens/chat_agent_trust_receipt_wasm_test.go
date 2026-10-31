// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestRunAssistantWriteResultRecordsOnlyCapturedChanges(t *testing.T) {
	oldBaseline, oldCapture := auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked
	oldActor := auditview.SessionActor()
	t.Cleanup(func() {
		auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked = oldBaseline, oldCapture
		auditview.SetSessionActor(oldActor)
	})
	baselines := 0
	auditview.CaptureAssistantBaselineChecked = func() error { baselines++; return nil }
	auditview.CaptureAssistantReceiptChecked = func() (string, bool, error) { return "", false, nil }
	actionCount := len(uistate.AssistantActions())

	noop := runAssistantWriteResult("noop_tool", func() string { return "Nothing changed." })
	if noop.Text != "Nothing changed." || noop.Changed || noop.UndoID != "" {
		t.Fatalf("no-op result = %+v", noop)
	}
	if len(uistate.AssistantActions()) != actionCount {
		t.Fatal("no-op created an Assistant action history item")
	}

	auditview.CaptureAssistantReceiptChecked = func() (string, bool, error) { return "receipt-42", true, nil }
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
	oldBaseline, oldCapture := auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked
	oldActor := auditview.SessionActor()
	t.Cleanup(func() {
		auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked = oldBaseline, oldCapture
		auditview.SetSessionActor(oldActor)
	})
	auditview.SetSessionActor("prior-actor")
	auditview.CaptureAssistantBaselineChecked = func() error { return errors.New("baseline unavailable") }
	captured := false
	auditview.CaptureAssistantReceiptChecked = func() (string, bool, error) {
		captured = true
		return "should-not-exist", true, nil
	}
	ran := false
	result := runAssistantWriteResult("blocked_tool", func() string {
		ran = true
		return "changed"
	})
	if ran || captured || result.Changed || result.UndoID != "" || result.VerificationError == "" {
		t.Fatalf("unisolated write ran or captured: ran=%v captured=%v result=%+v", ran, captured, result)
	}
	if auditview.SessionActor() != "prior-actor" {
		t.Fatalf("actor after refusal = %q", auditview.SessionActor())
	}
}

func TestRunAssistantWriteResultCapturesPartialPanicAndRestoresActor(t *testing.T) {
	oldBaseline, oldCapture := auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked
	oldActor := auditview.SessionActor()
	auditview.SetSessionActor("prior-actor")
	t.Cleanup(func() {
		auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked = oldBaseline, oldCapture
		auditview.SetSessionActor(oldActor)
	})
	auditview.CaptureAssistantBaselineChecked = func() error { return nil }
	captured := false
	auditview.CaptureAssistantReceiptChecked = func() (string, bool, error) {
		captured = auditview.SessionActor() == auditview.ActorAssistant
		return "partial-receipt", true, nil
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

func TestRunAssistantWriteReportsUnverifiableCaptureWithoutChangedCount(t *testing.T) {
	oldBaseline, oldCapture := auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked
	oldActor := auditview.SessionActor()
	auditview.SetSessionActor("prior-actor")
	t.Cleanup(func() {
		auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked = oldBaseline, oldCapture
		auditview.SetSessionActor(oldActor)
	})
	auditview.CaptureAssistantBaselineChecked = func() error { return nil }
	captureActor := ""
	auditview.CaptureAssistantReceiptChecked = func() (string, bool, error) {
		captureActor = auditview.SessionActor()
		return "", false, errors.New("store snapshot failed")
	}
	before := len(uistate.AssistantActions())
	runs := 0
	result := runAssistantWriteResult("uncertain_tool", func() string {
		runs++
		return "Saved the preference."
	})
	if runs != 1 || result.Changed || result.UndoID != "" || result.VerificationError == "" {
		t.Fatalf("run=%d result=%+v", runs, result)
	}
	if !strings.Contains(result.Text, "Saved the preference.") || !strings.Contains(result.Text, "A change may have been made") {
		t.Fatalf("uncertain result text = %q", result.Text)
	}
	if captureActor != auditview.ActorAssistant || auditview.SessionActor() != "prior-actor" {
		t.Fatalf("capture actor=%q restored actor=%q", captureActor, auditview.SessionActor())
	}
	if len(uistate.AssistantActions()) != before {
		t.Fatal("unverified write created an Assistant action count")
	}
}
