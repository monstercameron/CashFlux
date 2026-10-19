// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
)

// This file holds the SV0.2 client half of OAuth (Google/GitHub) sessions
// and of the server-scope reset.
//
// An OAuth sign-in does NOT work like a password/pairing sign-in. The
// server's OAuth callback (internal/server/oauth_http.go
// handleOAuthCallback) issues a stored session pair, but only the ACCESS
// token reaches the page — via the `cashflux.oauth` postMessage, together
// with its lifetime, the user ID and a CSRF token. The refresh token is
// set as an HttpOnly cookie (`cashflux_refresh`, path /v1/auth) that
// JavaScript can neither read nor copy, and rotation happens by POSTing
// /v1/auth/refresh with credentials included: the server consumes the
// cookie, rotates the family, sets fresh cookies, and answers with a new
// access token JSON body.
//
// Until SV0.2 the app stored that access token in prefs.ServerToken as if
// it were a static self-host token. Nothing ever refreshed it, so fifteen
// minutes after an OAuth sign-in every sync call failed Unauthenticated
// and the session silently stopped syncing. The fix is to record the
// session as what it is — a rotatable session whose refresh credential
// lives in the cookie — in the same storage password sessions use, plus a
// marker saying which refresh path applies.

// authRefreshModeKey records HOW the current rotatable session refreshes.
// Absent (the default, and every session stored before this key existed)
// means a locally held refresh token under authRefreshTokenKey, refreshed
// with the AuthService.RefreshToken RPC. authRefreshModeCookie means an
// OAuth session whose refresh token is the server's HttpOnly cookie,
// refreshed via POST /v1/auth/refresh (see refreshCookieAccessToken).
const authRefreshModeKey = "cashflux:auth:refresh-mode"

// authRefreshModeCookie is the authRefreshModeKey value for OAuth sessions.
const authRefreshModeCookie = "cookie"

// authCSRFKey holds the latest double-submit CSRF token for the cookie
// session. The server rotates the CSRF cookie on every refresh and returns
// the new value in the X-CashFlux-CSRF response header (CORS-exposed); the
// next refresh must present that value, so it is tracked here, beside the
// tokens, in preference to the prefs copy — prefs can be a render-cycle
// stale by the time a proactive refresh fires.
const authCSRFKey = "cashflux:auth:csrf"

// cloudAIKeySetKey records that an OpenAI key has been uploaded to the
// currently configured server. It is scoped to that server, so the scope
// reset below clears it alongside the credentials.
const cloudAIKeySetKey = "cashflux:cloud-ai-key-set"

// oauthFallbackExpiresInSeconds is the access-token lifetime assumed for an
// OAuth sign-in when the completion message has not (yet) delivered the
// server's own figure. The postMessage payload carries expiresIn
// (internal/server writeOAuthCallbackHTML), but startOAuthLogin in
// backend.go does not forward it yet — see the SV0.2 report's request for
// that file. The value mirrors the server's sessionAccessTTL
// (internal/server/session.go, 15 minutes) and is only ever used for the
// FIRST proactive-refresh interval: every /v1/auth/refresh response after
// that carries the real expiresIn and replaces it.
const oauthFallbackExpiresInSeconds int64 = 900

// hasCookieSession reports whether the current session is an OAuth cookie
// session — the counterpart of hasRotatableSession for the session type
// whose refresh token is not in local storage.
func hasCookieSession() bool {
	return strings.TrimSpace(lsGet(authRefreshModeKey)) == authRefreshModeCookie
}

