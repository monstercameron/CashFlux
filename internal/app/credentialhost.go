// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// CredentialVaultHost is mounted at the shell root (beside AccountEditHost). It reads
// the account-credentials atom and renders the encrypted credential vault modal for
// that account. Lives in the app package because credential encryption
// (encryptDataset/decryptDataset + activePasscode) lives here, not in screens.
//
// See credvault.go for the SECURITY REVIEW notice — this is a first pass.
func CredentialVaultHost() uic.Node {
	idAtom := uistate.UseAccountCredentials()
	// Capture the settings atom in render (never call a hook inside a handler — that
	// panics the wasm app). Hooks below are ALL declared unconditionally, before any
	// branch, so the host's hook count is stable across renders regardless of which
	// state (closed / gated / open) it is in.
	closeModal := func() { uistate.SetAccountCredentials("") }
	gateCancel := uic.UseEvent(Prevent(closeModal))
	gateOpenSettings := uic.UseEvent(Prevent(func() {
		closeModal()
		uistate.OpenGlobalSettingsAt("advanced") // the Advanced tab hosts the app lock
	}))

	accountID := idAtom.Get()
	if accountID == "" {
		return Fragment()
	}

	// Gate: no passcode / locked ⇒ can't encrypt, so don't offer storage.
	if !credVaultAvailable() {
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title: uistate.T("creds.title"), Width: uiw.FlipSmallW, Height: uiw.FlipSmallH, NoFooter: true, OnClose: closeModal,
			Back: Div(css.Class("acct-edit-form"),
				credWarningBanner(),
				P(css.Class("muted"), uistate.T("creds.needPasscode")),
				Div(css.Class("modal-sticky-foot"),
					Button(css.Class("btn"), Type("button"), OnClick(gateCancel), uistate.T("action.cancel")),
					Button(css.Class("btn btn-primary"), Type("button"), OnClick(gateOpenSettings), uistate.T("creds.setPasscode")),
				),
			),
		})
	}

	acctName := ""
	if app := appstate.Default; app != nil {
		for _, ac := range app.Accounts() {
			if ac.ID == accountID {
				acctName = ac.Name
				break
			}
		}
	}
	title := uistate.T("creds.title")
	if acctName != "" {
		title = uistate.T("creds.titleFor", acctName)
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title: title, Width: uiw.FlipMediumW, Height: uiw.FlipMediumH, NoFooter: true, OnClose: closeModal,
		Back: uic.CreateElement(credentialForm, credentialFormProps{AccountID: accountID, OnDone: closeModal}),
	})
}

// credWarningBanner is the prominent "not yet security-reviewed" notice shown at the
// top of every credential modal.
func credWarningBanner() uic.Node {
	return Div(
		Style(map[string]string{
			"background": "#2b2410", "border": "1px solid #6b5a1e", "color": "#e6c964",
			"borderRadius": "6px", "padding": "0.55rem 0.7rem", "fontSize": "0.78rem", "lineHeight": "1.35",
		}),
		Attr("data-testid", "cred-security-warning"), Attr("role", "note"),
		Span(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap15), uiw.Icon(icon.AlertTriangle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Strong(uistate.T("creds.warnTitle"))),
		P(Style(map[string]string{"margin": "0.3rem 0 0"}), uistate.T("creds.warnBody")),
	)
}

// credField wraps a control in the shared .labeled-field shell (label over control).
func credField(label string, control uic.Node) uic.Node {
	return Label(css.Class("labeled-field"),
		Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
		Span(css.Class("t-caption", tw.TextDim), label),
		control,
	)
}

type credentialFormProps struct {
	AccountID string
	OnDone    func()
}

