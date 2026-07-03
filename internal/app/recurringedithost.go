// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
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
	title := uistate.T("recurring.newTitle")
	if rid != "new" {
		title = uistate.T("recurring.editTitleModal")
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    title,
		Width:    "560px",
		Height:   "min(90vh, 700px)",
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.RecurringForm, screens.RecurringFormProps{ID: rid, OnDone: closeModal}),
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
