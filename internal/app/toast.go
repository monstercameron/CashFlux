// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// How long a notice stays on screen before auto-dismissing. Errors linger longer
// than ordinary notices so there's time to read what went wrong.
const (
	toastTimeoutMS    = 4500
	toastErrTimeoutMS = 7500
	toastTickMS       = 100
)

// toastUndoProps are the props for the toastUndoButton sub-component.
type toastUndoProps struct {
	// Atom is the notice atom so the Undo action can clear the toast after undo.
	Atom state.Atom[uistate.Notice]
}

// toastUndoButton is a dedicated component so that its UseEvent hook is
// registered at a stable (non-loop) call-site — required by the GoWebComponents
// framework (CLAUDE.md §"CRITICAL gotchas"). It renders the "Undo" button
// inside a Toast whose notice represents an undoable change.
func toastUndoButton(props toastUndoProps) uic.Node {
	pending := uic.UseState(false)
	doUndo := uic.UseEvent(func() {
		if pending.Get() {
			return
		}
		pending.Set(true)
		undoLastChangeThen(false, func(err error) {
			pending.Set(false)
			if err != nil {
				uistate.PostNotice(uistate.T("bv6d1.undoSaveFailed"), true)
				return
			}
			props.Atom.Set(props.Atom.Get().Cleared())
		})
	})

	label := uistate.T("toast.undoBtn")
	if label == "toast.undoBtn" {
		label = "Undo"
	}
	if pending.Get() {
		label = "Saving undo..."
	}
	return Button(
		Disabled(pending.Get()),
		css.Class("toast-undo"),
		Attr("type", "button"),
		Attr("aria-label", label),
		OnClick(doUndo),
		label,
	)
}

// toastNoticeIsUndoable reports whether the notice text looks like it
// represents a delete or change that the user may want to undo. This is a
// heuristic used when the Notice model has no explicit Undoable field.
//
// Follow-up: add a Notice.Undoable bool field to uistate.Notice so callers
// (e.g. delete handlers) can set it directly, removing the need for this
// text-matching heuristic entirely.
func toastNoticeIsUndoable(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "deleted") ||
		strings.Contains(lower, "removed") ||
		strings.Contains(lower, "changed") ||
		strings.Contains(lower, "updated") ||
		strings.Contains(lower, "moved") ||
		strings.Contains(lower, "archived")
}

func toastV5RowCopy(text string) (string, string) {
	if strings.HasPrefix(text, "Saved — ") {
		account, balance, ok := strings.Cut(strings.TrimPrefix(text, "Saved — "), " is now ")
		if ok {
			balance = strings.TrimSuffix(strings.TrimSpace(balance), ".")
			return uistate.T("v5row.transactionSaved"), fmt.Sprintf(uistate.T("v5row.accountBalance"), account, balance)
		}
	}
	if text == "Transaction updated." || text == "Updated" || text == "Saved!" {
		return uistate.T("v5row.transactionSaved"), ""
	}
	return text, ""
}

func toastV5RowCountsTime(visibility string, hovered, focused bool) bool {
	return visibility == "visible" && !hovered && !focused
}

