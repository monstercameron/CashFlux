// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// In-app dialogs that replace the native window.prompt and window.confirm. Each is a titled
// FlipPanel whose body scrolls in a modal-scroll region, with Cancel and the confirm action
// pinned in a modal-foot bar. Escape and a click on the backdrop cancel. The uistate helpers
// (ConfirmModal, PromptModal, PromptSecret) open a request; DialogHost, mounted once in the
// shell, renders it.

const dialogInputID = "cf-dialog-input"

// confirmModal / promptModal are app-package aliases for the shared uistate helpers.
func confirmModal(message string, destructive bool, onResult func(bool)) {
	uistate.ConfirmModal(message, destructive, onResult)
}
func promptModal(message, def string, onResult func(string)) {
	uistate.PromptModal(message, def, onResult)
}

// DialogHost renders the pending dialog as a FlipPanel. It always returns a stable root
// element (empty when no dialog is open) so the framework has an anchor to update, and calls
// its hooks unconditionally (hook order must be stable across renders). It also captures the
// rule-draft atom so that SetRuleDraft, which runs before navigation, notifies the right
// subscriber.
func DialogHost() uic.Node {
	d := uistate.UseDialog()
	uistate.CaptureDialog(d)
	rd := uistate.UseRuleDraft()
	uistate.CaptureRuleDraft(rd)
	req := d.Get()
	open := req != nil

	// entry holds the prompt's text as the person types it. It is a ref rather than state,
	// so typing does not re-render the dialog.
	entry := uic.UseRef("")
	// settled stops a request from being answered twice, so only the first answer counts.
	settled := uic.UseRef(false)
	onInput := uic.UseEvent(func(e uic.Event) { entry.Set(e.GetValue()) })

	finish := func(ok bool) {
		if req == nil || settled.Get() {
			return
		}
		settled.Set(true)
		val := ""
		if ok && req.Kind == uistate.DialogPrompt {
			val = entry.Get()
		}
		d.Set(nil)
		if req.OnResult != nil {
			req.OnResult(ok, val)
		}
	}

	// Re-seed the entry and the answer guard whenever a new request opens, so a previous
	// prompt's text or answer never carries over.
	openSig := "closed"
	if open {
		openSig = string(req.Kind) + ":" + req.Message
	}
	uic.UseLayoutEffect(func() func() {
		settled.Set(false)
		entry.Set("")
		if req != nil {
			entry.Set(req.Default)
		}
		return nil
	}, openSig)

	if !open {
		return Div(css.Class("cf-dialog-root"))
	}

	confirmLabel := req.ConfirmLabel
	if confirmLabel == "" {
		if req.Kind == uistate.DialogPrompt {
			confirmLabel = uistate.T("action.save")
		} else {
			confirmLabel = uistate.T("action.confirm")
		}
	}
	confirmCls := "btn btn-primary"
	title := req.Title
	if req.Destructive {
		confirmCls = "btn btn-danger"
		// A destructive confirm with no title of its own is headed "Are you sure?".
		if title == "" {
			title = uistate.T("dialog.deleteTitle")
		}
	}
	if title == "" {
		// A prompt is headed by its question, so the question is not repeated in the body.
		if req.Kind == uistate.DialogPrompt {
			title = req.Message
		} else {
			title = uistate.T("action.confirm")
		}
	}
	// The scrolling body holds the message, unless the title already says it, and the entry for
	// a prompt.
	fields := []any{}
	if title != req.Message {
		fields = append(fields, P(css.Class("cf-dialog-msg"), Attr("id", "cf-dialog-msg"), req.Message))
	}
	if req.Kind == uistate.DialogPrompt {
		inputType := "text"
		if req.Secret {
			inputType = "password"
		}
		// FieldValue, not Value: the entry is read on confirm rather than bound to state, so a
		// re-render of the host must not rewrite what has been typed.
		fields = append(fields, Input(css.Class("set-input cf-dialog-input"), Attr("id", dialogInputID), Type(inputType),
			Attr("aria-label", req.Message), Attr("autocomplete", "off"), ui.FieldValue(req.Default),
			OnInputHandler(onInput)))
	}

	// Cancel comes first in the pinned bar, so on a destructive confirm the safe choice is the
	// first button in tab order.
	return ui.FlipPanel(ui.FlipPanelProps{
		Title:     title,
		Width:     ui.FlipSmallW,
		Height:    ui.FlipSmallH,
		NoFooter:  true,
		FlushBody: true,
		// Escape and a click on the backdrop close the panel, which counts as a cancel.
		OnSave:  func() { finish(true) },
		OnClose: func() { finish(false) },
		Back: Div(css.Class("cf-dialog-body"),
			Div(append([]any{css.Class("modal-scroll")}, fields...)...),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn"), Type("button"), Attr("id", "cf-dialog-cancel"), OnClick(func() { finish(false) }), uistate.T("action.cancel")),
				Button(ClassStr(confirmCls), Type("button"), Attr("id", "cf-dialog-confirm"), OnClick(func() { finish(true) }), confirmLabel),
			),
		),
	})
}
