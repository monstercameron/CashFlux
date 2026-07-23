// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"syscall/js"

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
	"google.golang.org/grpc/status"
)

// customSyncPhase tracks where the phone-based "Custom Sync" enrollment flow
// (TODOS.md C420, plus the client half of C419) currently is.
type customSyncPhase string

const (
	customSyncIdle      customSyncPhase = "idle"
	customSyncSending   customSyncPhase = "sending"
	customSyncCodeSent  customSyncPhase = "codeSent"
	customSyncVerifying customSyncPhase = "verifying"
	customSyncSignedIn  customSyncPhase = "signedIn"
)

// customSyncGate tracks the TODOS.md C431 pre-flight entitlement check: the
// card must not show the phone-number input (and must not let a phone
// verification SMS go out) for an account whose cloud entitlement is
// currently inactive.
type customSyncGate string

const (
	// customSyncGateChecking is showing while the GetEntitlement round trip is
	// in flight — the phone input is withheld until it settles.
	customSyncGateChecking customSyncGate = "checking"
	// customSyncGateOK covers both a confirmed-active entitlement AND the
	// "nothing to check yet" case (a brand-new phone sign-in with no existing
	// session token): GetEntitlement requires an authenticated caller, so a
	// device with no session yet has no entitlement to check against — it
	// fails open into the normal phone flow rather than being blocked by a
	// check it has no way to satisfy.
	customSyncGateOK customSyncGate = "ok"
	// customSyncGateBlocked means GetEntitlement came back Active=false: the
	// upgrade prompt replaces the phone field.
	customSyncGateBlocked customSyncGate = "blocked"
)

// customSyncSessionMarker is the non-secret placeholder persisted to
// prefs.ServerToken once a Custom Sync (phone) session is established. The
// REAL rotating bearer credential lives only in local storage — see
// effectiveServerToken/storeAuthTokenPair in sync_client.go — and is never
// copied into prefs (copying it here would go stale the moment it first
// rotates, since only local storage gets updated after that). This marker
// exists purely so prefs.Prefs.BackendActive() (which gates on ServerToken
// being non-empty) and degradeToLocalOnly (which clears ServerToken in
// lockstep with the rotated session dying) keep working: both already treat
// "prefs.ServerToken is set" as "a working backend credential exists," and
// this marker satisfies that without duplicating the credential itself.
const customSyncSessionMarker = "custom-sync-session"