// persistOAuthCookieSession stores a completed OAuth sign-in as a rotatable
// cookie session, through the same storage persistAuthSession uses for
// password/pairing sessions: the access token lands under
// authAccessTokenKey (so EffectiveServerToken and Session see it), the
// expiry under authExpiresInKey with the proactive countdown armed, and
// the user ID under syncUserIDKey so queued work is bound to the right
// account from the first push.
//
// Two deliberate differences from a token-pair session:
//
//   - No refresh token is stored, and any refresh token left behind by a
//     previous session on this device is REMOVED. Leaving one behind would
//     make hasRotatableSession keep reporting the old session type and
//     replay a credential from a different sign-in (possibly a different
//     server) against this one.
//   - The refresh-mode marker and the CSRF token are stored, so the
//     refresh path (refreshCookieAccessToken) can find them after the
//     page reload the sign-in ends with.
//
// Unlike persistAuthSession, this path deliberately leaves ServerToken empty:
// OAuth access tokens are rotating credentials, never static server tokens.
// The sync lane must treat a stored rotatable session as backend-active and
// use EffectiveServerToken for its access token.
//
// The caller ends the sign-in with reloadAfterIdentityChange, so sync is
// deliberately NOT started here (startSync=false): the reloaded page's
// startBackendSync re-resolves identity and brings sync up in the new
// scope, which is the SV rule that scope is fixed for the life of a page.
func persistOAuthCookieSession(prefsAtom state.Atom[prefs.Prefs], serverURL, accessToken, csrf, userID string, expiresInSeconds int64) {
	if expiresInSeconds <= 0 {
		expiresInSeconds = oauthFallbackExpiresInSeconds
	}
	p := prefsAtom.Get()
	// Keep OAuth credentials out of ServerToken: that field is the static
	// self-host credential, while this access token rotates. The live copy
	// belongs in the same browser storage keys used by persistAuthSession.
	p.ServerURL = serverURL
	p.ServerToken = ""
	p.ServerCSRF = strings.TrimSpace(csrf)
	p.BackendDisabled = false
	// OAuth can use a commercial endpoint too; preserve the selected mode
	// and segment while applying the same normalization as password sign-in.
	mode, segment := p.ServerMode, p.ConnectionSegment
	p.ServerMode = prefs.ServerSelfHosted
	p = p.Normalize()
	if mode == prefs.ServerCloud {
		p.ServerMode = mode
		p.ConnectionSegment = segment
		p = p.Normalize()
	}
	prefsAtom.Set(p)
	uistate.PersistPrefs(p)
	lsRemove(authRefreshTokenKey)
	lsSet(authRefreshModeKey, authRefreshModeCookie)
	lsSet(authCSRFKey, strings.TrimSpace(csrf))
	storeAuthTokenPair(backendrpc.TokenPairResponse{
		AccessToken:      strings.TrimSpace(accessToken),
		ExpiresInSeconds: expiresInSeconds,
	})
	setSignedInUserID(userID)
}

// refreshCookieAccessToken is the cookie-session counterpart of
// sync_client.go's refreshAccessToken: it performs (or, if another tab wins
// the race, waits for and reuses the result of) a single POST
// /v1/auth/refresh round trip, guarded by the same cross-tab Web Locks
// guard so open tabs never race the rotation. The guard matters here for
// the same reason as for token pairs: the server consumes a presented
// refresh credential exactly once and treats a replay as compromise.
// Two tabs POSTing /v1/auth/refresh at once send the SAME cookie value
// (the browser attaches the stored cookie for both until one response
// lands), so unguarded tabs would revoke each other's session family.
//
// "Another tab already refreshed" is decided by syncstate.DecideRefresh on
// the ACCESS token — the refresh cookie itself is HttpOnly and invisible,
// so a changed stored access token is the only rotation signal a waiting
// tab can observe. It returns false when there is no cookie session, the
// session vanished while waiting, or the refresh attempt failed.
//
// Integration note (SV0.2): sync_client.go's refreshAccessToken and
// hasRotatableSession do not yet branch on the refresh-mode marker, so
// today this function is reached only where cloudtab/authtoken code calls
// it directly. The exact branch sync_client.go needs is listed in the
// SV0.2 report under "Requests for other files" — that file belongs to
// lane SV0.1.
func refreshCookieAccessToken(ctx context.Context, pr prefs.Prefs) bool {
	if !hasCookieSession() {
		return false
	}
	startingAccess := strings.TrimSpace(lsGet(authAccessTokenKey))
	ok := false
	withTokenRefreshLock(func() {
		// Re-read from storage before deciding: browserstore's cache is
		// per-tab, so without this the comparison could only ever see our
		// own starting value and every waiting tab would POST its own
		// refresh with the same cookie. Same rule (and same reason) as
		// refreshAccessToken in sync_client.go.
		browserstore.Reload(authAccessTokenKey, authRefreshModeKey, authCSRFKey)
		if !hasCookieSession() {
			return
		}
		switch syncstate.DecideRefresh("", "", startingAccess, lsGet(authAccessTokenKey)) {
		case syncstate.RefreshReuseStored:
			ok = true
			return
		case syncstate.RefreshSessionGone:
			return
		}
		ok = doCookieRefresh(ctx, pr)
	})
	return ok
}

// cookieRefreshResult is one completed /v1/auth/refresh exchange, decoded
// from the fetch response on the JS side of the bridge.
type cookieRefreshResult struct {
	status      int
	accessToken string
	userID      string
	csrf        string
	expiresIn   int64
	err         error
}

