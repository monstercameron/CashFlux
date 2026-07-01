// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
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
	settingsAtom := uistate.UseSettings()
	closeModal := func() { uistate.SetAccountCredentials("") }
	gateCancel := uic.UseEvent(Prevent(closeModal))
	gateOpenSettings := uic.UseEvent(Prevent(func() {
		closeModal()
		settingsAtom.Set(uistate.Global()) // open Settings to set up the app lock
	}))

	accountID := idAtom.Get()
	if accountID == "" {
		return Fragment()
	}

	// Gate: no passcode / locked ⇒ can't encrypt, so don't offer storage.
	if !credVaultAvailable() {
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title: uistate.T("creds.title"), Width: "420px", Height: "380px", NoFooter: true, OnClose: closeModal,
			Back: Div(css.Class("acct-edit-form"),
				credWarningBanner(),
				P(css.Class("muted"), uistate.T("creds.needPasscode")),
				Div(css.Class("acct-edit-actions"),
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
		Title: title, Width: "460px", Height: "560px", NoFooter: true, OnClose: closeModal,
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

// credentialForm loads the account's decrypted credential on mount, shows it masked,
// and re-encrypts the vault on save. All state is local and cleared on unmount; the
// plaintext only exists in memory while this modal is open.
func credentialForm(props credentialFormProps) uic.Node {
	loading := uic.UseState(true)
	errMsg := uic.UseState("")
	existing := uic.UseState(false)
	userS := uic.UseState("")
	passS := uic.UseState("")
	urlS := uic.UseState("")
	notesS := uic.UseState("")
	reveal := uic.UseState(false)

	onUser := uic.UseEvent(func(v string) { userS.Set(v) })
	onPass := uic.UseEvent(func(v string) { passS.Set(v) })
	onURL := uic.UseEvent(func(v string) { urlS.Set(v) })
	onNotes := uic.UseEvent(func(v string) { notesS.Set(v) })
	toggleReveal := uic.UseEvent(Prevent(func() { reveal.Set(!reveal.Get()) }))
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
				Username: strings.TrimSpace(userS.Get()), Password: passS.Get(),
				LoginURL: strings.TrimSpace(urlS.Get()), Notes: notesS.Get(),
				UpdatedAt: time.Now().Format(time.RFC3339),
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
				passS.Set(c.Password)
				urlS.Set(c.LoginURL)
				notesS.Set(c.Notes)
				existing.Set(true)
			}
			loading.Set(false)
		})
		return nil
	}, props.AccountID)

	if loading.Get() {
		return Div(css.Class("acct-edit-form"), credWarningBanner(), P(css.Class("muted"), uistate.T("common.loading")))
	}

	passType := "password"
	revealLabel := uistate.T("creds.reveal")
	if reveal.Get() {
		passType = "text"
		revealLabel = uistate.T("creds.hide")
	}

	return Div(css.Class("acct-edit-form"),
		credWarningBanner(),
		credField(uistate.T("creds.username"),
			Input(css.Class("field"), Attr("autofocus", ""), Attr("data-testid", "cred-username"), Type("text"),
				Attr("autocomplete", "off"), Attr("spellcheck", "false"),
				Placeholder(uistate.T("creds.usernamePh")), Value(userS.Get()), OnInput(onUser))),
		credField(uistate.T("creds.password"),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Input(css.Class("field"), Attr("data-testid", "cred-password"), Type(passType),
					Attr("autocomplete", "off"), Attr("spellcheck", "false"), Style(map[string]string{"flex": "1"}),
					Placeholder(uistate.T("creds.passwordPh")), Value(passS.Get()), OnInput(onPass)),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "cred-reveal"),
					Attr("aria-pressed", boolStr(reveal.Get())), Title(revealLabel), OnClick(toggleReveal),
					Text(revealLabel)))),
		credField(uistate.T("creds.loginUrl"),
			Input(css.Class("field"), Attr("data-testid", "cred-url"), Type("url"),
				Placeholder(uistate.T("creds.loginUrlPh")), Value(urlS.Get()), OnInput(onURL))),
		credField(uistate.T("creds.notes"),
			uiw.TextAreaInput(uiw.TextFieldProps{Value: notesS.Get(), Placeholder: uistate.T("creds.notesPh"),
				AriaLabel: uistate.T("creds.notes"), OnInput: onNotes})),
		If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
		Div(css.Class("acct-edit-actions"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			If(existing.Get(), Button(css.Class("btn-del"), Type("button"), Attr("data-testid", "cred-remove"), OnClick(removeCreds), uistate.T("creds.remove"))),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "cred-save"), OnClick(save), uistate.T("action.save")),
		),
	)
}

// boolStr renders a bool as the "true"/"false" string ARIA state attributes expect.
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
