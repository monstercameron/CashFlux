// SPDX-License-Identifier: MIT

//go:build js && wasm

// cashflux-portal is a Go→WebAssembly SPA: the CashFlux Cloud customer
// self-service portal, served from web/portal/index.html under the /portal/
// route of the backend. It lets a customer sign in (OAuth), see their
// subscription and usage, subscribe or manage billing (Stripe or PayPal),
// manage signed-in devices, and export or delete their account — all against
// the same-origin backend API, with no internal/* dependencies.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
	"github.com/monstercameron/GoWebComponents/v4/utils"
)

// --- server response mirrors -------------------------------------------------

type meResponse struct {
	UserID       string         `json:"userId"`
	Subscription meSubscription `json:"subscription"`
	Usage        meUsage        `json:"usage"`
	Billing      meBilling      `json:"billing"`
}

type meSubscription struct {
	Status           string `json:"status"`
	Plan             string `json:"plan,omitempty"`
	Provider         string `json:"provider,omitempty"`
	CurrentPeriodEnd string `json:"currentPeriodEnd,omitempty"`
	TrialEnd         string `json:"trialEnd,omitempty"`
	Active           bool   `json:"active"`
}

type meUsage struct {
	Day      string `json:"day"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}

type meBilling struct {
	Enabled          bool     `json:"enabled"`
	PaymentProviders []string `json:"paymentProviders,omitempty"`
}

type sessionRow struct {
	FamilyID  string `json:"familyId"`
	ExpiresAt string `json:"expiresAt"`
	Current   bool   `json:"current,omitempty"`
}

type sessionsResp struct {
	Sessions []sessionRow `json:"sessions"`
}

type billingSession struct {
	URL string `json:"url"`
}

// --- state -------------------------------------------------------------------

type screen int

const (
	screenLoading screen = iota
	screenHome
	screenDashboard
)

const tokenKey = "cashflux.portal.token"
const csrfKey = "cashflux.portal.csrf"
const csrfHeader = "X-CashFlux-CSRF"

// --- localStorage ------------------------------------------------------------

func lsGet(key string) string {
	v := js.Global().Get("localStorage").Call("getItem", key)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	if s := v.String(); s != "null" && s != "undefined" {
		return s
	}
	return ""
}

func lsSet(key, val string) { js.Global().Get("localStorage").Call("setItem", key, val) }
func lsRemove(key string)   { js.Global().Get("localStorage").Call("removeItem", key) }

func origin() string { return js.Global().Get("location").Get("origin").String() }

// --- API ---------------------------------------------------------------------

func authed(method, path, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf := lsGet(csrfKey); csrf != "" {
		req.Header.Set(csrfHeader, csrf)
	}
	return http.DefaultClient.Do(req)
}

// tryRefresh exchanges the same-origin HttpOnly refresh cookie for a fresh access
// token (rotating the CSRF token too), so a customer's session survives the short
// access-token lifetime without re-authenticating. Returns ("", false) if there is
// no valid refresh session.
func tryRefresh() (string, bool) {
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	if err != nil {
		return "", false
	}
	if csrf := lsGet(csrfKey); csrf != "" {
		req.Header.Set(csrfHeader, csrf)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	if json.Unmarshal(body, &out) != nil || strings.TrimSpace(out.AccessToken) == "" {
		return "", false
	}
	if newCSRF := resp.Header.Get(csrfHeader); newCSRF != "" {
		lsSet(csrfKey, newCSRF)
	}
	lsSet(tokenKey, out.AccessToken)
	return out.AccessToken, true
}

// fetchMe loads the caller's account snapshot. authErr is true on 401/403.
func fetchMe(token string) (me *meResponse, authErr bool, err error) {
	resp, err := authed(http.MethodGet, "/v1/me", token, nil)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, true, nil
	}
	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var m meResponse
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, false, err
	}
	return &m, false, nil
}

func fetchSessions(token string) []sessionRow {
	resp, err := authed(http.MethodGet, "/v1/auth/sessions", token, nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	var sr sessionsResp
	_ = json.Unmarshal(body, &sr)
	return sr.Sessions
}

// startOAuth opens the provider sign-in popup and captures the posted session.
func startOAuth(provider string, onDone func(token, csrf string), onError func(string)) {
	window := js.Global().Get("window")
	org := origin()
	var listener js.Func
	listener = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		if event.Get("origin").String() != org {
			return nil
		}
		data := event.Get("data")
		if data.IsUndefined() || data.IsNull() || data.Get("type").String() != "cashflux.oauth" {
			return nil
		}
		token := strings.TrimSpace(data.Get("accessToken").String())
		csrf := strings.TrimSpace(data.Get("csrf").String())
		if token == "" {
			onError("Sign-in didn't return an access token.")
			return nil
		}
		window.Call("removeEventListener", "message", listener)
		onDone(token, csrf)
		return nil
	})
	window.Call("addEventListener", "message", listener)
	returnTo := js.Global().Get("location").Get("href").String()
	loginURL := "/v1/auth/" + url.PathEscape(provider) + "?returnTo=" + url.QueryEscape(returnTo)
	popup := window.Call("open", loginURL, "cashflux-oauth", "popup,width=520,height=720")
	if popup.IsUndefined() || popup.IsNull() {
		onError("The browser blocked the sign-in window. Allow pop-ups and try again.")
	}
}

// redirectBilling POSTs to a billing endpoint and navigates to the returned URL.
func redirectBilling(path, token string, body map[string]string, onError func(string)) {
	go func() {
		var reader io.Reader
		if body != nil {
			data, _ := json.Marshal(body)
			reader = strings.NewReader(string(data))
		}
		resp, err := authed(http.MethodPost, path, token, reader)
		if err != nil {
			onError("Couldn't reach the server.")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			onError(fmt.Sprintf("Billing request failed (HTTP %d).", resp.StatusCode))
			return
		}
		raw, _ := io.ReadAll(resp.Body)
		var out billingSession
		if err := json.Unmarshal(raw, &out); err != nil || strings.TrimSpace(out.URL) == "" {
			onError("The billing response was invalid.")
			return
		}
		js.Global().Get("location").Call("assign", strings.TrimSpace(out.URL))
	}()
}

// --- app ---------------------------------------------------------------------

func App() ui.Node {
	view := ui.UseState(screenLoading)
	token := ui.UseState(lsGet(tokenKey))
	me := ui.UseState[*meResponse](nil)
	sessions := ui.UseState[[]sessionRow](nil)
	interval := ui.UseState("annual")
	provider := ui.UseState("stripe")
	msg := ui.UseState("")

	load := func(tok string) {
		go func() {
			good := tok
			m, authErr, err := fetchMe(good)
			if authErr {
				if nt, ok := tryRefresh(); ok {
					good = nt
					token.Set(nt)
					m, authErr, err = fetchMe(good)
				}
			}
			if authErr || err != nil || m == nil {
				lsRemove(tokenKey)
				lsRemove(csrfKey)
				token.Set("")
				view.Set(screenHome)
				return
			}
			me.Set(m)
			sessions.Set(fetchSessions(good))
			view.Set(screenDashboard)
		}()
	}

	// On mount: if a token is stored, resume; else show the landing.
	ui.UseEffect(func() func() {
		if t := token.Get(); t != "" {
			load(t)
		} else {
			view.Set(screenHome)
		}
		return nil
	}, "portal-boot")

	signIn := func(prov string) func() {
		return func() {
			msg.Set("")
			startOAuth(prov, func(tok, csrf string) {
				lsSet(tokenKey, tok)
				if csrf != "" {
					lsSet(csrfKey, csrf)
				}
				token.Set(tok)
				view.Set(screenLoading)
				load(tok)
			}, func(m string) { msg.Set(m) })
		}
	}
	onSignInGoogle := ui.UseEvent(signIn("google"))
	onSignInGitHub := ui.UseEvent(signIn("github"))
	onSignOut := ui.UseEvent(func() {
		lsRemove(tokenKey)
		lsRemove(csrfKey)
		token.Set("")
		me.Set(nil)
		view.Set(screenHome)
	})
	// revokeSession is a plain func (not a hook) so the per-row session component can
	// own its own OnClick — the framework forbids On* handlers inside a Map loop.
	revokeSession := func(family string) {
		go func() {
			resp, err := authed(http.MethodDelete, "/v1/auth/sessions/"+url.PathEscape(family), token.Get(), nil)
			if err == nil {
				resp.Body.Close()
			}
			sessions.Set(fetchSessions(token.Get()))
		}()
	}
	onSubscribe := ui.UseEvent(func() {
		path := "/v1/billing/checkout"
		if p := provider.Get(); p != "" && p != "stripe" {
			path += "?provider=" + url.QueryEscape(p)
		}
		redirectBilling(path, token.Get(), map[string]string{"interval": interval.Get()}, func(m string) { msg.Set(m) })
	})
	onManage := ui.UseEvent(func() {
		redirectBilling("/v1/billing/portal", token.Get(), map[string]string{}, func(m string) { msg.Set(m) })
	})
	onExport := ui.UseEvent(func() {
		// Same-origin authed download: navigate with the token isn't possible in a
		// header, so open export in a fetch → blob → download anchor.
		go downloadExport(token.Get(), func(m string) { msg.Set(m) })
	})
	onDelete := ui.UseEvent(func() {
		if !js.Global().Call("confirm", "Delete your CashFlux Cloud account and all its server data? This cannot be undone.").Bool() {
			return
		}
		go func() {
			resp, err := authed(http.MethodDelete, "/v1/account", token.Get(), nil)
			if err != nil {
				msg.Set("Couldn't reach the server.")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				msg.Set(fmt.Sprintf("Delete failed (HTTP %d).", resp.StatusCode))
				return
			}
			lsRemove(tokenKey)
			lsRemove(csrfKey)
			token.Set("")
			me.Set(nil)
			view.Set(screenHome)
		}()
	})

	switch view.Get() {
	case screenLoading:
		return Div(css.Class("portal-wrap"), Div(css.Class("portal-card"), P(css.Class("muted"), "Loading…")))
	case screenHome:
		return homeView(onSignInGoogle, onSignInGitHub, msg.Get())
	default:
		return dashboardView(me.Get(), sessions.Get(), interval, provider, revokeSession,
			onSubscribe, onManage, onExport, onDelete, onSignOut, msg.Get())
	}
}

func homeView(onGoogle, onGitHub ui.Handler, msg string) ui.Node {
	return Div(css.Class("portal-wrap"),
		Div(css.Class("portal-card"),
			H1(css.Class("portal-brand"), "CashFlux Cloud"),
			P(css.Class("portal-lede"), "Sync your budget across devices and manage your subscription."),
			If(msg != "", P(css.Class("portal-error"), msg)),
			Div(css.Class("portal-actions"),
				Button(css.Class("portal-btn portal-btn-primary"), Attr("data-testid", "signin-google"), OnClick(onGoogle), "Sign in with Google"),
				Button(css.Class("portal-btn"), Attr("data-testid", "signin-github"), OnClick(onGitHub), "Sign in with GitHub"),
			),
			P(css.Class("portal-fineprint"), "New here? Signing in creates your account."),
		),
	)
}

func dashboardView(me *meResponse, sessions []sessionRow, interval, provider ui.State[string], onRevoke func(string),
	onSubscribe, onManage, onExport, onDelete, onSignOut ui.Handler, msg string) ui.Node {
	if me == nil {
		return Div(css.Class("portal-wrap"), Div(css.Class("portal-card"), P(css.Class("muted"), "No account data.")))
	}
	sub := me.Subscription
	hasSub := sub.Status != "none" && sub.Status != "disabled"
	statusLabel := map[string]string{
		"active": "Active", "trialing": "Free trial", "past_due": "Payment past due",
		"canceled": "Canceled", "none": "No subscription", "disabled": "Local (no billing)",
	}[sub.Status]
	if statusLabel == "" {
		statusLabel = sub.Status
	}

	return Div(css.Class("portal-wrap"),
		Div(css.Class("portal-shell"),
			Div(css.Class("portal-topbar"),
				H1(css.Class("portal-brand"), "CashFlux Cloud"),
				Button(css.Class("portal-btn portal-btn-ghost"), Attr("data-testid", "signout"), OnClick(onSignOut), "Sign out"),
			),
			If(msg != "", P(css.Class("portal-error"), msg)),

			// Subscription card.
			Div(css.Class("portal-card"),
				H2(css.Class("portal-h2"), "Subscription"),
				Div(css.Class("portal-status", "status-"+sub.Status), Attr("data-testid", "sub-status"), statusLabel),
				If(sub.Plan != "", P(css.Class("muted"), "Plan: "+sub.Plan)),
				If(sub.CurrentPeriodEnd != "", P(css.Class("muted"), "Renews: "+sub.CurrentPeriodEnd)),
				If(hasSub && sub.Active, Button(css.Class("portal-btn"), Attr("data-testid", "manage-billing"), OnClick(onManage), "Manage billing")),
				If(!hasSub && me.Billing.Enabled, subscribeControls(me.Billing, interval, provider, onSubscribe)),
			),

			// Usage card.
			Div(css.Class("portal-card"),
				H2(css.Class("portal-h2"), "AI usage today"),
				P(css.Class("portal-usage"), fmt.Sprintf("%d requests · %d tokens", me.Usage.Requests, me.Usage.Tokens)),
			),

			// Devices card.
			Div(css.Class("portal-card"),
				H2(css.Class("portal-h2"), "Signed-in devices"),
				If(len(sessions) == 0, P(css.Class("muted"), "No active sessions.")),
				If(len(sessions) > 0, Div(css.Class("portal-sessions"),
					Map(sessions, func(s sessionRow) ui.Node {
						return ui.CreateElement(sessionItem, sessionItemProps{row: s, onRevoke: onRevoke})
					}),
				)),
			),

			// Data card.
			Div(css.Class("portal-card"),
				H2(css.Class("portal-h2"), "Your data"),
				Div(css.Class("portal-actions"),
					Button(css.Class("portal-btn"), Attr("data-testid", "export"), OnClick(onExport), "Export my data"),
					Button(css.Class("portal-btn portal-btn-danger"), Attr("data-testid", "delete"), OnClick(onDelete), "Delete account"),
				),
			),
		),
	)
}

type sessionItemProps struct {
	row      sessionRow
	onRevoke func(string)
}

// sessionItem is its own component so each row can own an OnClick hook safely —
// the framework rule forbids registering On* handlers inside a Map loop.
func sessionItem(p sessionItemProps) ui.Node {
	revoke := ui.UseEvent(func() { p.onRevoke(p.row.FamilyID) })
	label := "Session " + shortID(p.row.FamilyID)
	if p.row.Current {
		label += " (this device)"
	}
	meta := ""
	if len(p.row.ExpiresAt) >= 10 {
		meta = "Expires " + p.row.ExpiresAt[:10]
	}
	return Div(css.Class("portal-session"),
		Span(label),
		Div(css.Class("portal-actions"),
			If(meta != "", Span(css.Class("muted"), meta)),
			If(!p.row.Current, Button(css.Class("portal-btn portal-btn-ghost"), OnClick(revoke), "Sign out")),
		),
	)
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func subscribeControls(billing meBilling, interval, provider ui.State[string], onSubscribe ui.Handler) ui.Node {
	onInterval := func(v string) func() { return func() { interval.Set(v) } }
	onProvider := func(v string) func() { return func() { provider.Set(v) } }
	pill := func(cur, v, label string, set func()) ui.Node {
		cls := "portal-pill"
		if cur == v {
			cls += " on"
		}
		return Button(css.Class(cls), OnClick(set), label)
	}
	var provPills []any
	provPills = append(provPills, css.Class("portal-pills"))
	for _, p := range billing.PaymentProviders {
		label := map[string]string{"stripe": "Card (Stripe)", "paypal": "PayPal"}[p]
		if label == "" {
			label = p
		}
		provPills = append(provPills, pill(provider.Get(), p, label, onProvider(p)))
	}
	return Div(css.Class("portal-subscribe"),
		P(css.Class("muted"), "Choose a plan:"),
		Div(css.Class("portal-pills"),
			pill(interval.Get(), "annual", "Annual", onInterval("annual")),
			pill(interval.Get(), "monthly", "Monthly", onInterval("monthly")),
		),
		If(len(billing.PaymentProviders) > 1, Div(provPills...)),
		Button(css.Class("portal-btn portal-btn-primary"), Attr("data-testid", "subscribe"), OnClick(onSubscribe), "Subscribe"),
	)
}

// downloadExport fetches the authed export and triggers a browser download.
func downloadExport(token string, onError func(string)) {
	resp, err := authed(http.MethodGet, "/v1/account/export", token, nil)
	if err != nil {
		onError("Couldn't reach the server.")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		onError(fmt.Sprintf("Export failed (HTTP %d).", resp.StatusCode))
		return
	}
	data, _ := io.ReadAll(resp.Body)
	doc := js.Global().Get("document")
	blobParts := js.Global().Get("Array").New()
	blobParts.Call("push", string(data))
	blob := js.Global().Get("Blob").New(blobParts, map[string]any{"type": "application/json"})
	dlURL := js.Global().Get("URL").Call("createObjectURL", blob)
	a := doc.Call("createElement", "a")
	a.Set("href", dlURL)
	a.Set("download", "cashflux-cloud-export.json")
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	doc.Get("body").Call("removeChild", a)
	js.Global().Get("URL").Call("revokeObjectURL", dlURL)
}

func main() {
	ui.Render(ui.CreateElement(App), "#app")
	utils.WaitForever()
}
