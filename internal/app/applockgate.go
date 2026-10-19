// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/CashFlux/internal/keyring"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

const appLockGateID = "cf-applock-gate"
const smartQuoteCode = "SMART-QUOTE"

var errProtectedStorageUnavailable = errors.New("protected storage is unavailable")
var errLegacyDatasetLocked = errors.New("legacy dataset requires its original passcode")
var appLockClosing bool
var protectedGateCallbacks []js.Func
var errPendingLockTransition = errors.New("an App Lock change needs to finish")

// protectedGateText uses only the built-in English catalogue. uistate.T would
// initialize language preferences by reading protected records before unlock.
var protectedGateCatalog = i18n.DefaultBundle()

func protectedGateText(key string) string { return protectedGateCatalog.T(i18n.English, key) }

func appLockActive() bool {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return appLockClosing
	}
	gate := doc.Call("getElementById", appLockGateID)
	return appLockClosing || gate.Truthy() && gate.Get("style").Get("display").String() != "none"
}

func setMuzakLocked(on bool) {
	if muzak := js.Global().Get("cashfluxMuzak"); muzak.Truthy() && muzak.Get("setLocked").Truthy() {
		muzak.Call("setLocked", on)
	}
}

func unlockGate(doc, gate js.Value) {
	gate.Get("style").Set("display", "none")
	if root := doc.Call("getElementById", "app"); root.Truthy() {
		root.Set("inert", false)
	}
	setMuzakLocked(false)
}

// showAppLockGate waits for the secure write queue and key destruction before
// offering unlock. Gate visibility also blocks shortcuts while that work runs.
func showAppLockGate() {
	if appLockClosing || appLockActive() {
		return
	}
	state := browserstore.SecureKeyState()
	if state == nil || !state.AppLock {
		return
	}
	appLockClosing = true
	suspendAutosave = true
	rememberPasskeyForLock()
	setMuzakLocked(true)
	if root := js.Global().Get("document").Call("getElementById", "app"); root.Truthy() {
		root.Set("inert", true)
	}
	go func() {
		// Finish writes already queued before LockSecureThen stops cache access.
		browserstore.Barrier()
		browserstore.LockSecureThen(func() {
			activePasscode = ""
			clearDatasetKeyCache()
			clearArtifactKeyCache()
			appLockClosing = false
			showProtectedStorageGate(atrest.ErrLocked, func() { suspendAutosave = false })
		})
	}()
}