// CustomSyncCard is the "Custom Sync" phone-number enrollment surface: enter a
// phone number, get a text with a code, and sign in — no server URL or bearer
// token to copy around by hand. It is a standalone component (its own hooks,
// not inlined into a loop) that the /sync page renders by composition.
//
// On success it persists the returned session the same way the rest of the
// sync surface persists a backend session — writing Prefs.ServerToken and
// switching Prefs.ServerMode to self-hosted — so sync_client.go's connection
// loop picks up the new session on its next connect, exactly like pasting in
// a token by hand already does.
func CustomSyncCard() uic.Node {
	prefsAtom := uistate.UsePrefs()
	noticeAtom := uistate.UseNotice()

	phase := uic.UseState(customSyncIdle)
	phoneInput := uic.UseState("")
	codeInput := uic.UseState("")
	// setupCodeInput is the optional single-use invite code some private/
	// embedded deployments require to create a brand-new account
	// (Config.SetupCode/TODOS.md C445). Blank and harmless on every ordinary
	// deployment — the server only checks it when it has one configured.
	setupCodeInput := uic.UseState("")
	// idempotencyKey is regenerated for every fresh send so a genuine second
	// attempt (new code) doesn't collide with a stale one, while retries of
	// verifying the SAME code (e.g. a flaky connection) keep reusing it so the
	// server returns the same token pair instead of minting a second session.
	idempotencyKey := uic.UseState("")

	// gate/gateReason implement the C431 pre-flight entitlement check: see
	// customSyncGate's doc comment for why "no session yet" fails open.
	gate := uic.UseState(customSyncGateOK)
	gateReason := uic.UseState("")

	uic.UseEffect(func() func() {
		checkPr := prefsAtom.Get().Normalize()
		token := effectiveServerToken(checkPr)
		if strings.TrimSpace(token) == "" {
			gate.Set(customSyncGateOK)
			gateReason.Set("")
			return nil
		}
		gate.Set(customSyncGateChecking)
		checkCloudEntitlement(checkPr.ServerURL, token, func(resp backendrpc.GetEntitlementResponse) {
			if resp.Active {
				gate.Set(customSyncGateOK)
				gateReason.Set("")
				return
			}
			gate.Set(customSyncGateBlocked)
			gateReason.Set(resp.Reason)
		}, func(string) {
			// Couldn't determine eligibility (network hiccup dialing the
			// backend, etc.) — fail open rather than blocking phone sign-in
			// on a check that itself couldn't complete.
			gate.Set(customSyncGateOK)
			gateReason.Set("")
		})
		return nil
	}, prefsAtom.Get().Normalize().ServerURL+"\x00"+effectiveServerToken(prefsAtom.Get().Normalize()))

	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }

	onOpenUpgrade := uic.UseEvent(func() { uistate.OpenGlobalSettingsAt("cloud") })

	sendCode := uic.UseEvent(func() {
		pr := prefsAtom.Get().Normalize()
		phone := strings.TrimSpace(phoneInput.Get())
		if phone == "" {
			notify(uistate.T("customSync.phoneRequired"), true)
			return
		}
		phase.Set(customSyncSending)
		go func() {
			ctx := context.Background()
			// RequestPhoneVerification is skip-listed server-side (see
			// authinterceptor_skip.go) for both auth AND entitlement, so any
			// non-empty placeholder token satisfies syncbridge's client-side
			// "a bearer token is required" guard without being checked
			// server-side — the same pattern doRefreshAccessToken already
			// uses in sync_client.go. A brand-new user has no token yet
			// (pr.ServerToken is empty), so dialing with it directly would
			// fail before any network request is made.
			token := effectiveServerToken(pr)
			if token == "" {
				token = "refresh"
			}
			conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: token})
			if err != nil {
				phase.Set(customSyncIdle)
				notify(uistate.T("customSync.connectFailed"), true)
				return
			}
			defer conn.Close()
			var out backendrpc.RequestPhoneVerificationResponse
			err = conn.Invoke(ctx, backendrpc.MethodAuthRequestPhoneVerification, backendrpc.RequestPhoneVerificationRequest{
				PhoneNumber: phone,
				DeviceLabel: customSyncDeviceLabel(),
				SetupCode:   strings.TrimSpace(setupCodeInput.Get()),
			}, &out, backendrpc.JSONCallOptions()...)
			if err != nil {
				phase.Set(customSyncIdle)
				notify(customSyncErrorMessage(err, uistate.T("customSync.sendFailed")), true)
				return
			}
			idempotencyKey.Set(newIdempotencyKey())
			phase.Set(customSyncCodeSent)
			notify(uistate.T("customSync.codeSent"), false)
			// WebOTP: if the browser supports it, the incoming SMS auto-fills the
			// code field (and auto-submits) without the user typing anything —
			// this is the point of C420, not a nice-to-have. Unsupported browsers
			// (listenForSMSOTP checks feature support first) fall through to the
			// plain autocomplete="one-time-code" input below.
			capturedPhone := phone
			capturedSetupCode := strings.TrimSpace(setupCodeInput.Get())
			listenForSMSOTP(func(code string) {
				codeInput.Set(code)
				verifyCodeValue(prefsAtom, noticeAtom, phase, idempotencyKey, capturedPhone, code, capturedSetupCode)
			})
		}()
	})

	verifyCode := uic.UseEvent(func() {
		phone := strings.TrimSpace(phoneInput.Get())
		code := strings.TrimSpace(codeInput.Get())
		if code == "" {
			notify(uistate.T("customSync.codeRequired"), true)
			return
		}
		verifyCodeValue(prefsAtom, noticeAtom, phase, idempotencyKey, phone, code, strings.TrimSpace(setupCodeInput.Get()))
	})

	onStartOver := uic.UseEvent(func() {
		phase.Set(customSyncIdle)
		codeInput.Set("")
	})

	onPhoneInput := uic.UseEvent(func(v string) { phoneInput.Set(v) })
	onCodeInput := uic.UseEvent(func(v string) { codeInput.Set(v) })
	onSetupCodeInput := uic.UseEvent(func(v string) { setupCodeInput.Set(v) })

	sending := phase.Get() == customSyncSending
	verifying := phase.Get() == customSyncVerifying
	signedIn := phase.Get() == customSyncSignedIn
	awaitingCode := phase.Get() == customSyncCodeSent || verifying

	return Div(css.Class("card", "custom-sync-card", tw.Mt1, tw.Flex, tw.FlexCol, tw.Gap2), Attr("data-testid", "custom-sync-card"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			ui.Icon(icon.Lock, css.Class(tw.W5, tw.H5, tw.ShrinkO, tw.TextDim)),
			Span(css.Class(tw.Text15, tw.FontSemibold), uistate.T("customSync.title")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("customSync.intro")),

		If(signedIn, Fragment(
			P(css.Class(tw.Text12), Attr("data-testid", "custom-sync-signedin"), uistate.T("customSync.signedInAs", phoneInput.Get())),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "custom-sync-use-different-phone"),
				OnClick(onStartOver), uistate.T("customSync.useDifferentPhone")),
		)),

		// C431 pre-flight: the phone field only ever renders once the
		// entitlement check has cleared. While it's in flight, or the account
		// is gated, the phone form (and the real SMS send it would trigger)
		// is withheld entirely.
		If(!signedIn && !awaitingCode && gate.Get() == customSyncGateChecking,
			P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "custom-sync-checking"), uistate.T("customSync.checkingEligibility")),
		),

		If(!signedIn && !awaitingCode && gate.Get() == customSyncGateBlocked, Fragment(
			P(css.Class(tw.Text12), Attr("data-testid", "custom-sync-gated"), customSyncGateMessage(gateReason.Get())),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "custom-sync-upgrade"),
				OnClick(onOpenUpgrade), uistate.T("customSync.upgradeCta")),
		)),

		If(!signedIn && !awaitingCode && gate.Get() == customSyncGateOK, Fragment(
			Input(css.Class("set-input"), Type("tel"), Attr("inputmode", "tel"), Attr("autocomplete", "tel"),
				Attr("aria-label", uistate.T("customSync.phoneLabel")), Attr("data-testid", "custom-sync-phone"),
				Placeholder(uistate.T("customSync.phonePlaceholder")), Value(phoneInput.Get()), OnInput(onPhoneInput)),
			// Optional single-use invite code (Config.SetupCode/TODOS.md C445) —
			// blank and harmless on every ordinary deployment; only a
			// private/embedded deployment gating new accounts actually checks it.
			Input(css.Class("set-input"), Type("text"), Attr("autocomplete", "off"),
				Attr("aria-label", uistate.T("customSync.setupCodeLabel")), Attr("data-testid", "custom-sync-setup-code"),
				Placeholder(uistate.T("customSync.setupCodePlaceholder")), Value(setupCodeInput.Get()), OnInput(onSetupCodeInput)),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "custom-sync-send"),
				DisabledIf(sending), OnClick(sendCode),
				IfElse(sending, Text(uistate.T("customSync.sending")), Text(uistate.T("customSync.sendCode")))),
		)),

		If(awaitingCode, Fragment(
			P(css.Class(tw.Text12), uistate.T("customSync.codeSentTo", phoneInput.Get())),
			Input(css.Class("set-input"), Type("text"), Attr("inputmode", "numeric"), Attr("autocomplete", "one-time-code"),
				Attr("aria-label", uistate.T("customSync.codeLabel")), Attr("data-testid", "custom-sync-code"),
				Placeholder(uistate.T("customSync.codePlaceholder")), Value(codeInput.Get()), OnInput(onCodeInput)),
			Div(css.Class(tw.Flex, tw.Gap2),
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "custom-sync-verify"),
					DisabledIf(verifying), OnClick(verifyCode),
					IfElse(verifying, Text(uistate.T("customSync.verifying")), Text(uistate.T("customSync.verifyCode")))),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "custom-sync-start-over"),
					OnClick(onStartOver), uistate.T("customSync.startOver")),
			),
		)),
	)
}