// credentialForm loads the account's decrypted credential on mount and lets the user
// edit username / URL / notes and set-or-replace the password. The stored PASSWORD is
// never placed in the DOM: it isn't loaded into the input, there is no reveal, and it
// is only ever retrieved through the "Copy password" button, which re-auths against
// the app passcode and writes it straight to the clipboard (never rendered).
func credentialForm(props credentialFormProps) uic.Node {
	loading := uic.UseState(true)
	errMsg := uic.UseState("")
	existing := uic.UseState(false)
	hasPass := uic.UseState(false)
	userS := uic.UseState("")
	newPassS := uic.UseState("") // a NEW/changed password the user types; empty = keep existing
	urlS := uic.UseState("")
	notesS := uic.UseState("")
	// storedPw holds the decrypted existing password in wasm memory only (never bound
	// to a DOM node / Value()), so "Copy password" can write it synchronously inside
	// the re-auth click gesture without a fresh async decrypt.
	storedPw := uic.UseState("")

	onUser := uic.UseEvent(func(v string) { userS.Set(v) })
	onPass := uic.UseEvent(func(v string) { newPassS.Set(v) })
	onURL := uic.UseEvent(func(v string) { urlS.Set(v) })
	onNotes := uic.UseEvent(func(v string) { notesS.Set(v) })
	cancel := uic.UseEvent(Prevent(func() { props.OnDone() }))

	save := uic.UseEvent(Prevent(func() {
		loadCredVault(func(v credVault, err error) {
			if err != nil {
				errMsg.Set(err.Error())
				return
			}
			if v == nil {
				v = credVault{}
			}
			c := Credential{
				Username: strings.TrimSpace(userS.Get()),
				LoginURL: strings.TrimSpace(urlS.Get()), Notes: notesS.Get(),
				UpdatedAt: time.Now().Format(time.RFC3339),
			}
			// Keep the existing password unless the user typed a replacement — the stored
			// password never has to surface in the UI to be preserved.
			if typed := newPassS.Get(); typed != "" {
				c.Password = typed
			} else if old, ok := v[props.AccountID]; ok {
				c.Password = old.Password
			}
			if c.IsEmpty() {
				delete(v, props.AccountID)
			} else {
				v[props.AccountID] = c
			}
			saveCredVault(v, func(err error) {
				if err != nil {
					errMsg.Set(err.Error())
					return
				}
				uistate.PostNotice(uistate.T("creds.saved"), false)
				props.OnDone()
			})
		})
	}))
	removeCreds := uic.UseEvent(Prevent(func() {
		uistate.ConfirmModal(uistate.T("creds.removeConfirm"), true, func(ok bool) {
			if !ok {
				return
			}
			loadCredVault(func(v credVault, err error) {
				if err != nil {
					errMsg.Set(err.Error())
					return
				}
				delete(v, props.AccountID)
				saveCredVault(v, func(err error) {
					if err != nil {
						errMsg.Set(err.Error())
						return
					}
					uistate.PostNotice(uistate.T("creds.removed"), false)
					props.OnDone()
				})
			})
		})
	}))
	// Copy password: re-authenticate with the app passcode, then write the stored
	// password straight to the clipboard. Never shown, never put in the DOM.
	copyPass := uic.UseEvent(Prevent(func() {
		pw := storedPw.Get()
		if pw == "" {
			uistate.PostNotice(uistate.T("creds.noPassword"), true)
			return
		}
		promptReauth(func() { clipboardWriteSecret(pw) }) // write happens inside the re-auth OK gesture
	}))

	// Load the decrypted credential once on mount (re-runs if the account changes).
	uic.UseEffect(func() func() {
		loadCredVault(func(v credVault, err error) {
			if err != nil {
				errMsg.Set(err.Error())
				loading.Set(false)
				return
			}
			if c, ok := v[props.AccountID]; ok {
				userS.Set(c.Username)
				urlS.Set(c.LoginURL)
				notesS.Set(c.Notes)
				storedPw.Set(c.Password) // memory only, never rendered
				hasPass.Set(c.Password != "")
				existing.Set(true)
			}
			loading.Set(false)
		})
		return nil
	}, props.AccountID)

	if loading.Get() {
		return Div(css.Class("acct-edit-form"), credWarningBanner(), P(css.Class("muted"), uistate.T("common.loading")))
	}

	passPh := uistate.T("creds.passwordPhNew")
	if hasPass.Get() {
		passPh = uistate.T("creds.passwordPhKeep")
	}
	loginURL := safeLoginURL(urlS.Get())

	return Div(css.Class("acct-edit-form"),
		credWarningBanner(),
		credField(uistate.T("creds.username"),
			Input(css.Class("field"), Attr("autofocus", ""), Attr("data-testid", "cred-username"), Type("text"),
				Attr("autocomplete", "off"), Attr("spellcheck", "false"),
				Placeholder(uistate.T("creds.usernamePh")), Value(userS.Get()), OnInput(onUser))),
		credField(uistate.T("creds.password"),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				// This input is ONLY for setting/replacing the password; it is never
				// pre-filled with the stored value (which stays out of the DOM).
				Input(css.Class("field"), Attr("data-testid", "cred-password"), Type("password"),
					Attr("autocomplete", "new-password"), Attr("spellcheck", "false"), Style(map[string]string{"flex": "1"}),
					Placeholder(passPh), Value(newPassS.Get()), OnInput(onPass)),
				If(hasPass.Get(), Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "cred-copy"), Title(uistate.T("creds.copyPasswordTitle")), OnClick(copyPass),
					uiw.Icon(icon.Copy, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("creds.copyPassword")))))),
		credField(uistate.T("creds.loginUrl"),
			Input(css.Class("field"), Attr("data-testid", "cred-url"), Type("url"),
				Placeholder(uistate.T("creds.loginUrlPh")), Value(urlS.Get()), OnInput(onURL))),
		// Home / login-page quick link, shown only when a valid http(s) URL is set.
		If(loginURL != "", P(Style(map[string]string{"margin": "-0.35rem 0 0"}),
			A(css.Class("btn-link"), Attr("data-testid", "cred-open-login"),
				Href(loginURL), Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
				Text(uistate.T("creds.openLogin")+" ↗")))),
		credField(uistate.T("creds.notes"),
			uiw.TextAreaInput(uiw.TextFieldProps{Value: notesS.Get(), Placeholder: uistate.T("creds.notesPh"),
				AriaLabel: uistate.T("creds.notes"), OnInput: onNotes})),
		If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
		Div(css.Class("modal-sticky-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			If(existing.Get(), Button(css.Class("btn-del"), Type("button"), Attr("data-testid", "cred-remove"), OnClick(removeCreds), uistate.T("creds.remove"))),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "cred-save"), OnClick(save), uistate.T("action.save")),
		),
	)
}