// showProtectedStorageGate reads public key metadata only. It never seeds,
// deletes, repairs automatically, or offers a destructive reset action.
func showProtectedStorageGate(cause error, onReady func()) {
	doc := js.Global().Get("document")
	for _, callback := range protectedGateCallbacks {
		callback.Release()
	}
	protectedGateCallbacks = nil
	if old := doc.Call("getElementById", appLockGateID); old.Truthy() {
		old.Call("remove")
	}
	gate := doc.Call("createElement", "div")
	gate.Set("id", appLockGateID)
	gate.Call("setAttribute", "role", "dialog")
	gate.Call("setAttribute", "aria-modal", "true")
	gate.Call("setAttribute", "aria-labelledby", "cf-protected-title")
	gate.Get("style").Set("cssText", "position:fixed;inset:0;z-index:2147483647;display:grid;place-items:center;background:var(--bg,#111);color:var(--text,#f4f4f5);padding:1rem;")
	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "display:flex;flex-direction:column;gap:1rem;width:min(90vw,380px);padding:1.5rem;background:var(--bg-elev,#1a1a1d);border:1px solid var(--border,#444);border-radius:12px;")
	gate.Call("appendChild", card)
	add := func(tag, id, text string) js.Value {
		e := doc.Call("createElement", tag)
		e.Set("id", id)
		e.Set("textContent", text)
		card.Call("appendChild", e)
		return e
	}
	add("h1", "cf-protected-title", protectedGateText("enc31.title"))
	message := add("p", "cf-applock-msg", protectedGateText("enc31.preserved"))
	message.Call("setAttribute", "role", "status")
	message.Call("setAttribute", "aria-live", "polite")
	input := add("input", "cf-applock-input", "")
	input.Set("type", "password")
	input.Call("setAttribute", "aria-label", protectedGateText("enc31.passcode"))
	input.Call("setAttribute", "autocomplete", "current-password")
	unlock := add("button", "cf-protected-unlock", protectedGateText("enc31.unlock"))
	recovery := add("button", "cf-protected-recovery", protectedGateText("enc31.recovery"))
	passkey := add("button", "cf-protected-passkey", protectedGateText("enc31.passkey"))
	passkey.Get("style").Set("display", "none")
	if lockedPasskey[0] != "" {
		passkey.Get("style").Set("display", "block")
	}

	repair := add("button", "cf-protected-repair", protectedGateText("enc31.repair"))
	repair.Get("style").Set("display", "none")
	help := add("button", "cf-protected-help", protectedGateText("enc31.help"))
	state := browserstore.SecureKeyState()
	legacy := errors.Is(cause, errLegacyDatasetLocked)
	pendingChange := errors.Is(cause, errPendingLockTransition)
	unlock.Set("disabled", !legacy && !pendingChange && (state == nil || !state.AppLock))
	canRecover := false
	if state != nil && state.Ring != nil {
		for _, w := range state.Ring.Current().Wraps {
			if w.Kind == keyring.WrapRecovery {
				canRecover = true
			}
		}
	}
	recovery.Set("disabled", legacy || pendingChange || !canRecover)
	if pendingChange {
		unlock.Set("textContent", protectedGateText("enc31.retry"))
		input.Set("disabled", true)
	}
	useRecovery, busy, explicitlyUnlocked, finished := false, false, false, false
	needsRepair := errors.Is(cause, atrest.ErrMissingDeviceKey)
	finish := func() {
		if finished {
			return
		}
		finished = true
		input.Set("value", "")
		unlockGate(doc, gate)
		if onReady != nil {
			onReady()
		}
	}
	completed := func(err error) {
		busy = false
		if err != nil {
			message.Set("textContent", protectedGateText("enc31.failed"))
			input.Set("value", "")
			input.Call("focus")
			return
		}
		explicitlyUnlocked = true
		if needsRepair {
			message.Set("textContent", protectedGateText("enc31.repairPrompt"))
			repair.Get("style").Set("display", "block")
			unlock.Set("textContent", protectedGateText("enc31.continue"))
			recovery.Set("disabled", true)
			input.Set("value", "")
			input.Set("disabled", true)
			return
		}
		finish()
	}
	attempt := func() {
		if busy {
			return
		}
		if explicitlyUnlocked {
			finish()
			return
		}
		if unlock.Get("disabled").Bool() {
			return
		}
		busy = true
		secret := input.Get("value").String()
		message.Set("textContent", protectedGateText("enc31.opening"))
		if pendingChange {
			go func() { _, _, err := recoverSecureLockTransition(); completed(err) }()
		} else if legacy {
			unlockLegacyProfile(secret, completed)
		} else {
			unlockProtectedSession(secret, useRecovery, completed)
		}
	}
	listen := func(e js.Value, event string, f func()) {
		cb := js.FuncOf(func(js.Value, []js.Value) any { f(); return nil })
		e.Call("addEventListener", event, cb)
		protectedGateCallbacks = append(protectedGateCallbacks, cb)
	}
	listen(unlock, "click", attempt)
	listen(passkey, "click", func() {
		if busy || explicitlyUnlocked {
			return
		}
		busy = true
		unlockWithPasskey(doc, gate, completed)
	})
	listen(recovery, "click", func() {
		if busy || recovery.Get("disabled").Bool() {
			return
		}
		useRecovery = !useRecovery
		label := "enc31.passcode"
		action := "enc31.recovery"
		if useRecovery {
			label = "enc31.recoveryLabel"
			action = "enc31.usePasscode"
		}
		recovery.Set("textContent", protectedGateText(action))
		input.Call("setAttribute", "aria-label", protectedGateText(label))
		unlock.Set("disabled", false)
		input.Set("value", "")
		input.Call("focus")
	})
	listen(repair, "click", func() {
		if busy || !explicitlyUnlocked {
			return
		}
		busy = true
		browserstore.RepairDeviceSecure(func(err error) {
			busy = false
			if err != nil {
				message.Set("textContent", protectedGateText("enc31.repairFailed"))
				return
			}
			finish()
		})
	})
	listen(help, "click", func() { message.Set("textContent", protectedGateText("enc31.helpText")) })
	key := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Get("key").String() == "Enter" {
			args[0].Call("preventDefault")
			attempt()
		}
		if len(args) > 0 && args[0].Get("key").String() == "Tab" {
			controls := []js.Value{input, unlock, recovery, passkey, repair, help}
			var enabled []js.Value
			for _, e := range controls {
				if !e.Get("disabled").Truthy() && e.Get("style").Get("display").String() != "none" {
					enabled = append(enabled, e)
				}
			}
			if len(enabled) > 0 {
				first, last := enabled[0], enabled[len(enabled)-1]
				if args[0].Get("shiftKey").Bool() && doc.Get("activeElement").Equal(first) {
					args[0].Call("preventDefault")
					last.Call("focus")
				} else if !args[0].Get("shiftKey").Bool() && doc.Get("activeElement").Equal(last) {
					args[0].Call("preventDefault")
					first.Call("focus")
				}
			}
		}
		return nil
	})
	gate.Call("addEventListener", "keydown", key)
	protectedGateCallbacks = append(protectedGateCallbacks, key)
	doc.Get("body").Call("appendChild", gate)
	if root := doc.Call("getElementById", "app"); root.Truthy() {
		root.Set("inert", true)
	}
	setMuzakLocked(true)
	input.Call("focus")
}

