// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/interop"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// Standard flip-modal sizes. Every config or edit modal should pick one of these (width,
// height) pairs instead of hand-tuning its own numbers, so the app's modals feel consistent.
// The height is a maximum: tall content scrolls inside the panel and the footer stays in view.
const (
	// FlipSmall is for compact, single-purpose panels such as a top-up amount, a contribution
	// or a short confirm (one to three fields).
	FlipSmallW, FlipSmallH = "440px", "440px"
	// FlipMedium is for standard add and edit forms: budgets, goals, tracked categories,
	// auto-budget and income basis.
	FlipMediumW, FlipMediumH = "560px", "680px"
	// FlipLarge is for dense, multi-section panels such as global settings and the
	// month-close and track-untracked reviews.
	FlipLargeW, FlipLargeH = "760px", "720px"

	// FlipFullW and FlipFullH give the near-full-viewport size for a panel that hosts a
	// working surface rather than a form, such as the review inbox's bulk list. The panel
	// stays a modal at 90% of the viewport instead of becoming a route.
	FlipFullW, FlipFullH = "90vw", "90vh"
)

// FlipPanelProps configures a FlipPanel overlay.
type FlipPanelProps struct {
	Title  string   // title shown in the header
	Back   uic.Node // body content, shown in the scrolling area of the back face
	Width  string   // panel width; defaults to "384px" (prefer a Flip*W standard size)
	Height string   // maximum panel height; defaults to "470px" (prefer a Flip*H standard size)
	// FlushBody removes the body's padding and makes it a full-height flex column, so a
	// NoFooter form can split itself into a `.modal-scroll` field region and a pinned
	// `.modal-foot` action bar that stays put when the form is taller than the panel.
	// It is meant for a NoFooter form. It still changes the body's layout with the
	// standard footer, so do not combine the two.
	FlushBody bool
	// FormID pins the standard Save/Cancel footer. Its Save button submits the form with
	// this id, which sits in the scrolling body. The form's OnSubmit does the work, and its
	// OnDone closes the panel on success and leaves it open on a validation error, so the
	// form needs no action bar of its own. Cancel dismisses via OnClose. Use it for simple
	// Cancel and Save forms. For a footer with an extra action such as Delete, use FlushBody
	// with a `.modal-foot` bar instead.
	FormID string
	// LeftFooter is an optional action placed before Cancel. It is shown only with FormID.
	LeftFooter uic.Node
	// SaveLabel overrides the primary button text ("Save" by default), for example "Add goal"
	// or "Apply". It applies to the FormID footer and the OnSave footer.
	SaveLabel string
	// SaveTestID and CancelTestID override the footer buttons' data-testid. A form that moved
	// its own submit and cancel into the standard footer uses them to keep the ids its
	// end-to-end tests look for, such as "rec-save" and "rec-cancel". Save defaults to
	// "flip-save", and Cancel has no default id.
	SaveTestID   string
	CancelTestID string
	// OnSave is called when Save is pressed, and then the panel closes. A FormID footer
	// does not call it.
	OnSave func()
	// OnClose is called on Cancel, on the header close button, on Escape and on a backdrop
	// click. With the OnSave footer it is also called after Save.
	OnClose func()
	// CloseOnly replaces the Cancel and Save footer with a single Close button. Use it for
	// panels that have nothing to save, such as a widget with no settings, so the UI does
	// not suggest there is something to commit.
	CloseOnly bool
	// SaveDisabled greys out and blocks the Save button while the form is invalid, such as
	// when a required field is empty, so an invalid submit cannot close the panel or discard
	// input. While it is true, neither the Save button nor Enter runs OnSave, and with a FormID
	// footer Enter does not ask the form to submit either.
	SaveDisabled bool
	// NoFooter renders no footer at all. Use it when the Back body supplies its own action
	// buttons, so the panel does not show two sets of buttons. The header close button,
	// Escape and a backdrop click still dismiss via OnClose.
	NoFooter bool
}