// doCookieRefresh makes the actual POST /v1/auth/refresh call. It must run
// only while holding the token-refresh lock (via refreshCookieAccessToken)
// so it is never invoked twice concurrently for the same session.
func doCookieRefresh(ctx context.Context, pr prefs.Prefs) bool {
	endpoint := normalizedBackendEndpoint(pr.ServerURL)
	res, completed := fetchCookieRefresh(ctx, endpoint, cookieSessionCSRF(pr))
	if !completed {
		// The caller's deadline went first. The request may still land and
		// rotate the cookie server-side; a later refresh re-reads storage
		// under the lock and reuses whatever arrived, so nothing is lost by
		// reporting failure now.
		return false
	}
	if res.err != nil {
		logSyncError("cookie token refresh failed", res.err)
		return false
	}
	if res.status == 401 {
		// The refresh cookie itself is expired/revoked/replayed — there is
		// no credential left to recover, the exact counterpart of
		// doRefreshAccessToken's Unauthenticated branch: drop to local-only
		// silently, dataset untouched. The cookie-mode keys go first
		// because clearAuthSession (sync_client.go, lane SV0.1) does not
		// know about them yet.
		lsRemove(authRefreshModeKey)
		lsRemove(authCSRFKey)
		degradeToLocalOnly()
		return false
	}
	if res.status < 200 || res.status >= 300 || strings.TrimSpace(res.accessToken) == "" {
		logSyncError("cookie token refresh failed",
			fmt.Errorf("server returned HTTP %d", res.status))
		return false
	}
	expires := res.expiresIn
	if expires <= 0 {
		expires = oauthFallbackExpiresInSeconds
	}
	storeAuthTokenPair(backendrpc.TokenPairResponse{
		AccessToken:      res.accessToken,
		ExpiresInSeconds: expires,
	})
	// storeAuthTokenPair only writes a refresh token when the pair carries
	// one; a cookie session must never have one in storage (it would flip
	// hasRotatableSession to the token-pair meaning), so be sure.
	lsRemove(authRefreshTokenKey)
	if strings.TrimSpace(res.csrf) != "" {
		lsSet(authCSRFKey, strings.TrimSpace(res.csrf))
		fresh := uistate.LoadPrefs()
		fresh.ServerCSRF = strings.TrimSpace(res.csrf)
		uistate.PersistPrefs(fresh.Normalize())
	}
	if strings.TrimSpace(res.userID) != "" {
		setSignedInUserID(res.userID)
	}
	// Same write-side rule as doRefreshAccessToken: do not release the
	// cross-tab lock until the rotated access token has COMMITTED, or the
	// next tab re-reads the old token, decides nothing rotated, and POSTs
	// a second refresh presenting the cookie this call just consumed.
	browserstore.Barrier()
	return true
}

// cookieSessionCSRF resolves the double-submit token to present with a
// cookie-session request: the latest value tracked beside the tokens
// wins, falling back to the prefs copy from the original sign-in.
func cookieSessionCSRF(pr prefs.Prefs) string {
	if v := strings.TrimSpace(lsGet(authCSRFKey)); v != "" {
		return v
	}
	return strings.TrimSpace(pr.ServerCSRF)
}