// setPasscodeFlow opens the in-app passcode setup form (no refresh callback —
// used from the command palette, which closes on its own).
func setPasscodeFlow() { showAppLockSetup(nil) }

const appLockSetupID = "cf-applock-setup"

// appLockOnDone is invoked after the setup form successfully enables the lock, so
// a caller (e.g. the Settings panel) can refresh. Set on each open.
var appLockOnDone func()

// escT returns an HTML-escaped translated string, for safe innerHTML interpolation.
func escT(key string) string { return htmlEscaper.Replace(uistate.T(key)) }

// showAppLockSetup opens the passcode setup form — an in-app modal (replacing the
// MVP's native prompts, per UX audit §6.8) — building it on first use.
func showAppLockSetup(onDone func()) {
	appLockOnDone = onDone
	doc := js.Global().Get("document")
	if s := doc.Call("getElementById", appLockSetupID); !s.IsNull() && !s.IsUndefined() {
		s.Get("style").Set("display", "grid")
		resetSetupFields(doc)
		return
	}
	buildAppLockSetup(doc)
}

func resetSetupFields(doc js.Value) {
	for _, id := range []string{"cf-al-pass", "cf-al-confirm", "cf-al-hint"} {
		if e := doc.Call("getElementById", id); !e.IsNull() && !e.IsUndefined() {
			e.Set("value", "")
		}
	}
	if e := doc.Call("getElementById", "cf-al-mins"); !e.IsNull() && !e.IsUndefined() {
		e.Set("value", "0")
	}
	if e := doc.Call("getElementById", "cf-al-err"); !e.IsNull() && !e.IsUndefined() {
		e.Set("textContent", "")
	}
	if e := doc.Call("getElementById", "cf-al-pass"); !e.IsNull() && !e.IsUndefined() {
		e.Call("focus")
	}
}