// verifyCodeValue runs VerifyPhoneCode and, on success, persists the returned
// session into prefs exactly like the hand-entered-token flow does
// (p.ServerToken / p.ServerMode), then restarts the backend sync watch so it
// picks up the new session immediately. It is a free function (not a closure
// captured per-render) so both the manual "Verify" click and the WebOTP
// auto-fill callback share one code path.
func verifyCodeValue(prefsAtom state.Atom[prefs.Prefs], noticeAtom state.Atom[uistate.Notice], phase uic.State[customSyncPhase], idempotencyKey uic.State[string], phone, code, setupCode string) {
	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }
	phase.Set(customSyncVerifying)
	go func() {
		pr := prefsAtom.Get().Normalize()
		ctx := context.Background()
		// VerifyPhoneCode is skip-listed server-side the same as
		// RequestPhoneVerification (see sendCode's comment above) — dial with
		// whatever session token exists, falling back to a non-empty
		// placeholder for a brand-new user with none yet.
		token := effectiveServerToken(pr)
		if token == "" {
			token = "refresh"
		}
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: token})
		if err != nil {
			phase.Set(customSyncCodeSent)
			notify(uistate.T("customSync.connectFailed"), true)
			return
		}
		defer conn.Close()
		key := idempotencyKey.Get()
		if key == "" {
			key = newIdempotencyKey()
			idempotencyKey.Set(key)
		}
		var out backendrpc.TokenPairResponse
		err = conn.Invoke(ctx, backendrpc.MethodAuthVerifyPhoneCode, backendrpc.VerifyPhoneCodeRequest{
			PhoneNumber:    phone,
			Code:           code,
			DeviceLabel:    customSyncDeviceLabel(),
			IdempotencyKey: key,
			SetupCode:      setupCode,
		}, &out, backendrpc.JSONCallOptions()...)
		if err != nil {
			phase.Set(customSyncCodeSent)
			notify(customSyncErrorMessage(err, uistate.T("customSync.verifyFailed")), true)
			return
		}
		p := prefsAtom.Get()
		p.ServerURL = pr.ServerURL
		// customSyncSessionMarker, not the raw access token — see its doc
		// comment. The real credential pair is persisted below via
		// storeAuthTokenPair, which also arms the proactive refresh timer
		// (C423) instead of the token silently going stale with no refresh
		// path, the way a bare `p.ServerToken = out.AccessToken` would.
		p.ServerToken = customSyncSessionMarker
		p.ServerMode = prefs.ServerSelfHosted
		p.BackendDisabled = false
		p = p.Normalize()
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
		storeAuthTokenPair(out)
		phase.Set(customSyncSignedIn)
		notify(uistate.T("customSync.signedIn"), false)
		restartBackendSync()
	}()
}

