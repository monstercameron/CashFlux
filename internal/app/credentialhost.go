// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/interop"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// CredentialVaultHost is mounted at the shell root, beside AccountEditHost. It reads the
// account-credentials atom and renders the encrypted credential vault for that account. It
// lives in the app package because the encryption it relies on (encryptDataset, loadCredVault
// and saveCredVault) lives there, not in screens. The vault has not had a security review yet,
// so every credential modal shows a warning (see credWarningBanner).
func CredentialVaultHost() uic.Node {
	idAtom := uistate.UseAccountCredentials()
	// Every hook is declared before the first branch, so the hook count is the same in every
	// state (closed, gated, open). Hooks must never run inside an event handler, which panics
	// the wasm app.
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

	// Without an unlocked passcode nothing can be encrypted, so show a gate that sends the
	// person to set one instead of the vault form.
	if !credVaultAvailable() {
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title: uistate.T("creds.title"), Width: uiw.FlipSmallW, Height: uiw.FlipSmallH, NoFooter: true, FlushBody: true, OnClose: closeModal,
			Back: Div(css.Class("acct-edit-form"),
				Div(css.Class("modal-scroll"), credWarningBanner(), P(css.Class("muted"), uistate.T("creds.needPasscode"))),
				Div(css.Class("modal-foot"),
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
		Title: title, Width: uiw.FlipMediumW, Height: uiw.FlipMediumH, NoFooter: true, FlushBody: true, OnClose: closeModal,
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
	newPassS := uic.UseState("") // the password the person types to set or replace it; empty keeps the stored one
	urlS := uic.UseState("")
	notesS := uic.UseState("")
	// storedPw holds the decrypted password in wasm memory only, never in a DOM node. Copy
	// password writes it to the clipboard inside the passcode dialog's OK click, so it must
	// already be in memory at that point.
	storedPw := uic.UseState("")

	onUser := uic.UseEvent(func(v string) { userS.Set(v) })
	onPass := uic.UseEvent(func(v string) { newPassS.Set(v) })
	onURL := uic.UseEvent(func(v string) { urlS.Set(v) })
	onNotes := func(v string) { notesS.Set(v) }
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
		Div(css.Class("modal-scroll"),
			credWarningBanner(),
			credField(uistate.T("creds.username"),
				Input(css.Class("field"), Attr("autofocus", ""), Attr("data-testid", "cred-username"), Type("text"),
					Attr("autocomplete", "off"), Attr("spellcheck", "false"),
					Placeholder(uistate.T("creds.usernamePh")), OnInput(onUser), uiw.FieldValue(userS.Get()))),
			credField(uistate.T("creds.password"),
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
					// This input is ONLY for setting/replacing the password; it is never
					// pre-filled with the stored value (which stays out of the DOM).
					Input(css.Class("field"), Attr("data-testid", "cred-password"), Type("password"),
						Attr("autocomplete", "new-password"), Attr("spellcheck", "false"), Style(map[string]string{"flex": "1"}),
						Placeholder(passPh), OnInput(onPass), uiw.FieldValue(newPassS.Get())),
					If(hasPass.Get(), Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
						Attr("data-testid", "cred-copy"), Title(uistate.T("creds.copyPasswordTitle")), OnClick(copyPass),
						uiw.Icon(icon.Copy, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("creds.copyPassword")))))),
			credField(uistate.T("creds.loginUrl"),
				Input(css.Class("field"), Attr("data-testid", "cred-url"), Type("url"),
					Placeholder(uistate.T("creds.loginUrlPh")), OnInput(onURL), uiw.FieldValue(urlS.Get()))),
			// Quick link to the login page, shown only when the URL is safe to link to.
			If(loginURL != "", P(Style(map[string]string{"margin": "-0.35rem 0 0"}),
				A(css.Class("btn-link"), Attr("data-testid", "cred-open-login"),
					Href(loginURL), Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
					Text(uistate.T("creds.openLogin")+" ↗")))),
			credField(uistate.T("creds.notes"),
				uiw.TextAreaInput(uiw.TextFieldProps{Value: notesS.Get(), Placeholder: uistate.T("creds.notesPh"),
					AriaLabel: uistate.T("creds.notes"), OnInput: onNotes})),
			If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			If(existing.Get(), Button(css.Class("btn-del"), Type("button"), Attr("data-testid", "cred-remove"), OnClick(removeCreds), uistate.T("creds.remove"))),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "cred-save"), OnClick(save), uistate.T("action.save")),
		),
	)
}

// safeLoginURL returns the URL to link to, or "" when there is nothing safe to link. http and
// https URLs pass through, and a value with no scheme and no colon is treated as https. Any
// other value, such as javascript:, data: or a host with a port, gets no link.
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

// clipboardWriteSecret writes a secret straight to the system clipboard through GWC's clipboard
// wrapper. The secret passes from wasm memory to the clipboard without touching a DOM node. It
// must run inside a click: the passcode dialog's OK handler is one. The write is awaited in its
// own goroutine, so the click returns at once and the outcome arrives as a notice.
func clipboardWriteSecret(secret string) {
	clip, err := interop.NavigatorClipboard()
	if err != nil {
		uistate.PostNotice(uistate.T("creds.clipboardFail"), true)
		return
	}
	go func() {
		if err := clip.WriteText(context.Background(), secret); err != nil {
			uistate.PostNotice(uistate.T("creds.clipboardFail"), true)
			return
		}
		uistate.PostNotice(uistate.T("creds.copied"), false)
	}()
}

// promptReauth asks for the app passcode before a secret is copied, and calls onOK only after a
// correct entry. It uses the shared dialog, so the copy runs inside the click that confirms the
// passcode and keeps the user activation the clipboard needs. A wrong passcode is reported and
// the prompt is shown again, and cancelling leaves the secret where it is.
func promptReauth(onOK func()) {
	uistate.PromptSecret(uistate.T("creds.reauthTitle"), func(ok bool, entered string) {
		if !ok {
			return
		}
		if !loadAppLock().Verify(strings.TrimSpace(entered)) {
			uistate.PostNotice(uistate.T("creds.reauthWrong"), true)
			promptReauth(onOK)
			return
		}
		onOK()
	})
}