// Toast renders the app-wide notice surface: a single dismissible message
// pinned to the bottom of the viewport, driven by the shared uistate.Notice
// atom. Ordinary notices auto-dismiss after toastTimeoutMS; errors linger for
// toastErrTimeoutMS (and can always be dismissed by hand).
//
// Inline Undo: when the undo stack is non-empty (auditview.CanUndoFunc()) and
// the notice text matches a delete/change pattern, an inline "Undo" button
// appears. Clicking it calls auditview.UndoFunc() and dismisses the toast.
//
// Cleaner follow-up: add Notice.Undoable bool to uistate.Notice and set it
// from delete/change handlers, replacing the text-pattern heuristic below.
func Toast() uic.Node {
	atom := uistate.UseNotice()
	n := atom.Get()
	confirmed := uic.UseState(uistate.Notice{})
	app := appstate.Default
	needsCommit := n.Text != "" && !n.Err && app != nil && durableNoticeApp == app
	requested := n
	// Gate presentation rather than English text: every positive notice goes
	// through the same boundary, including budget writes, translations and prefs.
	// Sequence/app checks prevent an older completion replacing a newer notice.
	uic.UseEffect(func() func() {
		active := true
		stopRecovery := func() {}
		uistate.ClearPersistFailure("notice")
		if needsCommit {
			var acknowledge func()
			acknowledge = func() {
				if !active || appstate.Default != app || atom.Get().Seq != requested.Seq || confirmed.Get().Seq == requested.Seq {
					return
				}
				requestNoticeDurable(app, func(err error) {
					if !active || appstate.Default != app || atom.Get().Seq != requested.Seq {
						return
					}
					if err != nil {
						uistate.ReportPersistFailure("notice", acknowledge)
						return
					}
					uistate.ClearPersistFailure("notice")
					confirmed.Set(requested)
				})
			}
			stopRecovery = uistate.OnPersistRecovered(acknowledge)
			acknowledge()
		}
		return func() { active = false; stopRecovery() }
	}, n.Seq, needsCommit)
	if needsCommit && confirmed.Get().Seq != n.Seq {
		n = uistate.Notice{Seq: n.Seq}
	}

	// Wire the app's workflow "notify" action to this toast surface, once. The
	// captured atom lets a notice posted from event time (a workflow run) show up
	// here without calling a hook outside render.
	uic.UseEffect(func() func() {
		if appstate.Default != nil {
			appstate.Default.Notifier = func(msg string) {
				atom.Set(atom.Get().With(msg, false))
			}
		}
		return nil
	}, "wire-notifier")

	// Count only time while the document is visible and the toast is neither
	// hovered nor focused. This keeps long notices available for reading without
	// tying expiry to window focus (which is unreliable in embedded browsers).
	uic.UseEffect(func() func() {
		if n.Text == "" {
			return nil
		}
		timeout := toastTimeoutMS
		if n.Err {
			timeout = toastErrTimeoutMS
		}
		elapsed := 0.0
		last := js.Global().Get("performance").Call("now").Float()
		var id js.Value
		var cb js.Func
		stopped := false
		stop := func() {
			if !stopped {
				stopped = true
				js.Global().Call("clearInterval", id)
				cb.Release()
			}
		}
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			// An already-queued callback may outlive effect cleanup or its App.
			// It must never expire a newer notice or inspect another page's DOM.
			if appstate.Default != app || atom.Get().Seq != n.Seq {
				stop()
				return nil
			}
			now := js.Global().Get("performance").Call("now").Float()
			delta := now - last
			last = now
			doc := js.Global().Get("document")
			toast := js.Undefined()
			if doc.Truthy() && doc.Get("querySelector").Type() == js.TypeFunction {
				toast = doc.Call("querySelector", ".toast")
			}
			visible := doc.Truthy() && doc.Get("visibilityState").String() == "visible"
			hovered, focused := false, false
			if toast.Truthy() && toast.Get("matches").Type() == js.TypeFunction && toast.Get("contains").Type() == js.TypeFunction {
				hovered = toast.Call("matches", ":hover").Bool()
				focused = toast.Call("contains", doc.Get("activeElement")).Bool()
			}
			if toastV5RowCountsTime(map[bool]string{true: "visible", false: "hidden"}[visible], hovered, focused) {
				elapsed += delta
			}
			if elapsed >= float64(timeout) {
				stop()
				atom.Set(n.Cleared())
			}
			return nil
		})
		id = js.Global().Call("setInterval", cb, toastTickMS)
		// Cleanup clears the interval and releases its callback when another notice
		// replaces this one or the toast surface unmounts.
		return stop
	}, n.Seq, n.Text)

	// Errors interrupt (assertive/alert); ordinary notices are announced politely.
	// Keeping the same live-region element mounted across the empty and active
	// states — rather than mounting a fresh node only when there's text — is what
	// makes screen readers reliably announce each new notice.
	live, role := "polite", "status"
	if n.Err {
		live, role = "assertive", "alert"
	}
	if n.Text == "" {
		// Idle: an empty, visually-hidden live region stays in the DOM so the next
		// post is announced (a region inserted together with its text often isn't).
		return withSaveFailureBanner(Div(css.Class(tw.SrOnly), Attr("role", role), Attr("aria-live", live)))
	}
	cls := "toast"
	if n.Err {
		cls += " toast-err"
	}

	// Show the inline Undo button when the undo stack has something to undo and
	// the notice text indicates a destructive or mutating operation.
	// Prefer the explicit Notice.Undoable flag (set by delete/change handlers);
	// fall back to the text heuristic for callers that haven't been updated yet.
	showUndo := auditview.CanUndoFunc() && (n.Undoable || toastNoticeIsUndoable(n.Text))
	message, balanceLine := toastV5RowCopy(n.Text)

	return withSaveFailureBanner(Div(ClassStr(cls), Attr("role", role), Attr("aria-live", live),
		Span(css.Class("toast-msg"), message),
		If(balanceLine != "", Span(css.Class("toast-balance"), balanceLine)),
		If(showUndo, uic.CreateElement(toastUndoButton, toastUndoProps{Atom: atom})),
		Button(css.Class("toast-x"), Attr("type", "button"), Attr("title", uistate.T("action.dismiss")), Attr("aria-label", uistate.T("action.dismiss")),
			OnClick(func() { atom.Set(n.Cleared()) }), "×"),
	))
}
