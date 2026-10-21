// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/browserstore"

	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

type cloudAuthActions struct{ Google, GitHub, SignOut func() }

func useCloudAuth(atom state.Atom[prefs.Prefs], endpoint string, bump func()) cloudAuthActions {
	notice := uistate.UseNotice()
	notify := func(s string, bad bool) { notice.Set(notice.Get().With(s, bad)) }
	saveOAuth := func(pair backendrpc.TokenPairResponse, csrf string) {
		previous := outgoingRefreshScope()
		beginSessionTransition()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			target, err := resolveAuthScope(ctx, endpoint, pair.AccessToken)
			if err != nil {
				notify("Sign-in could not confirm your account. Please try again.", true)
				return
			}
			finish := func(err error) {
				if err != nil {
					logSyncError("save OAuth session failed", err)
					notify(uistate.T("cloud1.saveFailed"), true)
					return
				}
				saveSessionRoutingThen(target, func(err error) {
					if err != nil {
						logSyncError("save OAuth routing failed", err)
						notify(uistate.T("cloud1.saveFailed"), true)
						return
					}
					p := atom.Get()
					p.ServerURL, p.ServerToken, p.ServerCSRF = target.Endpoint, "", ""
					p.BackendDisabled = cloudSetupPending()
					commitAuthPreferences(atom, p, func(err error) {
						if err != nil {
							logSyncError("complete cloud OAuth sign-in failed", err)
							notify(uistate.T("cloud1.saveFailed"), true)
							return
						}
						reloadAfterIdentityChange("cloud OAuth sign-in")
					})
				})
			}
			if target.Mode == authRefreshModeCookie {
				persistOAuthCookieSessionThen(atom, previous, target, pair.AccessToken, csrf, pair.ExpiresInSeconds, finish)
			} else {
				saveScopedSessionThen(previous, target, pair, finish)
			}
		}()
	}
	login := func(provider string) {
		if !datasetEncryptionActive() {
			return
		}
		startScopedOAuthLogin(endpoint, provider, saveOAuth, func(msg string) { notify(msg, true) })
	}
	signOut := func() {
		uistate.ConfirmModalLabeled(uistate.T("cloud1.signOutConfirm"), uistate.T("settings.signOut"), false, func(ok bool) {
			if !ok {
				return
			}
			p := atom.Get()
			endPageSessionThen(p, func(err error) {
				if err != nil {
					logSyncError("remove cloud sign-in failed", err)
					notify(uistate.T("cloud1.saveFailed"), true)
					return
				}
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
		uic.CreateElement(cloudConnectionFeedback, cloudFeedbackProps{Phase: c.Phase, Error: c.Error}),
		If(c.Phase != discoveryOK, uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.test"), TestID: "cloud-signin-test-connection", OnClick: c.Test, Disabled: c.Phase == discoveryChecking})),
		If(password, uic.CreateElement(PasswordAuthCard, PasswordAuthCardProps{AllowRegistration: d.RegistrationOpen, InitiallyExpanded: true})),
		If(activation, uic.CreateElement(PendingDeviceCard, PendingDeviceCardProps{Primary: false})),
		If(oauth, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.signInGoogle"), TestID: "sync-oauth-google", OnClick: p.Auth.Google, Disabled: !p.Protected || !cloudProviderEnabled(c, "google")}),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.signInGitHub"), TestID: "sync-oauth-github", OnClick: p.Auth.GitHub, Disabled: !p.Protected || !cloudProviderEnabled(c, "github")}),
		)),
		If(oauth && !p.Protected, Fragment(
			P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.oauthProtection")),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.protectAction"), TestID: "cloud-oauth-protect", OnClick: p.OnProtect}),
		)),
		uic.CreateElement(cloudDisclosure, cloudDisclosureProps{Title: uistate.T("cloud1.moreSignIn"), TestID: "sync-advanced-token-toggle", Body: Fragment(
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

// startScopedOAuthLogin retains its PKCE verifier only in this popup attempt's
// memory. Origin AND popup source are checked; completion is one-shot.
func startScopedOAuthLogin(endpoint, provider string, done func(backendrpc.TokenPairResponse, string), onError func(string)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	scope, err := newRefreshScope("oauth-mode", "oauth-mode", endpoint, currentPageOrigin())
	if err != nil || (provider != "google" && provider != "github") {
		onError("The sign-in server address is invalid.")
		return
	}
	returnTo := js.Global().Get("location").Get("href").String()
	oauthOrigin := js.Global().Get("URL").New(endpoint).Get("origin").String()
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		onError("The security check could not start. Please try again.")
		return
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	query := url.Values{"returnTo": {returnTo}}
	if scope.Mode == authRefreshModeEncrypted {
		query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(digest[:]))
		query.Set("code_challenge_method", "S256")
	}
	window := js.Global().Get("window")
	var listener js.Func
	var popup js.Value
	var once sync.Once
	var timer *time.Timer
	finish := func(pair backendrpc.TokenPairResponse, csrf string, err error) {
		once.Do(func() {
			window.Call("removeEventListener", "message", listener)
			listener.Release()
			if timer != nil {
				timer.Stop()
			}
			if err != nil {
				onError("Sign-in could not finish. Please try again.")
			} else {
				done(pair, csrf)
			}
		})
	}
	listener = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		if jsString(event.Get("origin")) != oauthOrigin || !popup.Truthy() || !event.Get("source").Equal(popup) {
			return nil
		}
		data := event.Get("data")
		if data.Type() != js.TypeObject || data.IsNull() || jsString(data.Get("type")) != "cashflux.oauth" {
			return nil
		}
		// Detach immediately; a second message must never redeem or save twice.
		window.Call("removeEventListener", "message", listener)
		if scope.Mode == authRefreshModeCookie {
			pair := backendrpc.TokenPairResponse{AccessToken: strings.TrimSpace(jsString(data.Get("accessToken"))), ExpiresInSeconds: oauthFallbackExpiresInSeconds}
			if n := data.Get("expiresIn"); n.Type() == js.TypeNumber {
				pair.ExpiresInSeconds = int64(n.Int())
			}
			csrf := strings.TrimSpace(jsString(data.Get("csrf")))
			go func() {
				if pair.AccessToken == "" {
					finish(pair, "", errors.New("missing access token"))
					return
				}
				finish(pair, csrf, nil)
			}()
		} else {
			code := strings.TrimSpace(jsString(data.Get("code")))
			go func() {
				if code == "" {
					finish(backendrpc.TokenPairResponse{}, "", errors.New("missing authorization code"))
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				pair, err := exchangeOAuthCode(ctx, endpoint, code, verifier, returnTo)
				finish(pair, "", err)
			}()
		}
		return nil
	})
	window.Call("addEventListener", "message", listener)
	popup = window.Call("open", endpoint+"/v1/auth/"+provider+"?"+query.Encode(), "cashflux-oauth", "popup,width=520,height=720")
	if !popup.Truthy() {
		go finish(backendrpc.TokenPairResponse{}, "", errors.New("popup blocked"))
		return
	}
	timer = time.AfterFunc(5*time.Minute, func() { finish(backendrpc.TokenPairResponse{}, "", errors.New("sign-in timed out")) })
}

func exchangeOAuthCode(ctx context.Context, endpoint, code, verifier, returnTo string) (backendrpc.TokenPairResponse, error) {
	var out struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int64  `json:"expiresIn"`
	}
	err := sessionHTTP(ctx, endpoint, "/v1/auth/session/code", "", "", false, map[string]string{"code": code, "codeVerifier": verifier, "returnTo": returnTo}, &out)
	if err != nil {
		return backendrpc.TokenPairResponse{}, err
	}
	if out.AccessToken == "" || out.RefreshToken == "" || out.ExpiresIn <= 0 {
		return backendrpc.TokenPairResponse{}, errors.New("authorization code returned an incomplete session")
	}
	return backendrpc.TokenPairResponse{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken, ExpiresInSeconds: out.ExpiresIn}, nil
}

