//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/router"
)

// wireKeyboardShortcuts installs global keyboard shortcuts. Alt+1..9 jumps to the
// Nth primary navigation screen (Dashboard, Accounts, …) so the keyboard alone can
// move between sections. Registered once at boot; the listener lives for the app's
// lifetime, so its js.Func is intentionally never released.
//
// It keys off KeyboardEvent.code ("Digit1".."Digit9") so it's keyboard-layout
// independent and never matches the numpad (where Alt+number is an OS alt-code),
// and it stays out of the way while the user is typing in a field.
func wireKeyboardShortcuts() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	nav := primaryNav() // static — the screen set doesn't change at runtime

	onKeyDown := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		e := args[0]
		key := e.Get("key").String()
		// Esc dismisses the help overlay (no-op when it's closed); FlipPanel keeps
		// handling Esc for open settings panels independently.
		if key == "Escape" {
			closeHelpOverlay()
			return nil
		}
		if isEditableTarget(doc) {
			return nil
		}
		// "?" toggles the keyboard cheat sheet.
		if key == "?" {
			e.Call("preventDefault")
			toggleHelpOverlay()
			return nil
		}
		if !e.Get("altKey").Bool() || e.Get("ctrlKey").Bool() || e.Get("metaKey").Bool() {
			return nil
		}
		code := e.Get("code").String()
		if len(code) != 6 || code[:5] != "Digit" {
			return nil
		}
		d := code[5]
		if d < '1' || d > '9' {
			return nil
		}
		idx := int(d - '1')
		if idx >= len(nav) {
			return nil
		}
		e.Call("preventDefault")
		router.Navigate(nav[idx].Path)
		return nil
	})
	doc.Call("addEventListener", "keydown", onKeyDown)
}

// isEditableTarget reports whether focus is in a text input, so a shortcut chord
// doesn't fire (and steal the keystroke) while the user is typing.
func isEditableTarget(doc js.Value) bool {
	el := doc.Get("activeElement")
	if el.IsNull() || el.IsUndefined() {
		return false
	}
	if tag := el.Get("tagName"); !tag.IsNull() && !tag.IsUndefined() {
		switch tag.String() {
		case "INPUT", "TEXTAREA", "SELECT":
			return true
		}
	}
	if ce := el.Get("isContentEditable"); !ce.IsNull() && !ce.IsUndefined() && ce.Bool() {
		return true
	}
	return false
}

const helpOverlayID = "cf-help-overlay"

// helpHTML is the shortcuts cheat sheet body. (English for now — a follow-up can
// route these through the i18n catalog like the rest of the UI.)
const helpHTML = `<div style="display:flex;justify-content:space-between;align-items:center;gap:1rem;margin-bottom:0.8rem;">
  <strong style="font-size:1rem;">Keyboard shortcuts</strong>
  <button id="cf-help-close" type="button" aria-label="Close" style="background:transparent;border:0;color:inherit;cursor:pointer;font-size:1.15rem;line-height:1;min-width:24px;min-height:24px;">&times;</button>
</div>
<table style="width:100%;border-collapse:collapse;">
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Jump to a section</td><td style="text-align:right;white-space:nowrap;">Alt + 1&ndash;9</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Save the open settings panel</td><td style="text-align:right;white-space:nowrap;">Enter</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Close a panel / this help</td><td style="text-align:right;white-space:nowrap;">Esc</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Show dashboard resize handles</td><td style="text-align:right;white-space:nowrap;">Hold Shift</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Toggle this help</td><td style="text-align:right;white-space:nowrap;">?</td></tr>
</table>`

// toggleHelpOverlay shows or hides the keyboard cheat sheet, building it on first
// use. It's a self-contained DOM overlay (not a framework component), so the
// shortcut layer owns it end to end and nothing else has to mount it.
func toggleHelpOverlay() {
	doc := js.Global().Get("document")
	ov := doc.Call("getElementById", helpOverlayID)
	if ov.IsNull() || ov.IsUndefined() {
		buildHelpOverlay(doc)
		return
	}
	style := ov.Get("style")
	if style.Get("display").String() == "none" {
		style.Set("display", "grid")
	} else {
		style.Set("display", "none")
	}
}

// closeHelpOverlay hides the cheat sheet if it's open (a no-op otherwise).
func closeHelpOverlay() {
	doc := js.Global().Get("document")
	if ov := doc.Call("getElementById", helpOverlayID); !ov.IsNull() && !ov.IsUndefined() {
		ov.Get("style").Set("display", "none")
	}
}

// buildHelpOverlay creates the overlay once and appends it to <body>, visible.
// Subsequent opens just toggle its display. The click/close js.Funcs live for the
// app's lifetime (intentionally not released), matching the persistent overlay.
func buildHelpOverlay(doc js.Value) {
	ov := doc.Call("createElement", "div")
	ov.Set("id", helpOverlayID)
	ov.Get("style").Set("cssText", "position:fixed;inset:0;z-index:200;display:grid;place-items:center;background:rgba(0,0,0,0.55);")

	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "background:var(--bg-elev,#1a1a1d);color:var(--text,#f4f4f5);border:1px solid var(--border,#2a2a2c);border-radius:10px;padding:1.1rem 1.35rem;max-width:min(92vw,440px);box-shadow:0 12px 40px rgba(0,0,0,0.5);font-size:0.9rem;line-height:1.5;")
	card.Set("innerHTML", helpHTML)
	ov.Call("appendChild", card)

	// Click the dimmed backdrop (not the card) to dismiss.
	backdropCb := js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 && a[0].Get("target").Equal(ov) {
			ov.Get("style").Set("display", "none")
		}
		return nil
	})
	ov.Call("addEventListener", "click", backdropCb)

	doc.Get("body").Call("appendChild", ov)

	// Wire the ✕ button inside the card.
	if x := doc.Call("getElementById", "cf-help-close"); !x.IsNull() && !x.IsUndefined() {
		closeCb := js.FuncOf(func(js.Value, []js.Value) any {
			ov.Get("style").Set("display", "none")
			return nil
		})
		x.Call("addEventListener", "click", closeCb)
	}
}
