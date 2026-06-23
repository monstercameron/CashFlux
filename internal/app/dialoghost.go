//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// In-app modal dialogs that replace the native window.prompt/confirm/alert (C42):
// accessible, themed, keyboard-operable (Enter confirms, Esc/backdrop cancels).
// The DialogHost (mounted once in the shell) renders the pending request and
// captures its atom into uistate; the helpers below (and uistate.Prompt/ConfirmModal
// for the screens package) open one.

const dialogInputID = "cf-dialog-input"

// confirmModal / promptModal are app-package aliases for the shared uistate helpers.
func confirmModal(message string, destructive bool, onResult func(bool)) {
	uistate.ConfirmModal(message, destructive, onResult)
}
func promptModal(message, def string, onResult func(string)) {
	uistate.PromptModal(message, def, onResult)
}

func dialogInputValue() string {
	el := js.Global().Get("document").Call("getElementById", dialogInputID)
	if !el.Truthy() {
		return ""
	}
	return el.Get("value").String()
}

// DialogHost renders the pending modal dialog. It always returns a stable root
// element (empty when no dialog is open) so the framework has an anchor to update,
// and calls its hooks unconditionally (hook order must be stable across renders).
// It also captures the rule-draft atom so that SetRuleDraft (called before
// navigation) notifies the correct subscriber.
func DialogHost() uic.Node {
	d := uistate.UseDialog()
	uistate.CaptureDialog(d)
	rd := uistate.UseRuleDraft()
	uistate.CaptureRuleDraft(rd)
	req := d.Get()
	open := req != nil

	finish := func(ok bool) {
		if req == nil {
			return
		}
		val := ""
		if ok && req.Kind == uistate.DialogPrompt {
			val = dialogInputValue()
		}
		d.Set(nil)
		if req.OnResult != nil {
			req.OnResult(ok, val)
		}
	}

	// While a dialog is open: Enter confirms, Escape cancels, and focus moves in.
	// Re-runs when the open state or request changes; a no-op while closed.
	openSig := "closed"
	if open {
		openSig = string(req.Kind) + ":" + req.Message
	}
	uic.UseEffect(func() func() {
		if !open {
			return nil
		}
		doc := js.Global().Get("document")
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			e := args[0]
			switch e.Get("key").String() {
			case "Enter":
				if req.Kind == uistate.DialogPrompt {
					e.Call("preventDefault")
				}
				finish(true)
			case "Escape":
				finish(false)
			case "Tab":
				// Focus trap (WCAG 2.4.3): cycle focus among the dialog's controls so
				// Tab/Shift+Tab can't escape the modal.
				focusables := js.Global().Get("document").Call("querySelectorAll",
					".cf-dialog button, .cf-dialog input, .cf-dialog [tabindex]")
				n := focusables.Get("length").Int()
				if n == 0 {
					return nil
				}
				first := focusables.Index(0)
				last := focusables.Index(n - 1)
				activeEl := js.Global().Get("document").Get("activeElement")
				if e.Get("shiftKey").Bool() {
					if activeEl.Equal(first) {
						e.Call("preventDefault")
						last.Call("focus")
					}
				} else if activeEl.Equal(last) {
					e.Call("preventDefault")
					first.Call("focus")
				}
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", cb)
		focusID := "cf-dialog-confirm"
		if req.Kind == uistate.DialogPrompt {
			focusID = dialogInputID
		} else if req.Destructive {
			// Default focus to Cancel for destructive confirms so Enter can't
			// accidentally trigger the danger action (WCAG SC 3.2.4).
			focusID = "cf-dialog-cancel"
		}
		var fcb js.Func
		fcb = js.FuncOf(func(js.Value, []js.Value) any {
			fcb.Release()
			if el := doc.Call("getElementById", focusID); el.Truthy() {
				el.Call("focus")
				if req.Kind == uistate.DialogPrompt {
					el.Call("select")
				}
			}
			return nil
		})
		js.Global().Call("setTimeout", fcb, 30)
		return func() { doc.Call("removeEventListener", "keydown", cb); cb.Release() }
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
	dialogRole := "dialog"
	if req.Destructive {
		confirmCls = "btn btn-danger"
		// alertdialog is announced with urgency by screen readers (ARIA APG).
		dialogRole = "alertdialog"
		// Auto-derive a title for destructive confirms that didn't supply one,
		// so the heading is always present for aria-labelledby.
		if req.Title == "" {
			req.Title = uistate.T("dialog.deleteTitle")
		}
	}

	panel := Div(css.Class("cf-dialog"),
		If(req.Title != "", H3(css.Class("cf-dialog-title"), Attr("id", "cf-dialog-title"), req.Title)),
		P(css.Class("cf-dialog-msg"), req.Message),
		If(req.Kind == uistate.DialogPrompt,
			Input(css.Class("set-input cf-dialog-input"), Attr("id", dialogInputID), Type("text"),
				Attr("aria-label", req.Message), Value(req.Default))),
		Div(css.Class("cf-dialog-actions"),
			Button(css.Class("btn"), Type("button"), Attr("id", "cf-dialog-cancel"), OnClick(func() { finish(false) }), uistate.T("action.cancel")),
			Button(ClassStr(confirmCls), Type("button"), Attr("id", "cf-dialog-confirm"), OnClick(func() { finish(true) }), confirmLabel),
		),
	)
	// Scrim is a sibling of the panel so clicking the panel never bubbles a cancel.
	return Div(css.Class("cf-dialog-root"),
		Div(css.Class("cf-dialog-backdrop"), Attr("role", dialogRole), Attr("aria-modal", "true"),
			Attr("aria-labelledby", "cf-dialog-title"),
			Div(css.Class("cf-dialog-scrim"), OnClick(func() { finish(false) })),
			panel,
		),
	)
}