// sessionHTTP waits for the browser response (including cookie installation).
// Cancellation settles the fetch before a caller can release the refresh lock.
func sessionHTTP(ctx context.Context, endpoint, path, token, csrf string, cookies bool, input, output any) error {
	if err := validateBackendTransport(endpoint); err != nil {
		return fmt.Errorf("session request: %w", err)
	}
	headers := js.Global().Get("Headers").New()
	headers.Call("set", "Content-Type", "application/json")
	if token != "" {
		headers.Call("set", "Authorization", "Bearer "+token)
	}
	if csrf != "" {
		headers.Call("set", "X-CashFlux-CSRF", csrf)
	}
	opts := js.Global().Get("Object").New()
	opts.Set("method", "POST")
	opts.Set("headers", headers)
	opts.Set("credentials", "omit")
	if cookies {
		opts.Set("credentials", "include")
	}
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode session request: %w", err)
		}
		opts.Set("body", string(raw))
	}
	controller := js.Global().Get("AbortController").New()
	opts.Set("signal", controller.Get("signal"))
	type response struct {
		status int
		body   string
		err    error
	}
	done := make(chan response, 1)
	var received, decoded, failed js.Func
	var status int
	received = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resp := args[0]
		status = resp.Get("status").Int()
		resp.Call("text").Call("then", decoded).Call("catch", failed)
		return nil
	})
	decoded = js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- response{status: status, body: args[0].String()}
		return nil
	})
	failed = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		done <- response{err: errors.New("session request could not reach the server")}
		return nil
	})
	defer received.Release()
	defer decoded.Release()
	defer failed.Release()
	js.Global().Call("fetch", endpoint+path, opts).Call("then", received).Call("catch", failed)
	var res response
	select {
	case res = <-done:
	case <-ctx.Done():
		controller.Call("abort")
		<-done
		return ctx.Err()
	}
	if res.err != nil {
		return res.err
	}
	if res.status < 200 || res.status >= 300 {
		return fmt.Errorf("session request: HTTP %d", res.status)
	}
	if output != nil {
		if err := json.Unmarshal([]byte(res.body), output); err != nil {
			return fmt.Errorf("decode session response: %w", err)
		}
	}
	return nil
}