// safeLoginURL returns a clickable http(s) URL for the login quick link, or "" if the
// value is empty or a non-web scheme (javascript:/data: etc.). A scheme-less value is
// treated as https.
func safeLoginURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return u
	}
	if strings.ContainsRune(u, ':') { // some other scheme (javascript:, data:, mailto:, …) — don't link
		return ""
	}
	return "https://" + u
}

// clipboardWriteSecret writes a secret straight to the system clipboard via the
// Clipboard API. The secret passes from wasm memory to the clipboard without ever
// touching a DOM node. Must be called inside a user-gesture (a click) — the re-auth
// OK handler is one.
func clipboardWriteSecret(secret string) {
	if secret == "" {
		return
	}
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return
	}
	clip := nav.Get("clipboard")
	if !clip.Truthy() {
		uistate.PostNotice(uistate.T("creds.clipboardFail"), true)
		return
	}
	promise := clip.Call("writeText", secret)
	var onOK, onErr js.Func
	onOK = js.FuncOf(func(js.Value, []js.Value) any {
		uistate.PostNotice(uistate.T("creds.copied"), false)
		onOK.Release()
		onErr.Release()
		return nil
	})
	onErr = js.FuncOf(func(js.Value, []js.Value) any {
		uistate.PostNotice(uistate.T("creds.clipboardFail"), true)
		onOK.Release()
		onErr.Release()
		return nil
	})
	promise.Call("then", onOK).Call("catch", onErr)
}