// FlipPanel is the settings overlay shared by per-widget and global settings and by the
// account, budget and transaction editors. A dimmed, blurred backdrop holds a card that lifts
// and flips (3D rotateY) from a neutral front face to a back face. The back face has a title,
// a close button, a scrollable body, and a Save and Cancel footer unless NoFooter or CloseOnly
// says otherwise. Callers supply the title, the body and the size. The open animation runs
// once, on mount.
//
// The card is a GWC modal overlay. The overlay traps Tab inside the panel, makes the page
// behind it inert, and dismisses on Escape or a backdrop click. Escape goes only to the topmost
// overlay, so a popover or a nested panel opened from this one closes first. FlipPanel picks
// the initial focus itself (see useFlipPanelInitialFocus) and restores focus on unmount (see
// restoreFlipPanelFocus).
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

	// Generated ids contain colons, which a CSS selector cannot use unescaped. The id is
	// used in the initial-focus selector, so the colons are replaced here.
	surfaceID := "flip-" + strings.ReplaceAll(uic.UseId(), ":", "-")

	// The opener is the element that had focus before the panel mounted. The first render
	// records it, because effects run after the render and focus may have moved by then.
	opener := uic.UseRef(interop.Value{})
	openerRecorded := uic.UseRef(false)
	if !openerRecorded.Get() {
		openerRecorded.Set(true)
		opener.Set(domDocument().Get("activeElement"))
	}
	useFlipPanelInitialFocus(surfaceID)
	// Return focus to the opener when the panel unmounts.
	uic.UseEffect(func() func() {
		return func() { restoreFlipPanelFocus(opener.Get()) }
	}, true)

	onClose, onSave := props.OnClose, props.OnSave
	save := func() {
		// A disabled Save does nothing. OnSave is not called and the panel stays open, so no
		// entered input is lost.
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

	// Enter confirms, as in a native form, unless the active control keeps its own Enter (see
	// flipPanelEnterBelongsToPanel). With a FormID footer, Enter asks that form to submit, and
	// the form closes the panel when it saves. Otherwise Enter runs save. CloseOnly ignores Enter.
	onEnter := uic.UseEvent(func(e uic.KeyboardEvent) {
		if e.GetKey() != "Enter" || props.CloseOnly {
			return
		}
		if !flipPanelEnterBelongsToPanel(domDocument().Get("activeElement"), props.FormID) {
			return
		}
		e.PreventDefault()
		if props.FormID != "" {
			if props.SaveDisabled {
				return
			}
			domValue(domNode(props.FormID), "requestSubmit")
			return
		}
		save()
	})

	backdropCls, innerCls := "flip-backdrop", "flip-inner"
	if shown.Get() {
		backdropCls += " show"
		innerCls += " flipped"
	}

	bodyCls := "set-body"
	if props.FlushBody {
		bodyCls += " set-body-flush"
	}

	width, height := props.Width, props.Height
	if width == "" {
		width = "384px"
	}
	if height == "" {
		height = "470px"
	}

	saveLabel := props.SaveLabel
	if saveLabel == "" {
		saveLabel = uistate.T("action.save")
	}
	saveTestID := props.SaveTestID
	if saveTestID == "" {
		saveTestID = "flip-save"
	}
	saveClick := OnClick(save)
	cancelArgs := []any{css.Class("set-btn cancel"), Type("button"), OnClick(cancel), uistate.T("action.cancel")}
	if props.CancelTestID != "" {
		cancelArgs = append(cancelArgs, Attr("data-testid", props.CancelTestID))
	}
	var foot uic.Node
	switch {
	case props.NoFooter:
		// The Back body owns its own actions; render no footer at all.
		foot = Fragment()
	case props.CloseOnly:
		// CloseOnly takes precedence over FormID, so a caller can keep FormID set for a later
		// stage and still show only Close here. Close runs the same path as Save, so OnSave is
		// called if it is set, and SaveDisabled also blocks Close.
		foot = Div(css.Class("set-foot"),
			Button(css.Class("set-btn close"), Type("button"), saveClick, uistate.T("action.close")),
		)
	case props.FormID != "":
		// Save is a native submit for the body form. The form does the work in OnSubmit and
		// closes the panel through OnDone, so the panel does not close here.
		saveArgs := []any{css.Class("set-btn save"), Type("submit"), Attr("form", props.FormID), Attr("data-testid", saveTestID), saveLabel}
		if props.SaveDisabled {
			saveArgs = append(saveArgs, Attr("disabled", ""), Attr("aria-disabled", "true"))
		}
		foot = Div(css.Class("set-foot"),
			props.LeftFooter,
			Button(cancelArgs...),
			Button(saveArgs...),
		)
	default:
		saveArgs := []any{css.Class("set-btn save"), Type("button"), Attr("data-testid", saveTestID), saveClick, saveLabel}
		if props.SaveDisabled {
			saveArgs = append(saveArgs, Attr("disabled", ""), Attr("aria-disabled", "true"))
		}
		foot = Div(css.Class("set-foot"),
			Button(cancelArgs...),
			Button(saveArgs...),
		)
	}

	// The dialog is labelled by the back face's H3, so its accessible name is the visible
	// title. The id comes from the title, so two open panels with the same title share an id.
	titleID := "flip-title-" + slugifyTitle(props.Title)

	// Height is a maximum, not a fixed size. The wrap sizes to its content, never shorter than
	// 280px and never taller than min(Height, 86vh). A sparse panel hugs its content, and a dense
	// panel stops at the maximum and scrolls. The back face is the in-flow face, so an auto
	// height follows its content.
	panel := Div(css.Class("flip-wrap"), Style(map[string]string{
		"width":      width,
		"height":     "auto",
		"min-height": "280px",
		"max-height": "min(" + height + ", 86vh)",
	}),
		OnKeyDown(onEnter),
		Div(ClassStr(innerCls),
			// Front face: a neutral card seen during the flip. It is decorative, so assistive
			// technology skips it. Otherwise its title would be read twice.
			Div(css.Class("flip-face"), Attr("aria-hidden", "true"),
				Div(css.Class("wh"), Span(css.Class("grip"), Icon(icon.Grip, css.Class(tw.W4, tw.H4))), H3(props.Title)),
			),
			// Back face — the settings panel.
			Div(css.Class("flip-face flip-back"),
				Div(css.Class("set-h"),
					// An empty spacer. The stylesheet hides it on the back face, and a test
					// checks that it is rendered.
					Span(css.Class("flip-title-balance"), Style(map[string]string{"width": "1.5rem"})),
					H3(Attr("id", titleID), props.Title),
					Button(css.Class("set-close"), Type("button"), Attr("title", uistate.T("action.close")), Attr("aria-label", uistate.T("action.close")),
						OnClick(cancel),
						Icon(icon.Close, css.Class(tw.W4, tw.H4)),
					),
				),
				Div(ClassStr(bodyCls), props.Back),
				foot,
			),
		),
	)

	return uic.Overlay(uic.OverlayProps{
		Open:                true,
		Kind:                uic.OverlayKindDialog,
		Modal:               true,
		SurfaceID:           surfaceID,
		LabelledBy:          titleID,
		BackdropClass:       backdropCls,
		CloseOnOutsideClick: true,
		OnDismiss:           cancel,
		Child:               panel,
	})
}