func installSessionCookie(ctx context.Context, scope refreshScope, pair backendrpc.TokenPairResponse) (backendrpc.TokenPairResponse, error) {
	if scope.Mode != authRefreshModeCookie {
		return pair, nil
	}
	if err := sessionHTTP(ctx, scope.Endpoint, "/v1/auth/session/csrf", pair.AccessToken, "", true, nil, nil); err != nil {
		return backendrpc.TokenPairResponse{}, err
	}
	csrf := cookieSessionCSRF(prefs.Prefs{ServerURL: scope.Endpoint})
	if csrf == "" {
		return backendrpc.TokenPairResponse{}, errors.New("Sign-in could not read the security cookie. Please try again.")
	}
	var out struct {
		AccessToken string `json:"accessToken"`
		UserID      string `json:"userId"`
		ExpiresIn   int64  `json:"expiresIn"`
	}
	err := sessionHTTP(ctx, scope.Endpoint, "/v1/auth/session/cookie", pair.AccessToken, csrf, true, map[string]string{"refreshToken": pair.RefreshToken}, &out)
	if err != nil {
		return backendrpc.TokenPairResponse{}, err
	}
	if out.UserID != scope.AccountID || out.AccessToken == "" || out.ExpiresIn <= 0 {
		return backendrpc.TokenPairResponse{}, errors.New("Sign-in returned an unexpected account or session.")
	}
	return backendrpc.TokenPairResponse{AccessToken: out.AccessToken, ExpiresInSeconds: out.ExpiresIn}, nil
}

// endPageSessionThen uses the outgoing credential for best-effort revocation
// before durably deleting it. A failed local delete keeps the page for retry.
func endPageSessionThen(pr prefs.Prefs, done func(error)) {
	scope := outgoingRefreshScope()
	if scope == nil {
		if !backendSessionReady() {
			done(errors.New("Sign-out needs the original server identity. Reconnect and try again."))
			return
		}
		beginSessionTransition()
		clearAuthSession()
		clearRefreshMemory()
		browserstore.RemoveSecure(syncUserIDKey, done)
		return
	}
	beginSessionTransition()
	go func() {
		withTokenRefreshLock(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if scope.Mode == authRefreshModeCookie {
				// A second tab may have replaced the single browser cookie with
				// another account. Confirm its account before revoking any family.
				csrf := cookieSessionCSRF(prefs.Prefs{ServerURL: scope.Endpoint})
				if csrf != "" {
					result, completed := fetchCookieRefresh(ctx, scope.Endpoint, csrf)
					if completed && result.err == nil && result.status == 200 && result.userID == scope.AccountID {
						_ = sessionHTTP(ctx, scope.Endpoint, "/v1/auth/refresh/logout", result.accessToken, cookieSessionCSRF(prefs.Prefs{ServerURL: scope.Endpoint}), true, nil, nil)
					}
				}
			} else if credential, _, have, err := (scopedRefreshSession{*scope}).load(); err == nil && have {
				var out backendrpc.LogoutResponse
				_ = invokeWorkerRPC(ctx, scope.Endpoint, "refresh", backendrpc.MethodAuthLogout, backendrpc.LogoutRequest{RefreshToken: credential.Token}, &out)
			}
		})
		clearScopedSessionThen(*scope, func(err error) {
			if err != nil {
				done(err)
				return
			}
			browserstore.RemoveSecure(syncUserIDKey, done)
		})
	}()
}
