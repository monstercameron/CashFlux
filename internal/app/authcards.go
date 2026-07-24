// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/state"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// passwordAuthMode is which of the two password-fallback sub-forms
// PasswordAuthCard currently shows.
type passwordAuthMode string

const (
	passwordAuthLogin    passwordAuthMode = "login"
	passwordAuthRegister passwordAuthMode = "register"
)

// persistAuthSession is the single place every AuthService success handler in
// this file (Register, Login, RedeemPairingCode) lands a fresh
// TokenPairResponse: it keeps prefs.ServerToken/ServerMode/BackendDisabled in
// lockstep so prefs.BackendActive() (and the plain self-host token display
// elsewhere in Settings) stay correct, and — the part that must never be
// reinvented a third way — hands the pair to sync_client.go's
// storeAuthTokenPair, which persists the rotating refresh token, arms the
// proactive-refresh countdown, and cycles the backend watch onto the new
// session. Mirrors customsync.go's verifyCodeValue, plus the rotation
// bookkeeping that function's phone flow also needs.
func persistAuthSession(prefsAtom state.Atom[prefs.Prefs], serverURL string, pair backendrpc.TokenPairResponse) {
	p := prefsAtom.Get()
	p.ServerURL = serverURL
	p.ServerToken = strings.TrimSpace(pair.AccessToken)
	p.ServerMode = prefs.ServerSelfHosted
	p.BackendDisabled = false
	p = p.Normalize()
	prefsAtom.Set(p)
	uistate.PersistPrefs(p)
	storeAuthTokenPair(pair)
}

