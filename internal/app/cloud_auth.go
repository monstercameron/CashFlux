// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

type cloudAuthActions struct{ Google, GitHub, SignOut func() }

func useCloudAuth(atom state.Atom[prefs.Prefs], endpoint string, bump func()) cloudAuthActions {
	notice := uistate.UseNotice()
	notify := func(s string, bad bool) { notice.Set(notice.Get().With(s, bad)) }
	saveOAuth := func(token, csrf, userID string) {
		// The existing OAuth service enables its connection while adopting the
		// cookie session. Protection must already exist before it may run.
		if !datasetEncryptionActive() {
			return
		}
		persistOAuthCookieSession(atom, endpoint, token, csrf, userID, 0)
		p := atom.Get()
		p.BackendDisabled = cloudSetupPending()
		if p.BackendDisabled {
			// The shared OAuth adapter currently enables its watch. Put the
			// resumable setup back in charge before waiting for disk or reload.
			atom.Set(p)
			uistate.PersistPrefs(p)
			stopBackendWatch()
		}
		cloudCommitPreferences(atom, p, func(err error) {
			if err != nil {
				logSyncError("complete cloud OAuth sign-in failed", err)
				notify(uistate.T("cloud1.saveFailed"), true)
				return
			}
			reloadAfterIdentityChange("cloud OAuth sign-in")
		})
	}
	login := func(provider string) {
		if !datasetEncryptionActive() {
			return
		}
		startOAuthLogin(endpoint, provider, saveOAuth, func(msg string) { notify(msg, true) })
	}
	signOut := func() {
		uistate.ConfirmModalLabeled(uistate.T("cloud1.signOutConfirm"), uistate.T("settings.signOut"), false, func(ok bool) {
			if !ok {
				return
			}
			p := atom.Get()
			signOutBackendSession(endpoint, effectiveServerToken(p), cookieSessionCSRF(p), func() {
				clearAuthSession()
				lsRemove(authRefreshModeKey)
				lsRemove(authCSRFKey)
				p.ServerToken = ""
				p.ServerCSRF = ""
				p.BackendDisabled = true
				cloudCommitPreferences(atom, p, func(err error) {
					if err != nil {
						logSyncError("save cloud sign-out failed", err)
						notify(uistate.T("cloud1.saveFailed"), true)
						bump()
						return
					}
					reloadAfterIdentityChange("cloud sign-out")
				})
			})
		})
	}
	return cloudAuthActions{Google: func() { login("google") }, GitHub: func() { login("github") }, SignOut: signOut}
}

type cloudSignInProps struct {
	Connection cloudConnection
	Auth       cloudAuthActions
	Protected  bool
	OnProtect  func()
}

func cloudSignInView(p cloudSignInProps) uic.Node {
	c := p.Connection
	d := c.Discovery
	password := c.Phase == discoveryOK && d.CustomAuthEnabled
	activation := password && !d.RegistrationOpen
	oauth := c.Segment == "commercial" || (c.Phase == discoveryOK && len(d.AuthProviders) > 0)
	return Section(css.Class(tw.Flex, tw.FlexCol, tw.Gap3), Attr("data-testid", "cloud-setup-signin"),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.step.signin")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.signInHint")),
		uic.CreateElement(cloudConnectionFeedback, c.Phase),
		If(activation, uic.CreateElement(PendingDeviceCard, PendingDeviceCardProps{Primary: true})),
		If(password && !activation, uic.CreateElement(PasswordAuthCard, PasswordAuthCardProps{AllowRegistration: true, InitiallyExpanded: true})),
		If(oauth, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.signInGoogle"), TestID: "sync-oauth-google", OnClick: p.Auth.Google, Disabled: !p.Protected || !cloudProviderEnabled(c, "google")}),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.signInGitHub"), TestID: "sync-oauth-github", OnClick: p.Auth.GitHub, Disabled: !p.Protected || !cloudProviderEnabled(c, "github")}),
		)),
		If(oauth && !p.Protected, Fragment(
			P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.oauthProtection")),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.protectAction"), TestID: "cloud-oauth-protect", OnClick: p.OnProtect}),
		)),
		uic.CreateElement(cloudDisclosure, cloudDisclosureProps{Title: uistate.T("cloud1.moreSignIn"), TestID: "sync-advanced-token-toggle", Body: Fragment(
			If(activation, uic.CreateElement(PasswordAuthCard, PasswordAuthCardProps{AllowRegistration: false})),
			uic.CreateElement(DeviceLinkCard, DeviceLinkCardProps{}),
			uiw.Field(uiw.FieldProps{Label: uistate.T("cloud1.accessCode"), Hint: uistate.T("cloud1.accessCodeHint"), Control: func(a uiw.FieldA11y) uic.Node {
				return Input(css.Class("set-input"), Type("password"), ID(a.ID), Attr("aria-describedby", a.DescribedBy), Attr("data-testid", "sync-server-token"),
					OnInput(c.OnToken), uiw.FieldValue(c.Token))
			}}),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.useAccessCode"), TestID: "cloud-use-token", OnClick: c.SaveToken, Disabled: c.Phase != discoveryOK || strings.TrimSpace(c.Token) == ""}),
		)}),
	)
}

func cloudProviderEnabled(c cloudConnection, provider string) bool {
	if c.Segment == "commercial" {
		return true
	}
	for _, name := range c.Discovery.AuthProviders {
		if name == provider {
			return true
		}
	}
	return false
}

type cloudAccountProps struct {
	Account, Server string
	OnSignOut       func()
}

func cloudAccountView(p cloudAccountProps) uic.Node {
	account := p.Account
	if account == "" {
		account = uistate.T("cloud1.accountUnknown")
	}
	return Section(css.Class(cloudSectionRules()), H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.account")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.signedInAs", account, p.Server)),
		uic.CreateElement(SetPasswordCard),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.signOut"), TestID: "cloud-sign-out", OnClick: p.OnSignOut}),
	)
}
