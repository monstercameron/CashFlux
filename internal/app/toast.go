// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// Notices stay visible for eight seconds of active reading time.
const (
	toastTimeoutMS     = 8000
	toastUndoTimeoutMS = 10000
	toastErrTimeoutMS  = 8000
	toastTickMS        = 100
)

type toastDismissProps struct {
	Atom state.Atom[uistate.Notice]
}

type toastUndoProps struct {
	Atom state.Atom[uistate.Notice]
}

func toastDismissButton(props toastDismissProps) uic.Node {
	dismiss := uic.UseEvent(func() { props.Atom.Set(props.Atom.Get().Cleared()) })
	return Button(css.Class("toast-x"), Attr("type", "button"), Attr("title", uistate.T("action.dismiss")), Attr("aria-label", uistate.T("action.dismiss")), OnClick(dismiss), "×")
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
		commitToastUndo(props.Atom, func(error) { pending.Set(false) })
	})

	label := uistate.T("toast.undoBtn")
	if label == "toast.undoBtn" {
		label = uistate.T("tc4n9.undo")
	}
	if pending.Get() {
		label = uistate.T("tc4n9.savingUndo")
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

func toastV5RowCopy(text string) (string, string) {
	for _, prefix := range []string{"Saved. ", "Saved — "} {
		if !strings.HasPrefix(text, prefix) {
			continue
		}
		account, balance, ok := strings.Cut(strings.TrimPrefix(text, prefix), " is now ")
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

func toastDurabilityFailureIsRetryable(err error) bool {
	return err != nil && !errors.Is(err, appstate.ErrStaleDataset)
}

func toastV5RowCountsTime(visibility string, hovered, focused bool) bool {
	return visibility == "visible" && !hovered && !focused
}

// Toast renders the app-wide notice surface: a single dismissible message
// pinned to the bottom of the viewport, driven by the shared uistate.Notice
// atom. Ordinary notices auto-dismiss after toastTimeoutMS; errors linger for
// toastErrTimeoutMS (and can always be dismissed by hand).
//
// Inline Undo requires an explicit undoable notice and a committed undo point.
// Informational notices explicitly opt out of dataset persistence; write notices
// wait for durability unless their caller has already awaited the strict commit.
func Toast() uic.Node {
	atom := uistate.UseNotice()
	n := atom.Get()
	confirmed := uic.UseState(uistate.Notice{})
	app := appstate.Default
	needsCommit := toastNeedsDurable(n, app)
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
						if toastDurabilityFailureIsRetryable(err) {
							uistate.ReportPersistFailure("notice", acknowledge)
						}
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
				atom.Set(atom.Get().WithTransient(msg))
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
		timeout := toastTimeoutFor(n)
		if n.Err {
			timeout = toastErrTimeoutMS
		}
		clock := toastReadingClock{last: js.Global().Get("performance").Call("now").Float()}
		var id js.Value
		var cb js.Func
		// Hidden pages may stop interval callbacks entirely. Reset the baseline
		// on visibility transitions so returning cannot charge hidden time.
		document := js.Global().Get("document")
		visibilityReset := js.FuncOf(func(js.Value, []js.Value) any {
			clock.last = js.Global().Get("performance").Call("now").Float()
			return nil
		})
		listening := document.Truthy() && document.Get("addEventListener").Type() == js.TypeFunction && document.Get("removeEventListener").Type() == js.TypeFunction
		if listening {
			document.Call("addEventListener", "visibilitychange", visibilityReset)
		}
		stopped := false
		stop := func() {
			if !stopped {
				stopped = true
				js.Global().Call("clearInterval", id)
				cb.Release()
				if listening {
					document.Call("removeEventListener", "visibilitychange", visibilityReset)
				}
				visibilityReset.Release()
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
			doc := js.Global().Get("document")
			toast := js.Undefined()
			if doc.Truthy() && doc.Get("querySelector").Type() == js.TypeFunction {
				toast = doc.Call("querySelector", `[data-testid="app-notice"]`)
			}
			visibility := ""
			if doc.Truthy() {
				visibility = doc.Get("visibilityState").String()
			}
			hovered, focused := false, false
			if toast.Truthy() && toast.Get("matches").Type() == js.TypeFunction && toast.Get("contains").Type() == js.TypeFunction {
				hovered = toast.Call("matches", ":hover").Bool()
				focused = toast.Call("contains", doc.Get("activeElement")).Bool()
			}
			if clock.expired(now, float64(timeout), toastV5RowCountsTime(visibility, hovered, focused)) {
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

	showUndo := toastOffersUndo(n)
	message, balanceLine := toastV5RowCopy(n.Text)

	return withSaveFailureBanner(Div(ClassStr(cls), Attr("data-testid", "app-notice"), Attr("role", role), Attr("aria-live", live),
		Span(css.Class("toast-msg"), message),
		If(balanceLine != "", Span(css.Class("toast-balance"), balanceLine)),
		If(showUndo, uic.CreateElement(toastUndoButton, toastUndoProps{Atom: atom})),
		uic.CreateElement(toastDismissButton, toastDismissProps{Atom: atom}),
	))
}

func toastNeedsDurable(n uistate.Notice, app *appstate.App) bool {
	return n.Text != "" && !n.Err && !n.Durable && !n.NonDataset && app != nil && durableNoticeApp == app
}

func toastOffersUndo(n uistate.Notice) bool {
	return !n.Err && !n.NonDataset && n.Undoable && auditview.CanUndoFunc()
}

func toastTimeoutFor(n uistate.Notice) int {
	if n.Undoable && !n.Err && !n.NonDataset {
		return toastUndoTimeoutMS
	}
	if n.Err {
		return toastErrTimeoutMS
	}
	return toastTimeoutMS
}

// commitToastUndo uses the same commit completion boundary as forward actions.
// Replay owns the history cursor and strict durable request; failed persistence
// must retry that restoration rather than pop a second history entry.
func commitToastUndo(atom state.Atom[uistate.Notice], done func(error)) {
	commitUndoFeedbackWithFailure(false, func(err error) {
		if err == nil {
			if n := atom.Get(); !n.Durable || n.Text != uistate.T("tv8h.undone") {
				atom.Set(n.WithDurable(uistate.T("tv8h.undone"), false))
			}
		}
		if done != nil {
			done(err)
		}
	}, func() {
		atom.Set(atom.Get().With(uistate.T("tv8h.undoNotSaved"), true))
	})
}

// commitUndoFeedback also supports the keyboard and Activity entry points.
func commitUndoFeedback(redo bool, done func(error)) {
	commitUndoFeedbackWithFailure(redo, done, nil)
}

func commitUndoFeedbackWithFailure(redo bool, done func(error), showFailure func()) {
	app := appstate.Default
	label := uistate.T("tv8h.undone")
	if redo {
		label = uistate.T("tv8h.redone")
	}
	var replay func(func(error))
	replay = func(complete func(error)) { undoLastChangeThen(redo, complete) }
	var retry func()
	retry = func() {
		if appstate.Default != app {
			return
		}
		screens.CommitUndoableReplay(app, label, replay, func(err error) {
			if err != nil {
				if app != nil && undoPersistencePending && toastDurabilityFailureIsRetryable(err) {
					cursor := undoDurableCursor
					// Autosave may finish before the banner is clicked. Retrying
					// durability directly can never pop another history entry.
					replay = func(complete func(error)) {
						app.RequestDurable(func(result appstate.DurableResult) {
							if appstate.Default != app {
								return
							}
							if result.Err == nil && undoDurableCursor == cursor {
								undoPersistencePending = false
							}
							complete(result.Err)
						})
					}
					uistate.ReportPersistFailure("transaction-undo", retry)
				}
				if showFailure != nil {
					showFailure()
				} else {
					uistate.PostNotice(uistate.T("tv8h.undoNotSaved"), true)
				}
			} else {
				uistate.ClearPersistFailure("transaction-undo")
			}
			if done != nil {
				done(err)
			}
		})
	}
	retry()
}

// toastReadingClock counts only the intervals during which feedback can be read.
// Visibility events reset last because browsers can suspend background timers.
type toastReadingClock struct {
	last, elapsed float64
}

func (clock *toastReadingClock) expired(now, timeout float64, reading bool) bool {
	if reading && now > clock.last {
		clock.elapsed += now - clock.last
	}
	clock.last = now
	return clock.elapsed >= timeout
}