// fetchCookieRefresh POSTs to {endpoint}/v1/auth/refresh with cookies
// attached (credentials: "include" — the refresh credential is the
// HttpOnly cookie, and a fetch without it sends nothing the server can
// rotate) and the double-submit CSRF header the endpoint requires. It
// drives the fetch Promise chain into a channel so the calling goroutine
// can select on ctx; completed is false when ctx ended first.
//
// This uses the JS fetch API directly rather than net/http for the same
// reason signOutBackendOAuth does: Go's wasm net/http transport cannot be
// told to include cross-origin credentials, and the whole request is
// pointless without the cookie.
func fetchCookieRefresh(ctx context.Context, endpoint, csrf string) (cookieRefreshResult, bool) {
	done := make(chan cookieRefreshResult, 1)

	headers := js.Global().Get("Headers").New()
	if strings.TrimSpace(csrf) != "" {
		headers.Call("set", "X-CashFlux-CSRF", strings.TrimSpace(csrf))
	}
	opts := js.Global().Get("Object").New()
	opts.Set("method", "POST")
	opts.Set("credentials", "include")
	opts.Set("headers", headers)

	var onResponse, onJSON, onError js.Func
	release := func() {
		onResponse.Release()
		onJSON.Release()
		onError.Release()
	}
	onError = js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "refresh request failed"
		if len(args) > 0 && args[0].Truthy() {
			if s := jsString(args[0].Get("message")); s != "" {
				msg = s
			}
		}
		release()
		done <- cookieRefreshResult{err: errors.New(msg)}
		return nil
	})
	// status/respCSRF are captured by onResponse and read by onJSON: the
	// HTTP status and the rotated CSRF header live on the fetch response,
	// the tokens in its JSON body — two Promise hops for one result.
	var status int
	var respCSRF string
	onJSON = js.FuncOf(func(_ js.Value, args []js.Value) any {
		res := cookieRefreshResult{status: status, csrf: respCSRF}
		if len(args) > 0 && args[0].Truthy() {
			body := args[0]
			res.accessToken = jsString(body.Get("accessToken"))
			res.userID = jsString(body.Get("userId"))
			if v := body.Get("expiresIn"); v.Type() == js.TypeNumber {
				res.expiresIn = int64(v.Int())
			}
		}
		release()
		done <- res
		return nil
	})
	onResponse = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || !args[0].Truthy() {
			release()
			done <- cookieRefreshResult{err: errors.New("refresh request returned no response")}
			return nil
		}
		resp := args[0]
		status = resp.Get("status").Int()
		// X-CashFlux-CSRF is in the server's Access-Control-Expose-Headers,
		// so it is readable cross-origin; absent (an older server) it just
		// stays empty and the stored value is kept.
		respCSRF = jsString(resp.Get("headers").Call("get", "X-CashFlux-CSRF"))
		resp.Call("json").Call("then", onJSON).Call("catch", onError)
		return nil
	})
	js.Global().Call("fetch", endpoint+"/v1/auth/refresh", opts).
		Call("then", onResponse).Call("catch", onError)

	select {
	case res := <-done:
		return res, true
	case <-ctx.Done():
		// The Promise chain may still settle; its terminal callback sends
		// into the buffered channel and releases the funcs itself, so
		// abandoning it here blocks nothing and leaks nothing.
		return cookieRefreshResult{}, false
	}
}

// jsString reads a JS string value, returning "" for anything else (null,
// undefined, a number) instead of the "undefined"/"<null>" spellings
// js.Value.String() produces for non-strings.
func jsString(v js.Value) string {
	if v.Type() != js.TypeString {
		return ""
	}
	return v.String()
}

// resetScopeForServerChange clears every stored key that belongs to the
// previously configured server when the Server URL's host changes (SV0.2):
// the rotated access token, the refresh token, their expiry, the
// refresh-mode marker, the CSRF token, the signed-in user ID, the sync
// queue, every per-workspace sync bookmark (cashflux:sync-meta:*), and the
// "cloud AI key stored" flag. The dataset owner deliberately survives: the
// local dataset survives too, and the next account must see the previous owner
// and treat that data as foreign rather than adopting it.
// Without clearing the connection state,
// EffectiveServerToken keeps preferring the OLD server's rotated JWT, and
// the old server's queued snapshots and bookmarks are presented to the new
// server as if they were its own — data mixing across scopes, which the SV
// principles forbid outright.
//
// The dataset itself is NOT cleared: it is the household's local data, not
// the old server's, and discarding it would be the silent overwrite the
// principles also forbid. What is dropped is only the old CONNECTION's
// state — credentials that mean nothing to the new server, pushes queued
// against it, and bookmarks describing conversations with it. (A queued
// snapshot's data still exists in the local dataset; the new scope's first
// sync re-derives what to push.)
//
// It reports whether any scoped state was actually present, so the caller
// can tell a genuine scope change (there was a session on the old host —
// the page must reload into the new scope) from a first-time address being
// typed on a device that never had a session.
func resetScopeForServerChange() bool {
	exact := []string{
		authAccessTokenKey,
		authRefreshTokenKey,
		authExpiresInKey,
		authRefreshModeKey,
		authCSRFKey,
		syncUserIDKey,
		syncQueueKey,
		cloudAIKeySetKey,
	}
	keys := syncstate.KeysToClearOnServerChange(browserstore.Keys(), exact, []string{syncMetaPrefix})
	// Live teardown first — stop the watch, the proactive-refresh timer and
	// the pooled sockets authenticated as the old server — then the stored
	// keys. clearAuthSession already removes the token pair, expiry and
	// user ID; the loop covers the rest (and is harmless where it overlaps).
	clearAuthSession()
	for _, k := range keys {
		lsRemove(k)
	}
	// Keep the in-memory pending count honest; saveSyncQueue owns it and
	// removing the key directly above would leave it stale.
	saveSyncQueue(nil)
	return len(keys) > 0
}
