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
	"github.com/monstercameron/GoWebComponents/v7/state"
)

// Session refresh has two explicit modes: same-origin HTTPS uses the server's
// HttpOnly cookie; cross-origin uses a device-wrapped per-account record.
// Access and CSRF tokens stay in page memory. Lifecycle integration uses the
// durable callback helpers below and bootstrapScopedSession before sync starts.

// authRefreshModeKey is the current session's refresh-mode metadata.
const authRefreshModeKey = "cashflux:auth:refresh-mode"

// authRefreshModeCookie is the authRefreshModeKey value for OAuth sessions.
const authRefreshModeCookie = "cookie"

// authCSRFKey holds a transient double-submit token. It is never saved in prefs.
const authCSRFKey = "cashflux:auth:csrf"

// cloudAIKeySetKey is a legacy local hint about a server key, not an
// authoritative server status. Clear it when changing servers; AI Settings
// no longer reads it because a local hint can become stale.
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

// persistOAuthCookieSessionThen is the durable OAuth sign-in boundary. The
// callback's account ID and the selected server ID must be resolved before this
// call. The caller commits its preferences and reloads only after done(nil).
func persistOAuthCookieSessionThen(prefsAtom state.Atom[prefs.Prefs], previous *refreshScope, target refreshScope, accessToken, csrf string, expiresInSeconds int64, done func(error)) {
	selected, err := newRefreshScope(target.ServerID, target.AccountID, target.Endpoint, currentPageOrigin())
	if err != nil || selected != target || target.Mode != authRefreshModeCookie {
		done(errors.New("session refresh: OAuth cookie sign-in requires the same secure origin"))
		return
	}
	if expiresInSeconds <= 0 {
		expiresInSeconds = oauthFallbackExpiresInSeconds
	}
	pair := backendrpc.TokenPairResponse{AccessToken: accessToken, ExpiresInSeconds: expiresInSeconds}
	saveScopedSessionThen(previous, target, pair, func(err error) {
		if err == nil {
			persistOAuthCookieSession(prefsAtom, target.Endpoint, accessToken, csrf, target.AccountID, expiresInSeconds)
		}
		done(err)
	})
}

// clearScopedSessionThen stops live session work and durably removes this
// scope's credential before clearing generic metadata. The caller handles its
// server logout first and reloads only after done(nil).
func clearScopedSessionThen(scope refreshScope, done func(error)) {
	stopBackendWatch()
	stopProactiveRefreshTimer()
	dropSharedConn()
	(scopedRefreshSession{scope: scope}).clearThen(func(err error) {
		if err == nil {
			clearAuthSession()
			lsRemove(authRefreshModeKey)
			lsRemove(authCSRFKey)
		}
		done(err)
	})
}

// resetScopeForServerChangeThen must receive the outgoing page's fixed scope,
// rather than looking it up after new server preferences have been committed.
func resetScopeForServerChangeThen(previous refreshScope, done func(bool, error)) {
	clearScopedSessionThen(previous, func(err error) {
		if err != nil {
			done(false, err)
			return
		}
		done(resetScopeForServerChange(), nil)
	})
}

// hasCookieSession reports whether the current session is an OAuth cookie
// session — the counterpart of hasRotatableSession for the session type
// whose refresh token is not in local storage.
func hasCookieSession() bool {
	return strings.TrimSpace(lsGet(authRefreshModeKey)) == authRefreshModeCookie
}

// persistOAuthCookieSession applies legacy OAuth preference and memory state.
// New sign-in callers must use persistOAuthCookieSessionThen so the scoped
// cookie marker is durable before this state is adopted and the page reloads.
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
	p.ServerCSRF = ""
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

