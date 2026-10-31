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

// The in-app replacements for window.prompt and window.confirm. Each request is a FlipPanel
// with a titled header, a scrolling modal-scroll body and a modal-foot bar holding Cancel and
// the confirm button. Escape and a backdrop click cancel. The uistate helpers (ConfirmModal,
// PromptModal, PromptSecret and their variants) queue a request, and DialogHost, mounted once
// in the shell, renders the pending one.

// dialogInputID is the DOM id of a prompt's text field.
const dialogInputID = "cf-dialog-input"

// confirmModal and promptModal are the short names the app package uses for the uistate helpers.
func confirmModal(message string, destructive bool, onResult func(bool)) {
	uistate.ConfirmModal(message, destructive, onResult)
}
func promptModal(message, def string, onResult func(string)) {
	uistate.PromptModal(message, def, onResult)
}

// DialogHost renders the pending dialog as a FlipPanel, or an empty placeholder div when none
// is open, so the shell always has a node to update. Its hooks run before that early return,
// so the hook order is the same in every render. It also captures the rule-draft atom so that
// SetRuleDraft can notify its subscriber.
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
	// The body shows the message unless the heading already is the message. It holds the
	// text field for a prompt.
	fields := []any{css.Class("modal-scroll")}
	if title != req.Message {
		fields = append(fields, P(css.Class("cf-dialog-msg"), Attr("id", "cf-dialog-msg"), req.Message))
	}
	if req.Kind == uistate.DialogPrompt {
		inputType := "text"
		if req.Secret {
			inputType = "password"
		}
		// FieldValue sets the starting text without binding it to state. The entry is read on
		// confirm, so a re-render must not overwrite what the person has typed.
		fields = append(fields, Input(css.Class("set-input cf-dialog-input"), Attr("id", dialogInputID), Type(inputType),
			Attr("aria-label", req.Message), Attr("autocomplete", "off"), ui.FieldValue(req.Default),
			OnInputHandler(onInput)))
	}

	// Quick picks fill the entry when one is chosen. Each pick is its own keyed component, so
	// its click handler sits at a stable position however many picks there are.
	if req.Kind == uistate.DialogPrompt && len(req.Choices) > 0 {
		picks := MapKeyedComponent(req.Choices, func(c string) any { return c }, func(c string) uic.Node {
			return Button(css.Class("btn btn-sm cf-dialog-choice"), Type("button"), Attr("data-testid", "cf-dialog-choice"),
				OnClick(func() {
					entry.Set(c)
					ui.SetFieldValue(dialogInputID, c)
				}), c)
		})
		fields = append(fields, Div(css.Class("cf-dialog-choices"), Attr("role", "group"), Attr("aria-label", uistate.T("dialog.quickPicks")), picks))
	}

	// Cancel is first in the bar and in tab order, so on a destructive confirm the safe choice
	// comes first.
	return ui.FlipPanel(ui.FlipPanelProps{
		Title:     title,
		Width:     ui.FlipSmallW,
		Height:    ui.FlipSmallH,
		NoFooter:  true,
		FlushBody: true,
		// Enter reaches OnSave. Escape, a backdrop click and the header close button reach
		// OnClose, which is a cancel. finish ignores every answer after the first.
		OnSave:  func() { finish(true) },
		OnClose: func() { finish(false) },
		Back: Div(css.Class("cf-dialog-body"),
			Div(fields...),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn"), Type("button"), Attr("id", "cf-dialog-cancel"), OnClick(func() { finish(false) }), uistate.T("action.cancel")),
				Button(ClassStr(confirmCls), Type("button"), Attr("id", "cf-dialog-confirm"), OnClick(func() { finish(true) }), confirmLabel),
			),
		),
	})
}