// PasswordAuthCard is the username/password escape hatch (TODOS.md C422
// client UI): phone sign-in (CustomSyncCard) is the primary path, so this
// renders collapsed as a single understated link — "Use a password
// instead" — and only expands into the full Register/Login form once
// clicked, keeping it a fallback rather than a co-equal option. Standalone
// component (its own hooks), composed into the /sync page like
// CustomSyncCard.
func PasswordAuthCard() uic.Node {
	prefsAtom := uistate.UsePrefs()
	noticeAtom := uistate.UseNotice()

	expanded := uic.UseState(false)
	mode := uic.UseState(string(passwordAuthLogin))
	username := uic.UseState("")
	password := uic.UseState("")
	submitting := uic.UseState(false)
	signedIn := uic.UseState(false)
	// recoveryCode holds Register's one-time RecoveryCode so it can be shown
	// exactly once; there is no way to fetch it again afterward (TODOS.md
	// C422 — the deliberate stand-in for email-based password reset).
	recoveryCode := uic.UseState("")
	// idempotencyKey is cleared whenever the username/password inputs change
	// (a different logical request) and minted fresh the first time a submit
	// with the current inputs is attempted, so retries of one submission
	// reuse it — matching the pattern in customsync.go's idempotencyKey.
	idempotencyKey := uic.UseState("")

	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }

	onToggleExpand := uic.UseEvent(func() { expanded.Set(!expanded.Get()) })
	onMode := func(v string) {
		mode.Set(v)
		recoveryCode.Set("")
	}
	onUsernameInput := uic.UseEvent(func(v string) {
		username.Set(v)
		idempotencyKey.Set("")
	})
	onPasswordInput := uic.UseEvent(func(v string) {
		password.Set(v)
		idempotencyKey.Set("")
	})

	registerErrorMessage := func(err error) string {
		switch err {
		case ErrUsernameRequired:
			return uistate.T("authCards.usernameRequired")
		case ErrPasswordRequired:
			return uistate.T("authCards.passwordRequired")
		case ErrPasswordTooShort:
			return uistate.T("authCards.passwordTooShort", authMinPasswordLength)
		default:
			return uistate.T("authCards.registerFailed")
		}
	}
	loginErrorMessage := func(err error) string {
		switch err {
		case ErrUsernameRequired:
			return uistate.T("authCards.usernameRequired")
		case ErrPasswordRequired:
			return uistate.T("authCards.passwordRequired")
		default:
			return uistate.T("authCards.loginFailed")
		}
	}

	onSubmit := uic.UseEvent(func() {
		pr := prefsAtom.Get().Normalize()
		u := normalizeUsername(username.Get())
		pw := password.Get()
		registering := passwordAuthMode(mode.Get()) == passwordAuthRegister

		if registering {
			if err := validateRegisterCredentials(u, pw); err != nil {
				notify(registerErrorMessage(err), true)
				return
			}
		} else if err := validateLoginCredentials(u, pw); err != nil {
			notify(loginErrorMessage(err), true)
			return
		}

		key := idempotencyKey.Get()
		if key == "" {
			key = newIdempotencyKey()
			idempotencyKey.Set(key)
		}

		submitting.Set(true)
		go func() {
			ctx := context.Background()
			// Register/Login are skip-listed server-side for both auth AND
			// entitlement (see authinterceptor_skip.go), so any non-empty
			// placeholder token satisfies syncbridge's client-side "a bearer
			// token is required" guard without being checked server-side —
			// the same pattern customsync.go's sendCode already uses. A
			// brand-new device that never went through phone verification
			// first has no token yet (pr.ServerToken is empty), so dialing
			// with it directly would fail this client-side guard before any
			// network request is made — before it even reaches the point of
			// registering or logging in at all.
			token := effectiveServerToken(pr)
			if token == "" {
				token = "refresh"
			}
			conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: token})
			if err != nil {
				submitting.Set(false)
				notify(uistate.T("customSync.connectFailed"), true)
				return
			}
			defer conn.Close()

			var out backendrpc.TokenPairResponse
			if registering {
				err = conn.Invoke(ctx, backendrpc.MethodAuthRegister, backendrpc.RegisterRequest{
					Username:    u,
					Password:    pw,
					DeviceLabel: customSyncDeviceLabel(),
				}, &out, backendrpc.JSONCallOptions()...)
			} else {
				err = conn.Invoke(ctx, backendrpc.MethodAuthLogin, backendrpc.LoginRequest{
					Username:       u,
					Password:       pw,
					DeviceLabel:    customSyncDeviceLabel(),
					IdempotencyKey: key,
				}, &out, backendrpc.JSONCallOptions()...)
			}
			submitting.Set(false)
			if err != nil {
				if registering {
					notify(customSyncErrorMessage(err, uistate.T("authCards.registerFailed")), true)
				} else {
					notify(customSyncErrorMessage(err, uistate.T("authCards.loginFailed")), true)
				}
				return
			}
			persistAuthSession(prefsAtom, pr.ServerURL, out)
			signedIn.Set(true)
			password.Set("")
			if registering {
				recoveryCode.Set(strings.TrimSpace(out.RecoveryCode))
				notify(uistate.T("authCards.registerSuccess"), false)
			} else {
				notify(uistate.T("authCards.loggedInAs", u), false)
			}
		}()
	})

	onStartOver := uic.UseEvent(func() {
		signedIn.Set(false)
		username.Set("")
		password.Set("")
		recoveryCode.Set("")
		idempotencyKey.Set("")
	})
	onDismissRecovery := uic.UseEvent(func() { recoveryCode.Set("") })

	registering := passwordAuthMode(mode.Get()) == passwordAuthRegister
	code := recoveryCode.Get()

	if !expanded.Get() {
		return Div(css.Class(tw.Mt1), Attr("data-testid", "password-auth-collapsed"),
			Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "password-auth-expand"),
				OnClick(onToggleExpand), uistate.T("authCards.usePasswordInstead")),
		)
	}

	return Div(css.Class("card", "password-auth-card", tw.Mt1, tw.Flex, tw.FlexCol, tw.Gap2), Attr("data-testid", "password-auth-card"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			ui.Icon(icon.Lock, css.Class(tw.W5, tw.H5, tw.ShrinkO, tw.TextDim)),
			Span(css.Class(tw.Text15, tw.FontSemibold), uistate.T("authCards.passwordTitle")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.passwordIntro")),

		If(signedIn.Get() && code == "", Fragment(
			P(css.Class(tw.Text12), Attr("data-testid", "password-auth-signedin"),
				IfElse(registering, Text(uistate.T("authCards.registerSuccess")), Text(uistate.T("authCards.loggedInAs", username.Get())))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "password-auth-different"),
				OnClick(onStartOver), uistate.T("authCards.useDifferentAccount")),
		)),

		// The one-time recovery code: shown exactly once, right after a
		// successful Register, then gone for good (TODOS.md C422).
		If(code != "", Div(css.Class("card", tw.Flex, tw.FlexCol, tw.Gap2), Attr("data-testid", "password-auth-recovery"),
			Span(css.Class(tw.Text13, tw.FontSemibold), uistate.T("authCards.recoveryTitle")),
			P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.recoveryIntro")),
			Div(css.Class("set-input", tw.Text15), Attr("data-testid", "password-auth-recovery-code"), Text(code)),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "password-auth-recovery-dismiss"),
				OnClick(onDismissRecovery), uistate.T("authCards.recoveryDismiss")),
		)),

		If(!signedIn.Get() && code == "", Fragment(
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("authCards.modeGroupLabel"),
				Options: []ui.SegOption{
					{Value: string(passwordAuthLogin), Label: uistate.T("authCards.modeLogin")},
					{Value: string(passwordAuthRegister), Label: uistate.T("authCards.modeRegister")},
				},
				Selected: mode.Get(),
				OnSelect: onMode,
			}),
			Input(css.Class("set-input"), Type("text"), Attr("autocomplete", "username"),
				Attr("aria-label", uistate.T("authCards.usernameLabel")), Attr("data-testid", "password-auth-username"),
				Placeholder(uistate.T("authCards.usernamePlaceholder")), Value(username.Get()), OnInput(onUsernameInput)),
			Input(css.Class("set-input"), Type("password"),
				Attr("autocomplete", IfElseValue(registering, "new-password", "current-password")),
				Attr("aria-label", uistate.T("authCards.passwordLabel")), Attr("data-testid", "password-auth-password"),
				Placeholder(IfElseValue(registering, uistate.T("authCards.passwordPlaceholderRegister"), uistate.T("authCards.passwordPlaceholderLogin"))),
				Value(password.Get()), OnInput(onPasswordInput)),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "password-auth-submit"),
				DisabledIf(submitting.Get()), OnClick(onSubmit),
				passwordAuthSubmitLabel(registering, submitting.Get())),
		)),
	)
}