// refreshCookieAccessToken rotates a same-origin cookie session under the
// cross-tab lock. The browser commits the HttpOnly cookie before fetch settles.
func refreshCookieAccessToken(ctx context.Context, pr prefs.Prefs) bool {
	if !hasCookieSession() {
		return false
	}
	startingAccess := strings.TrimSpace(lsGet(authAccessTokenKey))
	ok := false
	withTokenRefreshLock(func() {
		// Only metadata is shared. A waiting tab sends the browser's latest
		// cookie and gets its own access token; access/CSRF are never shared
		// through IndexedDB. A same-page refresh may already have completed.
		browserstore.Reload(authRefreshModeKey)
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
	return doCookieRefreshForAccount(ctx, pr, "")
}

// doCookieRefreshForAccount is also used by pre-sync boot, where accepting a
// cookie for another account would cross the page's fixed identity boundary.
func doCookieRefreshForAccount(ctx context.Context, pr prefs.Prefs, accountID string) bool {
	mode, err := newRefreshScope("cookie-server", "cookie-account", pr.ServerURL, currentPageOrigin())
	if err != nil || mode.Mode != authRefreshModeCookie {
		return false
	}
	endpoint := normalizedBackendEndpoint(pr.ServerURL)
	csrf := cookieSessionCSRF(pr)
	if csrf == "" {
		logSyncError("cookie token refresh failed", errors.New("session refresh: the same-origin CSRF cookie is unavailable"))
		return false
	}
	res, completed := fetchCookieRefresh(ctx, endpoint, csrf)
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
	if accountID != "" && strings.TrimSpace(res.userID) != accountID {
		logSyncError("cookie token refresh failed", errors.New("session refresh: cookie belongs to another account"))
		return false
	}
	expires := res.expiresIn
	if expires <= 0 {
		expires = oauthFallbackExpiresInSeconds
	}
	if accountID == "" {
		storeAuthTokenPair(backendrpc.TokenPairResponse{AccessToken: res.accessToken, ExpiresInSeconds: expires})
	} else {
		// Boot publishes only memory state. Its caller starts sync after the
		// fixed identity has been checked, so no watch is started here.
		browserstore.SetMemory(authAccessTokenKey, res.accessToken)
		browserstore.SetMemory(authExpiresInKey, fmt.Sprint(expires))
		dropSharedConn()
	}
	// storeAuthTokenPair only writes a refresh token when the pair carries
	// one; a cookie session must never have one in storage (it would flip
	// hasRotatableSession to the token-pair meaning), so be sure.
	lsRemove(authRefreshTokenKey)
	if strings.TrimSpace(res.csrf) != "" {
		lsSet(authCSRFKey, strings.TrimSpace(res.csrf))
	}
	if strings.TrimSpace(res.userID) != "" {
		if accountID == "" {
			setSignedInUserID(res.userID)
		} else {
			browserstore.SetMemory(syncUserIDKey, accountID)
		}
	}
	// The fetch Promise has settled, so the browser has installed the rotated
	// HttpOnly cookie. Access and CSRF remain page memory only.
	uistate.BumpAuthRevision()
	return true
}

// cookieSessionCSRF reads the current same-origin cookie, then page memory.
// A persisted preference is never a credential source.
func cookieSessionCSRF(pr prefs.Prefs) string {
	scope, err := newRefreshScope("cookie-server", "cookie-account", pr.ServerURL, currentPageOrigin())
	if err != nil || scope.Mode != authRefreshModeCookie {
		return ""
	}
	// Read on every request: another tab may have refreshed the cookie while
	// this tab waited for the lock. The server must expose this non-HttpOnly
	// double-submit cookie at Path=/ so the SPA can read it after a reload.
	doc := js.Global().Get("document")
	if doc.Truthy() && doc.Get("cookie").Type() == js.TypeString {
		for _, part := range strings.Split(doc.Get("cookie").String(), ";") {
			pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(pair) == 2 && pair[0] == "cashflux_csrf" && pair[1] != "" {
				browserstore.SetMemory(authCSRFKey, pair[1])
				return pair[1]
			}
		}
	}
	if v := strings.TrimSpace(lsGet(authCSRFKey)); v != "" {
		return v
	}
	return ""
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
	controller := js.Undefined()
	if constructor := js.Global().Get("AbortController"); constructor.Truthy() {
		controller = constructor.New()
		opts.Set("signal", controller.Get("signal"))
	}

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
		// A canceled fetch must settle before its refresh lock is released.
		// Otherwise the next tab could rotate while this response is still
		// installing a cookie. Modern browsers abort; older ones wait for
		// the outstanding exchange rather than release its lock early.
		if controller.Truthy() {
			controller.Call("abort")
		}
		<-done
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