// customSyncGateMessage returns the plain-English upgrade-prompt copy for a
// GetEntitlement rejection reason (TODOS.md C431). The reason→key mapping
// itself is customSyncGateMessageKey (customsync_gate.go), kept in a
// no-build-tag file so it unit-tests on native Go.
func customSyncGateMessage(reason string) string {
	return uistate.T(customSyncGateMessageKey(reason))
}

// customSyncDeviceLabel returns a short, human-readable label for this
// browser/device — shown back to the user in the device list (ListDevices).
// It reads navigator.platform/userAgentData when available and falls back to
// a generic label off the browser, matching the js.Global() interop style
// backend.go already uses (appOrigin) rather than adding a new pattern.
func customSyncDeviceLabel() string {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return "This device"
	}
	if uaData := nav.Get("userAgentData"); uaData.Truthy() {
		if platform := uaData.Get("platform"); platform.Truthy() && strings.TrimSpace(platform.String()) != "" {
			return strings.TrimSpace(platform.String()) + " browser"
		}
	}
	if platform := nav.Get("platform"); platform.Truthy() && strings.TrimSpace(platform.String()) != "" {
		return strings.TrimSpace(platform.String()) + " browser"
	}
	return "This device"
}

// listenForSMSOTP wires the WebOTP API (navigator.credentials.get with an
// "otp" transport) so an incoming verification SMS auto-fills the code
// without the user typing it — the actual point of TODOS.md C420. It is a
// pure feature-detect no-op on browsers that don't support WebOTP (most
// desktop browsers, all non-Chromium mobile browsers): onCode is simply never
// called, and the plain autocomplete="one-time-code" input still works.
func listenForSMSOTP(onCode func(code string)) {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return
	}
	cred := nav.Get("credentials")
	if !cred.Truthy() || !cred.Get("get").Truthy() {
		return
	}
	otp := js.Global().Get("Object").New()
	otp.Set("transport", js.Global().Get("Array").New("sms"))
	opts := js.Global().Get("Object").New()
	opts.Set("otp", otp)
	var done, fail js.Func
	done = js.FuncOf(func(_ js.Value, args []js.Value) any {
		done.Release()
		fail.Release()
		if len(args) == 0 {
			return nil
		}
		code := strings.TrimSpace(args[0].Get("code").String())
		if code != "" {
			onCode(code)
		}
		return nil
	})
	fail = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		done.Release()
		fail.Release()
		return nil
	})
	cred.Call("get", opts).Call("then", done).Call("catch", fail)
}

// newIdempotencyKey returns a fresh random hex token for
// VerifyPhoneCodeRequest.IdempotencyKey (TODOS.md C443): distinct per
// send-code attempt, reused across retries of verifying the same code.
func newIdempotencyKey() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing would mean no source of randomness is available at
		// all (fatal for the whole app, not just this flow); fall back to a
		// fixed marker rather than panicking mid-render.
		return "idempotency-key-unavailable"
	}
	return hex.EncodeToString(buf)
}

// customSyncErrorMessage extracts a gRPC status message for display, falling
// back to fallback when err carries none — mirroring the status.FromError
// pattern already used in backend.go's uploadOpenAIKeyToBackend.
func customSyncErrorMessage(err error, fallback string) string {
	if st, ok := status.FromError(err); ok && strings.TrimSpace(st.Message()) != "" {
		return st.Message()
	}
	if err != nil {
		return fmt.Sprintf("%s: %v", fallback, err)
	}
	return fallback
}