// flipPanelInitialFocusSelector returns the selector for the control that takes focus when a
// panel opens: an [autofocus] field if there is one, otherwise the first field, choice or
// button in the body. The header and footer buttons sit outside the body, so they are never
// the default. querySelector returns the first match in document order, so an [autofocus]
// field that comes after an earlier body field does not win.
func flipPanelInitialFocusSelector(surfaceID string) string {
	scope := "#" + surfaceID + " "
	return scope + "[autofocus], " +
		scope + ".set-body input:not([disabled]), " +
		scope + ".set-body select, " +
		scope + ".set-body textarea, " +
		scope + ".set-body button"
}

// focusRetryFrames is how many animation frames the initial-focus pass tries before it gives
// up. A body that renders after the panel gets those frames to appear, and a panel with
// nothing focusable stops after the last one.
const focusRetryFrames = 4

// useFlipPanelInitialFocus moves focus to the panel's first body control once that control is
// in the DOM. The panel mounts before its body child renders, so the first attempt can find
// nothing. The attempt is retried over a few frames and stops at once when it succeeds. If the
// panel unmounts first, the pending frame is cancelled.
func useFlipPanelInitialFocus(surfaceID string) {
	uic.UseEffect(func() func() {
		selector := flipPanelInitialFocusSelector(surfaceID)
		attempts := 0
		var cancelFrame func()
		var attempt func(float64)
		attempt = func(float64) {
			attempts++
			if target := domValue(domDocument(), "querySelector", selector); target.Truthy() {
				domValue(target, "focus", map[string]any{"preventScroll": true})
				return
			}
			if attempts < focusRetryFrames {
				cancelFrame = interop.RequestAnimationFrame(attempt)
			}
		}
		cancelFrame = interop.RequestAnimationFrame(attempt)
		return func() {
			if cancelFrame != nil {
				cancelFrame()
			}
		}
	}, surfaceID)
}