// passwordAuthSubmitLabel picks the submit button's label/busy text for the
// current mode and in-flight state.
func passwordAuthSubmitLabel(registering, submitting bool) string {
	if registering {
		if submitting {
			return uistate.T("authCards.registering")
		}
		return uistate.T("authCards.registerSubmit")
	}
	if submitting {
		return uistate.T("authCards.loggingIn")
	}
	return uistate.T("authCards.loginSubmit")
}

// IfElseValue returns a when cond is true, otherwise b — a plain-value
// counterpart to the framework's IfElse (which builds nodes), used here to
// pick between two string attribute values.
func IfElseValue[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// DeviceLinkCard redeems a short-lived pairing code minted from the portal's
// Settings → Devices "Link a new device" flow (TODOS.md C421 client UI).
// This ONLY ever resolves an existing account — presented plainly as
// "already have an account? link this device," never alongside
// CustomSyncCard's brand-new-user phone flow as an equal alternative.
// Standalone component (its own hooks), composed into the /sync page.
func DeviceLinkCard() uic.Node {
	prefsAtom := uistate.UsePrefs()
	noticeAtom := uistate.UseNotice()

	// Collapsed by default, same reasoning as PasswordAuthCard: linking an
	// EXISTING device is a returning-user action, not something every
	// first-time visitor needs in front of them alongside CustomSyncCard's
	// brand-new-account phone flow.
	expanded := uic.UseState(false)
	code := uic.UseState("")
	submitting := uic.UseState(false)
	linked := uic.UseState(false)
	idempotencyKey := uic.UseState("")

	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }

	onToggleExpand := uic.UseEvent(func() { expanded.Set(!expanded.Get()) })

	onCodeInput := uic.UseEvent(func(v string) {
		code.Set(v)
		idempotencyKey.Set("")
	})

	onSubmit := uic.UseEvent(func() {
		pr := prefsAtom.Get().Normalize()
		normalized, err := normalizePairingCode(code.Get())
		if err != nil {
			if err == ErrPairingCodeMissing {
				notify(uistate.T("authCards.pairingCodeRequired"), true)
			} else {
				notify(uistate.T("authCards.pairingCodeInvalid"), true)
			}
			return
		}

		key := idempotencyKey.Get()
		if key == "" {
			key = newIdempotencyKey()
			idempotencyKey.Set(key)
		}

		submitting.Set(true)
		go func() {
			ctx := context.Background()
			// RedeemPairingCode is skip-listed server-side the same as
			// Register/Login (see this file's PasswordAuthCard.onSubmit for
			// the full rationale) — a brand-new device linking via a
			// portal-minted code has no session token yet either.
			token := effectiveServerToken(pr)
			if token == "" {
				token = "refresh"
			}
			conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: token})
			if err != nil {
				submitting.Set(false)
				notify(uistate.T("customSync.connectFailed"), true)
				return
			}
			defer conn.Close()
			var out backendrpc.TokenPairResponse
			err = conn.Invoke(ctx, backendrpc.MethodAuthRedeemPairingCode, backendrpc.RedeemPairingCodeRequest{
				PairingCode:    normalized,
				DeviceLabel:    customSyncDeviceLabel(),
				IdempotencyKey: key,
			}, &out, backendrpc.JSONCallOptions()...)
			submitting.Set(false)
			if err != nil {
				notify(customSyncErrorMessage(err, uistate.T("authCards.linkFailed")), true)
				return
			}
			persistAuthSession(prefsAtom, pr.ServerURL, out)
			linked.Set(true)
			notify(uistate.T("authCards.deviceLinked"), false)
		}()
	})

	onStartOver := uic.UseEvent(func() {
		linked.Set(false)
		code.Set("")
		idempotencyKey.Set("")
	})

	if !expanded.Get() {
		return Div(css.Class(tw.Mt1), Attr("data-testid", "device-link-collapsed"),
			Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "device-link-expand"),
				OnClick(onToggleExpand), uistate.T("authCards.haveAnAccount")),
		)
	}

	return Div(css.Class("card", "device-link-card", tw.Mt1, tw.Flex, tw.FlexCol, tw.Gap2), Attr("data-testid", "device-link-card"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			ui.Icon(icon.Repeat, css.Class(tw.W5, tw.H5, tw.ShrinkO, tw.TextDim)),
			Span(css.Class(tw.Text15, tw.FontSemibold), uistate.T("authCards.deviceLinkTitle")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.deviceLinkIntro")),

		If(linked.Get(), Fragment(
			P(css.Class(tw.Text12), Attr("data-testid", "device-link-linked"), uistate.T("authCards.deviceLinked")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "device-link-different"),
				OnClick(onStartOver), uistate.T("authCards.linkAnotherDevice")),
		)),

		If(!linked.Get(), Fragment(
			Input(css.Class("set-input"), Type("text"), Attr("inputmode", "numeric"), Attr("autocomplete", "one-time-code"),
				Attr("aria-label", uistate.T("authCards.pairingCodeLabel")), Attr("data-testid", "device-link-code"),
				Placeholder(uistate.T("authCards.pairingCodePlaceholder")), Value(code.Get()), OnInput(onCodeInput)),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "device-link-submit"),
				DisabledIf(submitting.Get()), OnClick(onSubmit),
				IfElse(submitting.Get(), Text(uistate.T("authCards.linking")), Text(uistate.T("authCards.linkDevice")))),
		)),
	)
}
