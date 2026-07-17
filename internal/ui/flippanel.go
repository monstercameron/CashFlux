// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// Standard flip-modal sizes. Every config/edit modal should pick ONE of these three
// (width, height) pairs instead of hand-tuning its own numbers, so the app's modals
// feel consistent. Tall content still fits — the body scrolls and the footer stays
// pinned (see FlushBody), so the size is a target, not a hard clamp.
const (
	// FlipSmall — compact, single-purpose panels: a top-up amount, a contribution, a
	// short confirm (roughly 1–3 fields).
	FlipSmallW, FlipSmallH = "440px", "440px"
	// FlipMedium — standard entity add/edit forms: budget, goal, tracked categories,
	// auto-budget, income basis.
	FlipMediumW, FlipMediumH = "560px", "680px"
	// FlipLarge — dense, multi-section or wide panels: global settings, add-account.
	FlipLargeW, FlipLargeH = "760px", "720px"
)

// FlipPanelProps configures a FlipPanel overlay.
type FlipPanelProps struct {
	Title  string   // settings title (centered in the header)
	Back   uic.Node // settings body content (scrolls inside the back face)
	Width  string   // panel width, default "384px" (prefer a Flip*W standard size)
	Height string   // panel height, default "470px" (prefer a Flip*H standard size)
	// FlushBody makes the scrollable body a full-height flex column with no padding, so
	// a NoFooter form can split itself into a `.modal-scroll` field region and a pinned
	// `.modal-foot` action bar — the action bar then stays put instead of scrolling off
	// the bottom when the form is taller than the panel. Only meaningful with NoFooter
	// (a form that supplies its own footer); ignored by the standard Save/Cancel footer.
	FlushBody bool
	// FormID pins a STANDARD Save/Cancel footer (identical across the app) whose Save is
	// a native submit for the form with this id (rendered in the scrollable body). Save
	// submits the form — the form's OnSubmit does the work and its OnDone closes the
	// panel on success, leaving it open on a validation error — so the form needs no
	// action bar of its own. Cancel dismisses via OnClose. Preferred for simple
	// Cancel+Save forms; use FlushBody + a `.modal-foot` bar only when the footer needs an
	// extra action (e.g. Delete).
	FormID string
	// SaveLabel overrides the primary button text ("Save" by default) — e.g. "Add goal",
	// "Apply". Applies to the FormID and the OnSave footers.
	SaveLabel string
	// SaveTestID / CancelTestID override the footer buttons' data-testid, so a form that
	// moved its own submit/cancel into the standard footer keeps the ids its e2e tests
	// target (e.g. "rec-save"/"rec-cancel"). Save defaults to "flip-save"; Cancel has none.
	SaveTestID   string
	CancelTestID string
	OnSave       func() // invoked on Save (then the panel closes)
	OnClose      func() // invoked on Cancel/close (and after Save)
	// CloseOnly replaces the Cancel/Save footer with a single Close button — for
	// panels that have nothing to save (e.g. a widget with no settings), so the UI
	// doesn't imply there's something to commit.
	CloseOnly bool
	// SaveDisabled greys out and blocks the Save button while the form is invalid
	// (e.g. a required field is empty), so an invalid submit can't close the panel
	// or discard input (L78-T1). OnSave is not invoked while it is true.
	SaveDisabled bool
	// NoFooter renders no Save/Cancel/Close footer at all — for panels whose Back body
	// supplies its own action buttons (so the modal isn't double-chromed). The header
	// ✕, Escape, and backdrop-click still dismiss via OnClose.
	NoFooter bool
}

// FlipPanel is the candidate-C settings overlay shared by both per-widget and
// global settings: a dimmed/blurred backdrop centering a card that lifts and
// flips (3D rotateY) from a neutral front to a settings back face with a
// centered title, a close button, a scrollable body, and a dark Save/Cancel
// footer. Generic and props-driven — callers supply the title, the body, and the
// size. The open animation runs once on mount.
func FlipPanel(props FlipPanelProps) uic.Node {
	return uic.CreateElement(flipPanel, props)
}

