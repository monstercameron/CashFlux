//go:build js && wasm

package app

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/lockquotes"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

const appLockGateID = "cf-applock-gate"

// appLockActive reports whether the unlock gate is currently covering the app.
// Global shortcut handlers consult it so a locked app can't be driven by keyboard.
func appLockActive() bool {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return false
	}
	g := doc.Call("getElementById", appLockGateID)
	return !g.IsNull() && !g.IsUndefined() && g.Get("style").Get("display").String() != "none"
}

// unlockGate dismisses the gate with a brief blur→fade so the app behind appears
// to sharpen into focus, then hides it and resets the styles for next time.
// Respects prefers-reduced-motion (then it just hides immediately).
func unlockGate(doc, gate js.Value) {
	st := gate.Get("style")
	reduce := false
	if m := js.Global().Call("matchMedia", "(prefers-reduced-motion: reduce)"); !m.IsNull() && !m.IsUndefined() {
		reduce = m.Get("matches").Bool()
	}
	if reduce {
		st.Set("display", "none")
		return
	}
	st.Set("transition", "opacity 0.35s ease, filter 0.35s ease, transform 0.35s ease")
	st.Set("opacity", "0")
	st.Set("filter", "blur(10px)")
	st.Set("transform", "scale(1.03)")
	var done js.Func
	done = js.FuncOf(func(js.Value, []js.Value) any {
		st.Set("display", "none")
		st.Set("opacity", "")
		st.Set("filter", "")
		st.Set("transform", "")
		st.Set("transition", "")
		done.Release()
		return nil
	})
	js.Global().Call("setTimeout", done, 380)
}

// maybeLockOnBoot shows the passcode gate at startup when the lock is enabled, so
// the app's content stays covered until the right passcode is entered. Called once
// from Run after mount.
func maybeLockOnBoot() {
	if loadAppLock().Active() {
		showAppLockGate()
	}
}

// showAppLockGate covers the whole app with a modal passcode gate (building it on
// first use). A correct passcode hides it. No-op when the lock isn't enabled.
func showAppLockGate() {
	if !loadAppLock().Active() {
		return
	}
	doc := js.Global().Get("document")
	if gate := doc.Call("getElementById", appLockGateID); !gate.IsNull() && !gate.IsUndefined() {
		gate.Get("style").Set("display", "grid")
		refreshLockMeta(doc)
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
	if msg := doc.Call("getElementById", "cf-applock-msg"); !msg.IsNull() && !msg.IsUndefined() {
		msg.Set("textContent", uistate.T("applock.unlockPrompt"))
		msg.Get("style").Set("color", "")
	}
	if hb := doc.Call("getElementById", "cf-lock-hint-btn"); !hb.IsNull() && !hb.IsUndefined() {
		hb.Get("style").Set("display", "none")
	}
}

func buildAppLockGate(doc js.Value) {
	gate := doc.Call("createElement", "div")
	gate.Set("id", appLockGateID)
	gate.Get("style").Set("cssText", "position:fixed;inset:0;z-index:1000;display:grid;place-items:center;background:var(--bg,#0e0e0f);")

	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "display:flex;flex-direction:column;gap:0.8rem;width:min(90vw,320px);text-align:center;color:var(--text,#f4f4f5);")
	card.Set("innerHTML", `<div id="cf-lock-greeting" style="font-size:1rem;opacity:0.85;"></div>`+
		`<div id="cf-lock-date" style="font-size:0.82rem;opacity:0.55;margin-bottom:0.3rem;"></div>`+
		`<div style="font-family:Fraunces,Georgia,serif;font-size:1.4rem;font-weight:600;">CashFlux</div>`+
		`<div id="cf-applock-msg" style="font-size:0.9rem;opacity:0.7;">`+htmlEscaper.Replace(uistate.T("applock.unlockPrompt"))+`</div>`)

	inp := doc.Call("createElement", "input")
	inp.Set("id", "cf-applock-input")
	inp.Set("type", "password")
	inp.Call("setAttribute", "inputmode", "numeric")
	inp.Call("setAttribute", "aria-label", "Passcode")
	inp.Get("style").Set("cssText", "width:100%;box-sizing:border-box;padding:0.6rem 0.8rem;text-align:center;font-size:1.1rem;letter-spacing:0.2em;background:var(--bg-elev,#1a1a1d);border:1px solid var(--border,#2a2a2c);border-radius:8px;color:inherit;outline:none;")
	card.Call("appendChild", inp)

	btn := doc.Call("createElement", "button")
	btn.Set("type", "button")
	btn.Set("textContent", uistate.T("applock.unlock"))
	btn.Get("style").Set("cssText", "padding:0.6rem 0.8rem;border-radius:8px;border:0;background:var(--accent,#2e8b57);color:#052e13;font-weight:600;cursor:pointer;")
	card.Call("appendChild", btn)

	forgot := doc.Call("createElement", "button")
	forgot.Set("type", "button")
	forgot.Set("textContent", uistate.T("applock.forgot"))
	forgot.Get("style").Set("cssText", "background:transparent;border:0;color:var(--text-faint,#888890);font-size:0.8rem;cursor:pointer;text-decoration:underline;")
	card.Call("appendChild", forgot)

	gate.Call("appendChild", card)
	doc.Get("body").Call("appendChild", gate)

	fails := 0
	hintBtnEl := func() js.Value { return doc.Call("getElementById", "cf-lock-hint-btn") }
	attempt := func() {
		if loadAppLock().Verify(inp.Get("value").String()) {
			unlockGate(doc, gate)
			fails = 0
			if hb := hintBtnEl(); !hb.IsNull() && !hb.IsUndefined() {
				hb.Get("style").Set("display", "none")
			}
			return
		}
		fails++
		if msg := doc.Call("getElementById", "cf-applock-msg"); !msg.IsNull() && !msg.IsUndefined() {
			msg.Set("textContent", uistate.T("applock.wrong"))
			msg.Get("style").Set("color", "var(--danger,#d8716f)")
		}
		// After a few misses, offer the hint — but only if one was set.
		if fails >= 3 && loadAppLock().Hint != "" {
			if hb := hintBtnEl(); !hb.IsNull() && !hb.IsUndefined() {
				hb.Get("style").Set("display", "block")
			}
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

	// Forgot passcode → wipe & reset. The gate is a soft, unencrypted deterrent,
	// so erasing local data is the only honest recovery from a lost passcode.
	forgotCb := js.FuncOf(func(js.Value, []js.Value) any {
		if confirmAction(uistate.T("applock.forgotConfirm")) {
			wipeAllLocalData()
			reloadPage()
		}
		return nil
	})
	forgot.Call("addEventListener", "click", forgotCb)

	// Hidden until a few failed attempts; reveals the (passcode-safe) hint.
	hintBtn := doc.Call("createElement", "button")
	hintBtn.Set("id", "cf-lock-hint-btn")
	hintBtn.Set("type", "button")
	hintBtn.Set("textContent", uistate.T("applock.showHint"))
	hintBtn.Get("style").Set("cssText", "display:none;background:transparent;border:0;color:var(--text-faint,#888890);font-size:0.8rem;cursor:pointer;text-decoration:underline;")
	card.Call("appendChild", hintBtn)
	hintCb := js.FuncOf(func(js.Value, []js.Value) any {
		if h := loadAppLock().Hint; h != "" {
			if msg := doc.Call("getElementById", "cf-applock-msg"); !msg.IsNull() && !msg.IsUndefined() {
				msg.Set("textContent", uistate.T("applock.hintPrefix")+h)
				msg.Get("style").Set("color", "")
			}
		}
		return nil
	})
	hintBtn.Call("addEventListener", "click", hintCb)

	quoteEl := doc.Call("createElement", "div")
	quoteEl.Set("id", "cf-lock-quote")
	quoteEl.Get("style").Set("cssText", "font-size:0.8rem;opacity:0.5;font-style:italic;margin-top:0.6rem;line-height:1.4;")
	card.Call("appendChild", quoteEl)

	// Focus trap: keep Tab within the gate's controls so a locked app's covered
	// background can't be reached by keyboard (mirrors the FlipPanel trap).
	focusables := []js.Value{inp, btn, forgot}
	trapCb := js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) == 0 || a[0].Get("key").String() != "Tab" {
			return nil
		}
		e := a[0]
		active := doc.Get("activeElement")
		first, last := focusables[0], focusables[len(focusables)-1]
		if e.Get("shiftKey").Bool() {
			if active.Equal(first) {
				e.Call("preventDefault")
				last.Call("focus")
			}
		} else if active.Equal(last) {
			e.Call("preventDefault")
			first.Call("focus")
		}
		return nil
	})
	gate.Call("addEventListener", "keydown", trapCb)

	refreshLockMeta(doc)
	resetAppLockInput(doc)
}

