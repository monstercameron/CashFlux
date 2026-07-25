// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// handoffParam is the query parameter carrying an activation code that was minted
// for this visitor rather than typed by them.
const handoffParam = "activate"

// TryActivationHandoff signs this device in from an activation code supplied in the
// page URL, then strips it. It is the client half of "open CashFlux from the admin
// console and just be signed in": the console mints a code against its own
// authenticated session and navigates here with it, so the operator never types a
// credential to reach their own data.
//
// Three properties make putting a code in a URL acceptable here, and all three are
// load-bearing:
//
//   - the code is single-use and expires in minutes, so a copy in shell history or
//     a bookmark is worthless almost immediately;
//   - it is removed from the address bar via replaceState the moment it is read, so
//     it never enters this page's session history;
//   - it grants no more than typing the same six digits into Settings would.
//
// A device that already has a session ignores the parameter entirely — arriving
// with a code must never silently re-key a working client onto a different account.
func TryActivationHandoff() {
	code := handoffCodeFromURL()
	if code == "" {
		return
	}
	stripHandoffParam()

	pr := uistate.LoadPrefs().Normalize()
	if strings.TrimSpace(pr.ServerToken) != "" {
		return // already signed in; the code simply goes unused and expires
	}

	go func() {
		ctx := context.Background()
		// Same-origin by construction: the code came from the page's own URL, so the
		// server that minted it is the one serving this app.
		origin := currentPageOrigin()
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: origin, Token: "refresh"})
		if err != nil {
			logSyncError("activation handoff dial failed", err)
			return
		}
		defer conn.Close()

		var out backendrpc.TokenPairResponse
		if err := conn.Invoke(ctx, backendrpc.MethodAuthRedeemPairingCode, backendrpc.RedeemPairingCodeRequest{
			PairingCode:    code,
			DeviceLabel:    customSyncDeviceLabel(),
			IdempotencyKey: newIdempotencyKey(),
		}, &out, backendrpc.JSONCallOptions()...); err != nil {
			logSyncError("activation handoff redeem failed", err)
			return
		}

		// Persist through the same path every other sign-in uses, so the token
		// lifecycle, the watch and the first pull all behave identically to a code
		// typed by hand. The server URL is set explicitly because a device arriving
		// this way may never have had one.
		p := uistate.LoadPrefs()
		p.ServerURL = origin
		p.ServerMode = prefs.ServerSelfHosted
		p.ServerToken = strings.TrimSpace(out.AccessToken)
		p.BackendDisabled = false
		uistate.PersistPrefs(p.Normalize())
		storeAuthTokenPair(out)
		restartBackendSync()
		uistate.PostNotice(uistate.T("authCards.activated"), false)
	}()
}

// handoffCodeFromURL reads the activation code from the query string, returning ""
// when absent or malformed. Anything that is not a plain code is ignored rather
// than passed to the server.
func handoffCodeFromURL() string {
	loc := js.Global().Get("location")
	if loc.IsUndefined() || loc.IsNull() {
		return ""
	}
	search := loc.Get("search").String()
	if search == "" {
		return ""
	}
	params := js.Global().Get("URLSearchParams").New(search)
	raw := params.Call("get", handoffParam)
	if raw.IsNull() || raw.IsUndefined() {
		return ""
	}
	code, err := normalizePairingCode(raw.String())
	if err != nil {
		return ""
	}
	return code
}

// stripHandoffParam rewrites the address bar without the code, using replaceState
// so it never becomes a history entry the back button can return to.
func stripHandoffParam() {
	loc := js.Global().Get("location")
	history := js.Global().Get("history")
	if loc.IsUndefined() || history.IsUndefined() || history.Get("replaceState").IsUndefined() {
		return
	}
	url := js.Global().Get("URL").New(loc.Get("href").String())
	url.Get("searchParams").Call("delete", handoffParam)
	history.Call("replaceState", js.Null(), "", url.Get("pathname").String()+url.Get("search").String()+url.Get("hash").String())
}
