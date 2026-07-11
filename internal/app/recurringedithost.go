// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// RecurringEditHost is the shell-root flip modal for adding or editing a recurring
// cash flow, driven by the recurring:editID atom ("" = closed, "new" = create, or a
// recurring ID = edit). Mounted at the shell root so the modal's position:fixed
// centres on the viewport (bento tiles carry transforms that would offset it). The
// form owns Save/Cancel and clears the atom via OnDone; the close is a bare atom
// set (the reliable InvestAddHost pattern — no BumpDataRevision in the close path).
func RecurringEditHost() uic.Node {
	edit := uistate.UseRecurringEditID()
	rid := edit.Get()
	if rid == "" {
		return Fragment()
	}
	closeModal := func() { edit.Set("") }
	title, saveLabel := uistate.T("recurring.newTitle"), uistate.T("recurring.add")
	if rid != "new" {
		title, saveLabel = uistate.T("recurring.editTitleModal"), uistate.T("recurring.saveFlow")
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:        title,
		Width:        uiw.FlipMediumW,
		Height:       uiw.FlipMediumH,
		FormID:       "recurring-form",
		SaveLabel:    saveLabel,
		SaveTestID:   "rec-save",
		CancelTestID: "rec-cancel",
		OnClose:      closeModal,
		Back:         uic.CreateElement(screens.RecurringForm, screens.RecurringFormProps{ID: rid, OnDone: closeModal}),
	})
}

// BillsSmartHost is the shell-root flip modal for the smart pay schedule: the
// two setup questions (payday + frequency), the live plan preview from the
// deterministic billsched engine, and the Use-plan / Turn-off decision. Driven
// by the bills:smartOpen atom; bare-atom close.
func BillsSmartHost() uic.Node {
	open := uistate.UseBillsSmartOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("bills.smartTitle"),
		Width:    "620px",
		Height:   "min(90vh, 720px)",
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.BillsSmartForm, screens.BillsSmartFormProps{OnDone: closeModal}),
	})
}

// SubsPrefsHost is the shell-root flip modal for the subscription-detection
// preferences (sensitivity + account-type/category ignore filters), driven by the
// subs:prefsOpen atom. Every control inside saves immediately, so the modal has a
// single Done action; closing is a bare atom clear.
func SubsPrefsHost() uic.Node {
	open := uistate.UseSubsPrefsOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("subs.detectPrefsTitle"),
		Width:    "560px",
		Height:   "min(90vh, 620px)",
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.SubsDetectPrefsForm, screens.SubsDetectPrefsFormProps{OnDone: closeModal}),
	})
}