// restoreFlipPanelFocus returns focus to the opener when the panel unmounts. If the opener was
// replaced while the panel was open, focus goes to the control that now stands in for it. If
// focus still does not land, it goes to the page's <main> (the element with id "main"), so
// keyboard users resume on the page and not on the document body.
func restoreFlipPanelFocus(opener interop.Value) {
	target := flipPanelRestoreTarget(opener)
	domValue(target, "focus")
	if active := domDocument().Get("activeElement"); active.Truthy() && domValue(active, "isSameNode", target).Bool() {
		return
	}
	if landmark := domNode("main"); landmark.Truthy() {
		domValue(landmark, "setAttribute", "tabindex", "-1")
		domValue(landmark, "focus")
	}
}

// flipPanelRestoreTarget returns the opener while it is still in the page. Otherwise it returns
// the current element with the same id or data-testid, or the opener itself if there is none.
func flipPanelRestoreTarget(opener interop.Value) interop.Value {
	if !opener.Truthy() {
		return opener
	}
	connected := opener.Get("isConnected")
	if !connected.Present() || connected.Bool() {
		return opener
	}
	if id := opener.Get("id").String(); id != "" {
		if match := domNode(id); match.Truthy() {
			return match
		}
	}
	if testID := domValue(opener, "getAttribute", "data-testid").String(); testID != "" {
		matches := domValue(domDocument(), "querySelectorAll", "[data-testid]")
		for i := range matches.Get("length").Int() {
			match := domValue(matches, "item", i)
			if domValue(match, "getAttribute", "data-testid").String() == testID {
				return match
			}
		}
	}
	return opener
}

// flipPanelEnterBelongsToPanel reports whether the panel should handle Enter for the active
// control. With no active control the panel handles it. Buttons, text areas and selects keep
// their own Enter. A control inside a form keeps that form's native submit, unless the form is
// the panel's FormID footer form.
func flipPanelEnterBelongsToPanel(active interop.Value, formID string) bool {
	if !active.Truthy() {
		return true
	}
	switch strings.ToUpper(active.Get("tagName").String()) {
	case "TEXTAREA", "BUTTON", "SELECT":
		return false
	}
	form := active.Get("form")
	if !form.Truthy() {
		return true
	}
	return formID != "" && form.Get("id").String() == formID
}

// slugifyTitle lower-cases a dialog title, turns spaces and hyphens into hyphens and drops every
// other character. The result works as the back-face H3's element id for aria-labelledby.
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