// promptReauth shows a small overlay asking the user to re-enter the app passcode and,
// on a correct entry, synchronously calls onOK (so a clipboard write inside onOK stays
// within the click gesture). It is a raw-DOM overlay (like the app-lock setup) so it
// can layer above the FlipPanel modal. onOK is NOT called on cancel / wrong passcode.
func promptReauth(onOK func()) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	inputStyle := "width:100%;box-sizing:border-box;padding:0.55rem 0.7rem;text-align:center;letter-spacing:0.15em;background:#121214;border:1px solid #2a2a2c;border-radius:8px;color:#f4f4f5;font:inherit;"
	ov := doc.Call("createElement", "div")
	ov.Get("style").Set("cssText", "position:fixed;inset:0;z-index:1200;display:grid;place-items:center;background:rgba(4,4,6,.62);backdrop-filter:blur(3px);")
	ov.Call("setAttribute", "role", "dialog")
	ov.Call("setAttribute", "aria-modal", "true")
	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "display:flex;flex-direction:column;gap:0.7rem;width:min(90vw,320px);padding:1.2rem;background:#1a1a1d;color:#f4f4f5;border:1px solid #2a2a2c;border-radius:10px;box-shadow:0 12px 40px rgba(0,0,0,0.55);")
	card.Set("innerHTML",
		`<div style="font-size:1rem;font-weight:600;">`+htmlEscaper.Replace(uistate.T("creds.reauthTitle"))+`</div>`+
			`<input id="cf-cred-reauth" type="password" inputmode="numeric" autocomplete="off" aria-label="`+htmlEscaper.Replace(uistate.T("applock.passcode"))+`" placeholder="`+htmlEscaper.Replace(uistate.T("applock.passcode"))+`" style="`+inputStyle+`">`+
			`<div id="cf-cred-reauth-err" style="color:#d8716f;font-size:0.82rem;min-height:1em;"></div>`+
			`<div style="display:flex;gap:0.5rem;justify-content:flex-end;">`+
			`<button id="cf-cred-reauth-cancel" type="button" style="padding:0.5rem 0.9rem;border-radius:8px;border:1px solid #2a2a2c;background:transparent;color:inherit;cursor:pointer;">`+htmlEscaper.Replace(uistate.T("action.cancel"))+`</button>`+
			`<button id="cf-cred-reauth-ok" type="button" style="padding:0.5rem 0.9rem;border-radius:8px;border:0;background:var(--accent,#2e8b57);color:#052e13;font-weight:600;cursor:pointer;">`+htmlEscaper.Replace(uistate.T("creds.copyConfirm"))+`</button>`+
			`</div>`)
	ov.Call("appendChild", card)
	doc.Get("body").Call("appendChild", ov)

	get := func(id string) js.Value { return doc.Call("getElementById", id) }
	var okCb, cancelCb, keyCb js.Func
	remove := func() {
		ov.Call("remove")
		okCb.Release()
		cancelCb.Release()
		keyCb.Release()
	}
	submit := func() {
		entered := strings.TrimSpace(get("cf-cred-reauth").Get("value").String())
		if !loadAppLock().Verify(entered) {
			get("cf-cred-reauth-err").Set("textContent", uistate.T("creds.reauthWrong"))
			return
		}
		remove()
		onOK() // in-gesture: a clipboard write here keeps the user activation
	}
	okCb = js.FuncOf(func(js.Value, []js.Value) any { submit(); return nil })
	cancelCb = js.FuncOf(func(js.Value, []js.Value) any { remove(); return nil })
	keyCb = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Get("key").String() == "Enter" {
			submit()
		}
		return nil
	})
	get("cf-cred-reauth-ok").Call("addEventListener", "click", okCb)
	get("cf-cred-reauth-cancel").Call("addEventListener", "click", cancelCb)
	get("cf-cred-reauth").Call("addEventListener", "keydown", keyCb)
	get("cf-cred-reauth").Call("focus")
}