// refreshLockMeta fills the lock screen's safe, non-financial metadata — a
// time-of-day greeting, the date, and the day's rotating quote (deterministic via
// the day ordinal, no randomness) — honoring the Settings toggles. Recomputed each
// time the gate is shown.
func refreshLockMeta(doc js.Value) {
	cfg := loadAppLock()
	set := func(id, text string, show bool) {
		e := doc.Call("getElementById", id)
		if e.IsNull() || e.IsUndefined() {
			return
		}
		if show {
			e.Set("textContent", text)
			e.Get("style").Set("display", "block")
		} else {
			e.Get("style").Set("display", "none")
		}
	}
	now := js.Global().Get("Date").New()
	greeting := "Good evening"
	switch h := now.Call("getHours").Int(); {
	case h < 12:
		greeting = "Good morning"
	case h < 18:
		greeting = "Good afternoon"
	}
	showMeta := !cfg.HideMeta
	set("cf-lock-greeting", greeting, showMeta)
	set("cf-lock-date", now.Call("toLocaleDateString", js.Undefined(),
		map[string]any{"weekday": "long", "month": "long", "day": "numeric"}).String(), showMeta)
	dayOrdinal := int(js.Global().Get("Date").Call("now").Float() / 86400000)
	set("cf-lock-quote", lockquotes.ForIndex(dayOrdinal), !cfg.HideQuotes)
}

// wipeAllLocalData removes every cashflux:* localStorage key — the reset path for
// a forgotten passcode. The caller reloads afterward (boot then re-seeds a fresh
// sample, as on first run).
func wipeAllLocalData() {
	ls := js.Global().Get("localStorage")
	if ls.IsNull() || ls.IsUndefined() {
		return
	}
	var keys []string
	for i := 0; i < ls.Get("length").Int(); i++ {
		if k := ls.Call("key", i); !k.IsNull() && !k.IsUndefined() && strings.HasPrefix(k.String(), "cashflux:") {
			keys = append(keys, k.String())
		}
	}
	for _, k := range keys {
		ls.Call("removeItem", k)
	}
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

	const inputStyle = "width:100%;box-sizing:border-box;padding:0.55rem 0.7rem;background:var(--bg-elev,#1a1a1d);border:1px solid var(--border,#2a2a2c);border-radius:8px;color:inherit;font:inherit;outline:none;"
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