func buildAppLockSetup(doc js.Value) {
	ov := doc.Call("createElement", "div")
	ov.Set("id", appLockSetupID)
	ov.Get("style").Set("cssText", "position:fixed;inset:0;z-index:1001;display:grid;place-items:center;background:rgba(0,0,0,0.6);")

	const inputStyle = "width:100%;box-sizing:border-box;padding:0.55rem 0.7rem;background:var(--bg-elev,#1a1a1d);border:1px solid var(--border,#2a2a2c);border-radius:8px;color:inherit;font:inherit;"
	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "display:flex;flex-direction:column;gap:0.7rem;width:min(90vw,340px);padding:1.2rem;background:var(--bg-elev,#1a1a1d);color:var(--text,#f4f4f5);border:1px solid var(--border,#2a2a2c);border-radius:10px;box-shadow:0 12px 40px rgba(0,0,0,0.5);")
	card.Set("innerHTML",
		`<div style="font-size:1.05rem;font-weight:600;">`+escT("applock.setTitle")+`</div>`+
			`<input id="cf-al-pass" type="password" inputmode="numeric" aria-label="`+escT("applock.passcode")+`" placeholder="`+escT("applock.passcode")+`" style="`+inputStyle+`">`+
			`<input id="cf-al-confirm" type="password" inputmode="numeric" aria-label="`+escT("applock.confirm")+`" placeholder="`+escT("applock.confirm")+`" style="`+inputStyle+`">`+
			`<label style="font-size:0.82rem;opacity:0.85;display:flex;flex-direction:column;gap:0.3rem;">`+escT("applock.autoLabel")+
			`<input id="cf-al-mins" type="number" min="0" value="0" style="`+inputStyle+`"></label>`+
			`<input id="cf-al-hint" type="text" aria-label="`+escT("applock.hintLabel")+`" placeholder="`+escT("applock.hintPlaceholder")+`" style="`+inputStyle+`">`+
			`<div id="cf-al-err" style="color:var(--danger,#d8716f);font-size:0.82rem;min-height:1em;"></div>`+
			`<div style="display:flex;gap:0.5rem;justify-content:flex-end;">`+
			`<button id="cf-al-cancel" type="button" style="padding:0.5rem 0.9rem;border-radius:8px;border:1px solid var(--border,#2a2a2c);background:transparent;color:inherit;cursor:pointer;">`+escT("action.cancel")+`</button>`+
			`<button id="cf-al-ok" type="button" style="padding:0.5rem 0.9rem;border-radius:8px;border:0;background:var(--accent,#2e8b57);color:#052e13;font-weight:600;cursor:pointer;">`+escT("applock.enable")+`</button>`+
			`</div>`)
	ov.Call("appendChild", card)
	doc.Get("body").Call("appendChild", ov)

	get := func(id string) js.Value { return doc.Call("getElementById", id) }
	hide := func() { ov.Get("style").Set("display", "none") }
	submit := func() {
		pass := strings.TrimSpace(get("cf-al-pass").Get("value").String())
		conf := strings.TrimSpace(get("cf-al-confirm").Get("value").String())
		errEl := get("cf-al-err")
		switch {
		case pass == "":
			errEl.Set("textContent", uistate.T("applock.needPasscode"))
			return
		case applock.PasscodeStrength(pass) == applock.StrengthTooShort:
			// R30: don't let a too-short passcode be set (the meter's hard floor).
			errEl.Set("textContent", uistate.T("applock.tooShort"))
			return
		case applock.PasscodeStrength(pass) == applock.StrengthWeak:
			// C287: reject trivial / low-variety passcodes that meet the length
			// floor but are trivially guessable (all-same like "000000", or a
			// simple run like "123456"). PasscodeStrength demotes these to Weak.
			errEl.Set("textContent", uistate.T("applock.tooWeak"))
			return
		case pass != conf:
			errEl.Set("textContent", uistate.T("applock.mismatch"))
			return
		}
		hint := strings.TrimSpace(get("cf-al-hint").Get("value").String())
		if !applock.ValidHint(hint, pass) {
			errEl.Set("textContent", uistate.T("applock.hintLeaks"))
			return
		}
		mins := 0
		if v, err := strconv.Atoi(strings.TrimSpace(get("cf-al-mins").Get("value").String())); err == nil && v > 0 {
			mins = v
		}
		if enableAppLock(pass, mins, hint) {
			hide()
			if appLockOnDone != nil {
				appLockOnDone()
			}
		}
	}

	cancelCb := js.FuncOf(func(js.Value, []js.Value) any { hide(); return nil })
	get("cf-al-cancel").Call("addEventListener", "click", cancelCb)
	okCb := js.FuncOf(func(js.Value, []js.Value) any { submit(); return nil })
	get("cf-al-ok").Call("addEventListener", "click", okCb)
	enterCb := js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 && a[0].Get("key").String() == "Enter" {
			a[0].Call("preventDefault")
			submit()
		}
		return nil
	})
	get("cf-al-confirm").Call("addEventListener", "keydown", enterCb)

	resetSetupFields(doc)
}

// wireAutoLock arms an inactivity timer that re-shows the gate after the
// configured auto-lock window. Registered once at boot; activity (pointer/key/
// scroll) resets the idle clock, and a periodic check re-locks once idle passes
// the window. The listeners live for the app's lifetime (intentionally not
// released). No-op behaviorally until a lock with a positive window is set.
func wireAutoLock() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	now := func() float64 { return js.Global().Get("Date").Call("now").Float() }
	last := now()

	reset := js.FuncOf(func(js.Value, []js.Value) any { last = now(); return nil })
	for _, ev := range []string{"mousemove", "keydown", "click", "touchstart", "scroll"} {
		doc.Call("addEventListener", ev, reset)
	}

	check := js.FuncOf(func(js.Value, []js.Value) any {
		c := loadAppLock()
		if !c.Active() || c.AutoLockMinutes <= 0 {
			return nil
		}
		if !c.ShouldAutoLock(int((now() - last) / 60000)) {
			return nil
		}
		// Don't re-show if the gate is already up.
		gate := doc.Call("getElementById", appLockGateID)
		if !gate.IsNull() && !gate.IsUndefined() && gate.Get("style").Get("display").String() != "none" {
			return nil
		}
		showAppLockGate()
		last = now()
		return nil
	})
	js.Global().Call("setInterval", check, 30000)
}
