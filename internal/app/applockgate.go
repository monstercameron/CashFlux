//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"
)

const appLockGateID = "cf-applock-gate"

// maybeLockOnBoot shows the passcode gate at startup when the lock is enabled, so
// the app's content stays covered until the right passcode is entered. Called once
// from Run after mount.
func maybeLockOnBoot() {
	if loadAppLock().Enabled {
		showAppLockGate()
	}
}

// showAppLockGate covers the whole app with a modal passcode gate (building it on
// first use). A correct passcode hides it. No-op when the lock isn't enabled.
func showAppLockGate() {
	if !loadAppLock().Enabled {
		return
	}
	doc := js.Global().Get("document")
	if gate := doc.Call("getElementById", appLockGateID); !gate.IsNull() && !gate.IsUndefined() {
		gate.Get("style").Set("display", "grid")
		resetAppLockInput(doc)
		return
	}
	buildAppLockGate(doc)
}

func resetAppLockInput(doc js.Value) {
	if inp := doc.Call("getElementById", "cf-applock-input"); !inp.IsNull() && !inp.IsUndefined() {
		inp.Set("value", "")
		inp.Call("focus")
	}
}

func buildAppLockGate(doc js.Value) {
	gate := doc.Call("createElement", "div")
	gate.Set("id", appLockGateID)
	gate.Get("style").Set("cssText", "position:fixed;inset:0;z-index:1000;display:grid;place-items:center;background:var(--bg,#0e0e0f);")

	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "display:flex;flex-direction:column;gap:0.8rem;width:min(90vw,320px);text-align:center;color:var(--text,#f4f4f5);")
	card.Set("innerHTML", `<div style="font-family:Fraunces,Georgia,serif;font-size:1.4rem;font-weight:600;">CashFlux</div>`+
		`<div id="cf-applock-msg" style="font-size:0.9rem;opacity:0.7;">Enter your passcode to unlock</div>`)

	inp := doc.Call("createElement", "input")
	inp.Set("id", "cf-applock-input")
	inp.Set("type", "password")
	inp.Call("setAttribute", "inputmode", "numeric")
	inp.Call("setAttribute", "aria-label", "Passcode")
	inp.Get("style").Set("cssText", "width:100%;box-sizing:border-box;padding:0.6rem 0.8rem;text-align:center;font-size:1.1rem;letter-spacing:0.2em;background:var(--bg-elev,#1a1a1d);border:1px solid var(--border,#2a2a2c);border-radius:8px;color:inherit;outline:none;")
	card.Call("appendChild", inp)

	btn := doc.Call("createElement", "button")
	btn.Set("type", "button")
	btn.Set("textContent", "Unlock")
	btn.Get("style").Set("cssText", "padding:0.6rem 0.8rem;border-radius:8px;border:0;background:var(--accent,#2e8b57);color:#052e13;font-weight:600;cursor:pointer;")
	card.Call("appendChild", btn)

	gate.Call("appendChild", card)
	doc.Get("body").Call("appendChild", gate)

	attempt := func() {
		if loadAppLock().Verify(inp.Get("value").String()) {
			gate.Get("style").Set("display", "none")
			return
		}
		if msg := doc.Call("getElementById", "cf-applock-msg"); !msg.IsNull() && !msg.IsUndefined() {
			msg.Set("textContent", "Wrong passcode — try again")
			msg.Get("style").Set("color", "var(--danger,#d8716f)")
		}
		inp.Set("value", "")
		inp.Call("focus")
	}

	btnCb := js.FuncOf(func(js.Value, []js.Value) any { attempt(); return nil })
	btn.Call("addEventListener", "click", btnCb)
	keyCb := js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 && a[0].Get("key").String() == "Enter" {
			a[0].Call("preventDefault")
			attempt()
		}
		return nil
	})
	inp.Call("addEventListener", "keydown", keyCb)

	resetAppLockInput(doc)
}

// setPasscodeFlow prompts for a passcode (twice, to confirm) and enables the lock.
// Uses native prompts for the MVP; a proper in-app form is a follow-up.
func setPasscodeFlow() {
	p := js.Global().Call("prompt", "Set a passcode for CashFlux:")
	if p.IsNull() || p.IsUndefined() {
		return
	}
	pass := strings.TrimSpace(p.String())
	if pass == "" {
		return
	}
	c := js.Global().Call("prompt", "Confirm the passcode:")
	if c.IsNull() || c.IsUndefined() || strings.TrimSpace(c.String()) != pass {
		js.Global().Call("alert", "The passcodes didn't match — nothing changed.")
		return
	}
	if enableAppLock(pass, 0) {
		js.Global().Call("alert", "Passcode lock enabled. You'll be asked for it next time you open or lock CashFlux.")
	}
}