func flipPanel(props FlipPanelProps) uic.Node {
	// shown drives both the backdrop fade-in and the card flip; it flips to true
	// once just after mount so the CSS transition animates.
	shown := uic.UseState(false)
	uic.UseEffect(func() func() {
		if !shown.Get() {
			shown.Set(true)
		}
		return nil
	}, true)

	// Modal keyboard behavior: Esc closes; Tab is trapped inside the dialog;
	// focus moves into the dialog on open and is restored to the trigger on close.
	// The listener is added on mount and removed on unmount, which (since the panel
	// mounts fresh each open and unmounts on close) matches the dialog's lifetime.
	onCloseRef := props.OnClose
	onSaveRef := props.OnSave
	closeOnly := props.CloseOnly
	formID := props.FormID
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return nil
		}
		// Remember what had focus so we can restore it when the dialog closes.
		prevFocus := doc.Get("activeElement")

		// focusables lists the dialog's tabbable elements in DOM order.
		focusables := func() []js.Value {
			wrap := doc.Call("querySelector", ".flip-wrap")
			if wrap.IsNull() || wrap.IsUndefined() {
				return nil
			}
			list := wrap.Call("querySelectorAll", "a[href], button, input, select, textarea, [tabindex]")
			out := make([]js.Value, 0, list.Get("length").Int())
			for i := 0; i < list.Get("length").Int(); i++ {
				el := list.Index(i)
				if el.Call("getAttribute", "tabindex").String() == "-1" {
					continue
				}
				if d := el.Get("disabled"); !d.IsUndefined() && d.Bool() {
					continue
				}
				out = append(out, el)
			}
			return out
		}

		// Move focus into the dialog so keyboard/SR users start inside the modal
		// rather than behind it. C43: prefer an element explicitly marked [autofocus]
		// (e.g. quick-add's Amount field) over the first focusable, so a form can land
		// the cursor on the field the user actually fills first; fall back to fs[0].
		if fs := focusables(); len(fs) > 0 {
			target := fs[0]
			for _, el := range fs {
				if el.Call("hasAttribute", "autofocus").Bool() {
					target = el
					break
				}
			}
			target.Call("focus")
		}

		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			e := args[0]
			switch e.Get("key").String() {
			case "Escape":
				if onCloseRef != nil {
					onCloseRef()
				}
			case "Enter":
				// Enter submits (Save) like a native form. Skip when focus is on a
				// button (let it click), in a textarea (multi-line) or a select, and
				// when the panel has nothing to save (CloseOnly).
				if closeOnly {
					return nil
				}
				if active := doc.Get("activeElement"); !active.IsNull() && !active.IsUndefined() {
					switch active.Get("tagName").String() {
					case "TEXTAREA", "BUTTON", "SELECT":
						return nil
					}
				}
				e.Call("preventDefault")
				// A FormID footer submits its form (native submit → OnSubmit); the form's
				// OnDone closes on success and it stays open on a validation error.
				if formID != "" {
					if f := doc.Call("getElementById", formID); !f.IsNull() && !f.IsUndefined() {
						f.Call("requestSubmit")
					}
					return nil
				}
				if onSaveRef != nil {
					onSaveRef()
				}
				if onCloseRef != nil {
					onCloseRef()
				}
			case "Tab":
				fs := focusables()
				if len(fs) == 0 {
					return nil
				}
				first, last := fs[0], fs[len(fs)-1]
				active := doc.Get("activeElement")
				if e.Get("shiftKey").Bool() {
					if active.Equal(first) {
						e.Call("preventDefault")
						last.Call("focus")
					}
				} else if active.Equal(last) {
					e.Call("preventDefault")
					first.Call("focus")
				}
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", cb)

		// GM4-19: clicking the blurred backdrop (outside the panel card) closes the
		// panel. Implemented here (rather than as an OnClick prop) so we can check
		// event.target against the backdrop element and ignore clicks on the card.
		backdropCb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			t := args[0].Get("target")
			bd := doc.Call("querySelector", ".flip-backdrop")
			if !bd.IsNull() && !bd.IsUndefined() && t.Equal(bd) {
				if onCloseRef != nil {
					onCloseRef()
				}
			}
			return nil
		})
		doc.Call("addEventListener", "click", backdropCb)

		return func() {
			doc.Call("removeEventListener", "keydown", cb)
			cb.Release()
			doc.Call("removeEventListener", "click", backdropCb)
			backdropCb.Release()
			if !prevFocus.IsNull() && !prevFocus.IsUndefined() {
				prevFocus.Call("focus")
				// #67: the trigger may have vanished by close time (e.g. an add-menu
				// item inside a since-closed popover) — focus() then silently no-ops
				// and keyboard users land on <body>. Fall back to the main content
				// landmark so focus resumes at the page, not nowhere.
				if !doc.Get("activeElement").Equal(prevFocus) {
					if m := doc.Call("getElementById", "main"); !m.IsNull() && !m.IsUndefined() {
						m.Call("setAttribute", "tabindex", "-1")
						m.Call("focus")
					}
				}
			}
		}
	}, true)

	width, height := props.Width, props.Height
	if width == "" {
		width = "384px"
	}
	if height == "" {
		height = "470px"
	}

	backdropCls, innerCls := "flip-backdrop", "flip-inner"
	if shown.Get() {
		backdropCls += " show"
		innerCls += " flipped"
	}

	bodyCls := "set-body"
	if props.FlushBody {
		bodyCls += " set-body-flush"
	}

	onClose, onSave := props.OnClose, props.OnSave

	save := func() {
		// Block save while the form is invalid: don't run OnSave and, crucially,
		// don't close — so no entered input is lost (L78-T1).
		if props.SaveDisabled {
			return
		}
		if onSave != nil {
			onSave()
		}
		if onClose != nil {
			onClose()
		}
	}
	cancel := func() {
		if onClose != nil {
			onClose()
		}
	}
	saveLabel := props.SaveLabel
	if saveLabel == "" {
		saveLabel = uistate.T("action.save")
	}
	saveTestID := props.SaveTestID
	if saveTestID == "" {
		saveTestID = "flip-save"
	}
	cancelArgs := []any{css.Class("set-btn cancel"), Type("button"), OnClick(cancel), uistate.T("action.cancel")}
	if props.CancelTestID != "" {
		cancelArgs = append(cancelArgs, Attr("data-testid", props.CancelTestID))
	}
	var foot uic.Node
	switch {
	case props.NoFooter:
		// The Back body owns its own actions; render no footer at all.
		foot = Fragment()
	case props.FormID != "":
		// Standard pinned footer whose Save is a native submit for the body form — the
		// form does the work in OnSubmit and closes via OnDone, so no auto-close here.
		saveArgs := []any{css.Class("set-btn save"), Type("submit"), Attr("form", props.FormID), Attr("data-testid", saveTestID), saveLabel}
		if props.SaveDisabled {
			saveArgs = append(saveArgs, Attr("disabled", ""), Attr("aria-disabled", "true"))
		}
		foot = Div(css.Class("set-foot"),
			Button(cancelArgs...),
			Button(saveArgs...),
		)
	case props.CloseOnly:
		// Nothing to save: a single Close button (no Cancel/Save pair).
		// GM2-10: use .set-btn.close (neutral dismiss styling) not .set-btn.save
		// (green/primary styling) — so the button reads as "dismiss" not "submit".
		foot = Div(css.Class("set-foot"),
			Button(css.Class("set-btn close"), Type("button"), OnClick(save), uistate.T("action.close")),
		)
	default:
		saveArgs := []any{css.Class("set-btn save"), Type("button"), Attr("data-testid", saveTestID), OnClick(save), saveLabel}
		if props.SaveDisabled {
			saveArgs = append(saveArgs, Attr("disabled", ""), Attr("aria-disabled", "true"))
		}
		foot = Div(css.Class("set-foot"),
			Button(cancelArgs...),
			Button(saveArgs...),
		)
	}

	// UX-08: the dialog is labelled BY its one visible back-face H3
	// (aria-labelledby) instead of a parallel aria-label, so the accessible name
	// and the visible title can never drift apart. The id is derived from the
	// title (only one flip modal is ever open at a time).
	titleID := "flip-title-" + slugifyTitle(props.Title)
	return Div(ClassStr(backdropCls),
		Div(css.Class("flip-wrap"), Style(map[string]string{"width": width, "height": height}),
			Attr("role", "dialog"), Attr("aria-modal", "true"), Attr("aria-labelledby", titleID),
			Div(ClassStr(innerCls),
				// Front face — a neutral card briefly seen during the flip. It is
				// DECORATIVE: without aria-hidden its H3 title sat in the
				// accessibility tree beside the back face's real title, so every
				// flip modal announced its heading twice (QA L3/CF-29).
				Div(css.Class("flip-face"), Attr("aria-hidden", "true"),
					Div(css.Class("wh"), Span(css.Class("grip"), Icon(icon.Grip, css.Class(tw.W4, tw.H4))), H3(props.Title)),
				),
				// Back face — the settings panel.
				Div(css.Class("flip-face flip-back"),
					Div(css.Class("set-h"),
						Span(Style(map[string]string{"width": "1.5rem"})), // balance the close button so the title centers
						H3(Attr("id", titleID), props.Title),
						// GM4-17: tabindex="-1" removes the close button from the Tab order so
						// initial focus lands on the first form control (a setting), not the ×
						// button. The button remains fully mouse/click accessible.
						Button(css.Class("set-close"), Type("button"), Attr("title", uistate.T("action.close")), Attr("tabindex", "-1"),
							OnClick(func() {
								if onClose != nil {
									onClose()
								}
							}),
							Icon(icon.Close, css.Class(tw.W4, tw.H4)),
						),
					),
					Div(ClassStr(bodyCls), props.Back),
					foot,
				),
			),
		),
	)
}

// slugifyTitle lower-cases a dialog title and keeps [a-z0-9-] so it can serve
// as the back-face H3's element id for aria-labelledby.
func slugifyTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
