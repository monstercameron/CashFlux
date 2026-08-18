// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/backoff"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
	"github.com/monstercameron/CashFlux/internal/rpcworker"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

// importRemoteDataset replaces the local dataset with the server's copy WITHOUT
// discarding the OpenAI key held on this device.
//
// Every push to the backend goes through ExportJSONRedacted (see persist.go and
// the two upload sites below), which blanks Settings.OpenAIKey on the way out —
// deliberately, so the secret never leaves the machine. The consequence is that a
// server dataset can NEVER legitimately carry a key, so a straight ImportJSON —
// "replaces all data", Settings included — silently wipes the one the user typed
// here.
//
// That is invisible and it is total: on a hosted instance the key is entered,
// saved, and then erased by the next pull, after which the assistant and the
// Smart+ statement import both read an empty key and fail with messages about the
// network. Locally there is no sync, so the same build works perfectly — which is
// exactly how this hid.
//
// A key arriving in the incoming dataset still wins, so a user's own complete
// backup (manual ExportJSON keeps the key) restores as written.
func importRemoteDataset(app *appstate.App, dataset []byte) error {
	keep := strings.TrimSpace(app.Settings().OpenAIKey)
	if err := app.ImportJSON(dataset); err != nil {
		return err
	}
	if keep == "" {
		return nil
	}
	s := app.Settings()
	if strings.TrimSpace(s.OpenAIKey) != "" {
		return nil
	}
	s.OpenAIKey = keep
	if err := app.PutSettings(s); err != nil {
		app.Log().Error("restoring on-device OpenAI key after remote import", "err", err)
	}
	return nil
}

const syncMetaPrefix = "cashflux:sync-meta:"
const syncDeviceIDKey = "cashflux:sync-device-id"
const syncQueueKey = "cashflux:sync-queue"
const syncStatusKey = "cashflux:sync-status"

// syncConflictPrefix stores the LAST local dataset that lost an LWW conflict (the
// server held a newer snapshot), keyed by workspace. C309: this is the recoverable
// backup so a rejected local edit is never silently lost — the user can restore it
// from Settings → Cloud sync. One slot per workspace (the latest loser).
const syncConflictPrefix = "cashflux:sync-conflict:"

type syncMeta struct {
	UpdatedAt string `json:"updatedAt,omitempty"`
	Hash      string `json:"hash,omitempty"`
	Version   int64  `json:"version,omitempty"`
}

type queuedSyncMutation struct {
	// UserID is the account this snapshot was queued under (C696). It is part
	// of the queue's KEY, not a label: a workspace id alone does not say whose
	// workspace it is, and queued work that outlives an identity change would
	// otherwise be pushed under a token that cannot address it. Empty means the
	// entry predates user binding or was queued while signed out.
	UserID           string `json:"userId,omitempty"`
	WorkspaceID      string `json:"workspaceId"`
	Name             string `json:"name,omitempty"`
	Color            string `json:"color,omitempty"`
	Sort             int    `json:"sort,omitempty"`
	DeviceID         string `json:"deviceId,omitempty"`
	Dataset          string `json:"dataset"`
	ClientUpdatedAt  string `json:"clientUpdatedAt"`
	Hash             string `json:"hash"`
	LastAttemptError string `json:"lastAttemptError,omitempty"`
}

type syncStatus struct {
	State        string `json:"state"`
	Pending      int    `json:"pending,omitempty"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
	Message      string `json:"message,omitempty"`
	// AuthFailed is true when the server explicitly rejected the current
	// credentials (gRPC Unauthenticated), as opposed to a network/availability
	// failure. Settings' Cloud pane uses this to stop showing "Sign out" (which
	// implies an active session) once the saved token is known to be rejected —
	// a locally-saved token string is not the same thing as a working session.
	AuthFailed bool `json:"authFailed,omitempty"`
}

var syncPushMu sync.Mutex

// --- Token lifecycle (TODOS.md C423 client half, C424, C425, C427) ---
//
// A "Custom Sync" AuthService session (login/enroll/pairing) mints a rotating
// access/refresh token pair (backendrpc.TokenPairResponse) instead of the
// static self-host CASHFLUX_SERVER_TOKEN. These three keys hold that
// session's local state; a self-host static token never touches them, so
// effectiveServerToken transparently falls back to prefs.ServerToken when no
// rotated session exists.
const (
	authAccessTokenKey  = "cashflux:auth:access-token"
	authRefreshTokenKey = "cashflux:auth:refresh-token"
	authExpiresInKey    = "cashflux:auth:expires-in-seconds"
)

// proactiveRefreshTimer is the single in-flight countdown to the next
// proactive refresh (armed by storeAuthTokenPair). It is a relative timer
// (time.AfterFunc), never an absolute deadline compared against wall-clock
// time later — a device with a wrong clock cannot make it misfire either way
// (TODOS.md C423's correctness note).
var (
	proactiveRefreshMu    sync.Mutex
	proactiveRefreshTimer *time.Timer
)

// effectiveServerToken returns the bearer token every backend RPC should use:
// the locally rotated access token from a Custom Sync session when one
// exists, otherwise the static token from prefs (self-host token mode).
// One definition, in uistate, because the screens cannot import this package
// (internal/app imports internal/screens) and every Smart+ feature over there
// needs the same answer. See uistate.EffectiveServerToken.
func effectiveServerToken(pr prefs.Prefs) string {
	return uistate.EffectiveServerToken(pr.ServerToken)
}

// hasRotatableSession reports whether a Custom Sync refresh token is on
// hand — the signal that this device's credential can be refreshed/degraded,
// as opposed to a static self-host token, which never rotates and is left
// entirely alone by this machinery.
func hasRotatableSession() bool {
	return strings.TrimSpace(lsGet(authRefreshTokenKey)) != ""
}

// dropSharedConn tells services.wasm to cancel active operations and close its
// connection pool. Call it whenever the endpoint or bearer identity changes so
// no subsequent RPC can reuse a tunnel authenticated as the previous session.
func dropSharedConn() {
	if client := rpcworker.Default(); client != nil {
		client.ResetSession()
	}
}

// isAuthError reports whether err is the backend rejecting the bearer token
// (codes.Unauthenticated) — the trigger for the reactive refresh fallback,
// as opposed to any other RPC failure (network, quota, validation, ...),
// which a refresh cannot fix and retrying would just waste a round trip.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	return rpcprotocol.IsCode(err, "Unauthenticated")
}

// invokeAuthed calls method in services.wasm and, on an Unauthenticated failure,
// performs the C423 reactive fallback: exactly one RefreshToken attempt via
// refreshAccessToken, then — only if that succeeded — resets the worker connection
// pool and retries the original call exactly once under the refreshed token.
func invokeAuthed(ctx context.Context, pr prefs.Prefs, method string, req, resp any) error {
	err := invokeWorkerRPC(ctx, pr.ServerURL, effectiveServerToken(pr), method, req, resp)
	if !isAuthError(err) {
		return err
	}
	if !refreshAccessToken(ctx, pr) {
		return err
	}
	fresh := uistate.LoadPrefs().Normalize()
	return invokeWorkerRPC(ctx, fresh.ServerURL, effectiveServerToken(fresh), method, req, resp)
}

// refreshAccessToken performs (or, if another tab wins the race, waits for
// and reuses the result of) a single RefreshToken round trip, guarded by the
// cross-tab Web Locks guard (TODOS.md C424) so concurrently open tabs never
// race the server for a refresh. It returns false when there is no
// rotatable session to refresh, or the refresh attempt failed.
func refreshAccessToken(ctx context.Context, pr prefs.Prefs) bool {
	startingRefresh := strings.TrimSpace(lsGet(authRefreshTokenKey))
	if startingRefresh == "" {
		return false
	}
	ok := false
	withTokenRefreshLock(func() {
		// Re-read from storage before deciding. The guard below compares against
		// what ANOTHER tab may have written while we waited for the lock, and
		// browserstore's cache is per-tab and frozen at boot — so without this
		// the comparison could only ever see our own starting value, always
		// concluded "unchanged", and replayed a refresh token the other tab had
		// already consumed. The server treats a replayed refresh token as a
		// compromise signal and revokes the whole session family, so this
		// omission could sign the user out of every device simply for having a
		// second tab open (2026-08-18).
		browserstore.Reload(authRefreshTokenKey, authAccessTokenKey)
		// Reuse without replaying: another tab may have already refreshed
		// while we waited for the lock. A refresh token is single-use —
		// replaying our now-stale copy would trip the server's reuse/
		// compromise detection and revoke the WHOLE session family. If the
		// stored refresh token has moved on, there is already fresh state
		// to use; nothing left for us to do.
		if strings.TrimSpace(lsGet(authRefreshTokenKey)) != startingRefresh {
			ok = true
			return
		}
		ok = doRefreshAccessToken(ctx, pr, startingRefresh)
	})
	return ok
}

// doRefreshAccessToken makes the actual AuthService.RefreshToken call. It
// must run only while holding the token-refresh lock (via refreshAccessToken)
// so it is never invoked twice concurrently for the same session.
//
// RefreshToken/Logout are exempt from the server's auth interceptor (see
// authinterceptor_skip.go), so the tunnel dial below only needs SOME
// non-empty token to satisfy syncbridge's handshake requirement — it need
// not itself be valid, which matters because this is exactly the call made
// when the access token has expired.
func doRefreshAccessToken(ctx context.Context, pr prefs.Prefs, refreshToken string) bool {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	token := effectiveServerToken(pr)
	if token == "" {
		token = "refresh"
	}
	var resp backendrpc.TokenPairResponse
	err := invokeWorkerRPC(dialCtx, pr.ServerURL, token, backendrpc.MethodAuthRefreshToken,
		backendrpc.RefreshTokenRequest{RefreshToken: refreshToken}, &resp)
	if err != nil {
		if isAuthError(err) {
			// C427 graceful degrade: the refresh token itself is
			// expired/revoked, not just the access token. There is no
			// credential left to recover — drop to local-only silently. No
			// error dialog, no data loss: the encrypted dataset on disk
			// stays fully usable, just no longer synced.
			degradeToLocalOnly()
		} else {
			logSyncError("token refresh failed", err)
		}
		return false
	}
	storeAuthTokenPair(resp)
	return true
}

// storeAuthTokenPair persists a freshly (re)issued token pair and rearms the
// proactive countdown. C425: an already-open watch stream authenticated
// with the OLD token has no reason to keep running under it — cycle it
// through the existing reconnect/backoff machinery (stopBackendWatch/
// startBackendWatch, unchanged) as one more trigger, so it re-subscribes
// with the new access token right away instead of running until it
// eventually gets rejected on its own.
func storeAuthTokenPair(pair backendrpc.TokenPairResponse) {
	if strings.TrimSpace(pair.AccessToken) != "" {
		lsSet(authAccessTokenKey, pair.AccessToken)
	}
	if strings.TrimSpace(pair.RefreshToken) != "" {
		lsSet(authRefreshTokenKey, pair.RefreshToken)
	}
	if pair.ExpiresInSeconds > 0 {
		lsSet(authExpiresInKey, strconv.FormatInt(pair.ExpiresInSeconds, 10))
		armProactiveRefresh(pair.ExpiresInSeconds)
	}
	// services.wasm keys its pooled connections by endpoint and bearer. Explicitly
	// reset on every rotation so old credentialed sockets and streams cannot linger.
	dropSharedConn()
	stopBackendWatch()
	startBackendWatch()
}

// armProactiveRefresh (re)starts the local countdown to the next proactive
// refresh, firing at ~80% of the server-issued lifetime (TODOS.md C423): a
// pure relative time.AfterFunc duration, derived only from the
// server-supplied expiresInSeconds — never an absolute expiry timestamp
// compared against time.Now() later, which a skewed device clock could get
// wrong in either direction (refreshing needlessly early, or never firing
// because "now" never appears to reach a bad deadline).
func armProactiveRefresh(expiresInSeconds int64) {
	if expiresInSeconds <= 0 {
		return
	}
	d := time.Duration(float64(expiresInSeconds) * 0.8 * float64(time.Second))
	if d <= 0 {
		return
	}
	proactiveRefreshMu.Lock()
	defer proactiveRefreshMu.Unlock()
	if proactiveRefreshTimer != nil {
		proactiveRefreshTimer.Stop()
	}
	proactiveRefreshTimer = time.AfterFunc(d, func() {
		pr := uistate.LoadPrefs().Normalize()
		if !pr.BackendActive() || !hasRotatableSession() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		refreshAccessToken(ctx, pr)
	})
}

// stopProactiveRefreshTimer cancels the pending countdown, if any — part of
// dropping to local-only (degradeToLocalOnly) and of tearing down sync
// (stopBackendWatch's caller sites), so a stale timer never fires a refresh
// attempt for a session that no longer exists.
func stopProactiveRefreshTimer() {
	proactiveRefreshMu.Lock()
	defer proactiveRefreshMu.Unlock()
	if proactiveRefreshTimer != nil {
		proactiveRefreshTimer.Stop()
		proactiveRefreshTimer = nil
	}
}

// degradeToLocalOnly is the C427 graceful-degrade path: the refresh token
// itself came back rejected (expired/revoked), so there is no credential
// left worth keeping. It clears every locally stored credential (the
// rotated session AND, since a rotatable session implies this was never the
// static self-host token, the prefs-level ServerToken/BackendDisabled too),
// tears down the watch, and settles the sync chip on "local" — silently, no
// error dialog. The encrypted dataset already on disk is untouched and
// fully usable; only cloud sync stops.
// clearAuthSession tears down every locally held credential for the current
// Custom Sync session: the rotated access/refresh pair, the refresh countdown,
// and the live watch. It deliberately touches no prefs — callers decide whether
// this is a user-initiated sign-out (toggle stays on, ready for a new code) or
// the C427 forced degrade (backend switched off entirely).
//
// Sign-out has to come through here. Clearing only prefs.ServerToken stops sync
// (BackendActive goes false) but leaves a WORKING refresh token sitting in
// storage — a credential the user believes they just revoked, recoverable by
// anything that can read this origin's storage.
func clearAuthSession() {
	lsRemove(authAccessTokenKey)
	lsRemove(authRefreshTokenKey)
	lsRemove(authExpiresInKey)
	// Forget WHO the session belonged to as well. Leaving it behind meant the
	// next person to sign in on this browser inherited the previous account's
	// id until the page happened to reload, and their edits were queued under
	// it. The QUEUE is deliberately not cleared: unpushed work is the user's
	// data, and signing out is not a request to discard it.
	clearSignedInUserID()
	stopProactiveRefreshTimer()
	stopBackendWatch()
	// The shared tunnel is authenticated as the session that just ended. Leaving it
	// open would keep a live, credentialed socket to the server after sign-out.
	dropSharedConn()
}

func degradeToLocalOnly() {
	clearAuthSession()
	pr := uistate.LoadPrefs()
	pr.ServerToken = ""
	pr.BackendDisabled = true
	uistate.PersistPrefs(pr.Normalize())
	setSyncStatus(syncStatus{State: "local"})
}

// restoreTokenLifecycleOnBoot rearms the proactive refresh countdown for a
// session that already had a rotated token pair when this page loaded (e.g.
// a reload mid-session). It restarts the countdown from the FULL
// server-issued duration rather than trying to account for time already
// elapsed in a prior page load — consistent with never trusting a stored
// wall-clock deadline; the reactive fallback covers the (rare) case where
// that restarted countdown undershoots and the access token expires before
// it fires.
func restoreTokenLifecycleOnBoot() {
	if !hasRotatableSession() {
		return
	}
	raw := strings.TrimSpace(lsGet(authExpiresInKey))
	if raw == "" {
		return
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds <= 0 {
		return
	}
	armProactiveRefresh(seconds)
}

func startBackendSync() {
	// Don't wire up auto-sync (or its visibility/focus/online listeners) when the
	// backend is off or unconfigured — otherwise the app dials a websocket on load
	// and surfaces connection errors the user can't act on (C81 follow-up).
	if !uistate.LoadPrefs().Normalize().BackendActive() {
		return
	}
	restoreTokenLifecycleOnBoot()
	// Credentials rotate, and they rotate in whichever tab happened to get there
	// first. Every other tab is left holding a bearer that is about to stop
	// working, a socket authenticated with it, and possibly a sticky "your
	// credentials were rejected" flag from the 401 it earned on the way. React to
	// the change instead of waiting for a reload (2026-08-18).
	uistate.WatchAuthAcrossTabs(func() {
		// A newer credential exists, so any earlier rejection is stale. Leaving
		// AuthFailed set is what kept a recovered tab insisting the user had to
		// sign in again.
		if st := loadSyncStatus(); st.AuthFailed {
			st.AuthFailed = false
			st.Message = ""
			setSyncStatus(st)
		}
		// The pooled connections are keyed by endpoint AND bearer; the live ones
		// are authenticated as the credential that just got replaced.
		dropSharedConn()
		stopBackendWatch()
		startBackendWatch()
		flushBackendSyncQueue()
	})
	// C696: learn which account this session belongs to before anything is
	// pushed. Queued work written while signed out is adopted by the current
	// account; work belonging to a DIFFERENT one is left alone and surfaces as
	// a decision rather than as an endless retry. Off the critical path — a
	// slow or unreachable /v1/me must not delay boot.
	go func() {
		pr := uistate.LoadPrefs().Normalize()
		adoptIdentityForSync(pr.ServerURL, effectiveServerToken(pr))
	}()
	wireSyncLifecycleListeners()
	// C696: re-resolve WHO this session is on every restart, not just at boot.
	// persistAuthSession (register / login / redeem-pairing) funnels through
	// here without reloading the page, so a boot-only identity would stay frozen
	// across a sign-out and sign-in — and the next edit would be stamped with,
	// and pushed as, the previous account. Found by adversarial review,
	// 2026-08-17.
	go func() {
		pr := uistate.LoadPrefs().Normalize()
		adoptIdentityForSync(pr.ServerURL, effectiveServerToken(pr))
	}()
	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
	mergeRemoteWorkspaces()
	startBackendWatch()
}

// syncListenersWired guards the one-time registration of the page lifecycle
// listeners, so enabling the backend at RUNTIME (not just boot) gets the same
// visibility/focus/online/offline reconciliation as a fresh load — without
// double-registering on repeated toggles.
var syncListenersWired bool

// wireSyncLifecycleListeners registers the visibility/focus/online/offline
// listeners that trigger reconciling pulls and reflect connectivity. Idempotent:
// the guard means it runs at most once whether reached from boot or a runtime
// enable.
func wireSyncLifecycleListeners() {
	if syncListenersWired {
		return
	}
	syncListenersWired = true
	cb := js.FuncOf(func(js.Value, []js.Value) any {
		if js.Global().Get("document").Get("visibilityState").String() == "visible" {
			wakeSync()
		}
		return nil
	})
	// Both events fire when a tab is switched back to, and they are registered with
	// the SAME callback — so every tab switch used to trigger two flushes and two
	// pulls, each of which dials its own WebSocket. wakeSync coalesces them.
	js.Global().Call("addEventListener", "visibilitychange", cb)
	js.Global().Call("addEventListener", "focus", cb)
	js.Global().Call("addEventListener", "online", js.FuncOf(func(js.Value, []js.Value) any {
		flushBackendSyncQueue()
		return nil
	}))
	// C323: reflect going offline immediately. Without this only "online" was wired,
	// so a dropped connection left the chip on its last (often "synced") state until
	// the next failed dial. Mark any pending work as queued/offline right away.
	js.Global().Call("addEventListener", "offline", js.FuncOf(func(js.Value, []js.Value) any {
		st := loadSyncStatus()
		st.State = "offline"
		st.Pending = pendingSyncCount()
		setSyncStatus(st)
		return nil
	}))
}

// pushBlockedReason reports why this device must not send its local dataset right
// now, or "" when pushing is safe. Both cases protect the SERVER's copy from being
// overwritten by local data that isn't the user's real data — a hazard with teeth,
// because pushes are last-write-wins on a client-supplied timestamp, so a device
// holding demo data always wins against a device holding a year of real records.
//
//   - The seeded sample. A browser signing in for the first time boots the demo
//     dataset BEFORE it ever contacts the server. Nothing of the user's is lost by
//     never uploading it, and uploading it can destroy everything: sign in on a new
//     browser, have the pull fail or defer, touch anything, and the sample would go
//     up with time.Now() and win.
//   - A snapshot we could not read. State "locked" means the server holds an
//     encrypted dataset this device has no passcode for. It is still the account's
//     real data; overwriting it with whatever this device happens to hold, because
//     we could not decrypt it, is the worst possible response to that situation.
//
// Both clear themselves: personalising the sample drops the flag, and unlocking
// (or setting the matching passcode) re-pulls and clears "locked".
func pushBlockedReason() string {
	if uistate.SampleActive() {
		return "local dataset is the seeded sample"
	}
	if loadSyncStatus().State == "locked" {
		return "server snapshot is encrypted and this device cannot read it"
	}
	return ""
}

// yieldToBrowser hands the main thread back to JavaScript for one macrotask.
//
// Go on wasm is SINGLE-THREADED: `go func()` does not move work to another thread,
// because there isn't one. A goroutine runs on the same thread as the renderer,
// inside whatever JS callback the scheduler happened to be in, and only a BLOCKING
// operation returns control to the event loop. CPU-bound work — a SQLite import, a
// dataset export, JSON of any size — therefore freezes the page for exactly as long
// as it runs, no matter how many goroutines it is spread across.
//
// Blocking on a channel fed by setTimeout is the one portable way to say "let the
// browser breathe": the Go scheduler finds every goroutine parked, returns to JS,
// and the browser gets to paint a frame and — critically here — deliver the
// WebSocket open/message events that pending RPCs are waiting on. Without a yield
// between the dial and the heavy work that follows it, a sync can starve the very
// connection it is trying to use, then time out blaming the network.
//
// It yields to requestIdleCallback rather than setTimeout, because those two mean
// different things. setTimeout(0) says "run me in the next task", which can still be
// ahead of a frame the browser has already queued — so sync work resumes with a
// render pending behind it. requestIdleCallback says "run me when the browser has
// finished everything it wanted to do this frame, and has time left over". That is
// the scheduling contract this app needs: RENDERING WINS, always. Networking and the
// crypto/serialisation around it are background reconciliation, and no part of them
// is worth a dropped frame.
//
// The idleTimeout cap is the safety valve. A page under permanent load never goes
// idle, and without a deadline sync would simply stop happening rather than merely
// yielding — so after that long the callback runs regardless. It bounds the worst
// case at "sync is late", never "sync is dead".
//
// Not free: each call costs at least a frame. Use it between phases of long work,
// never inside a tight loop.
func yieldToBrowser() {
	done := make(chan struct{}, 1)
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		cb.Release()
		done <- struct{}{}
		return nil
	})
	if ric := js.Global().Get("requestIdleCallback"); !ric.IsUndefined() && ric.Type() == js.TypeFunction {
		js.Global().Call("requestIdleCallback", cb, map[string]any{"timeout": idleTimeoutMS})
	} else {
		// Safari and friends: one macrotask is the best available approximation.
		js.Global().Call("setTimeout", cb, 0)
	}
	<-done
}

// idleTimeoutMS bounds how long a yield will wait for a quiet frame before running
// anyway. Long enough that a busy burst — a page transition, a big list rendering —
// finishes first; short enough that a user who leaves the app under load still gets
// their data reconciled while they are looking at it.
const idleTimeoutMS = 1500

// wakeSync reconciles with the server after the page becomes usable again,
// collapsing the burst of events that means "the user came back".
//
// visibilitychange and focus both fire on a tab switch, and each previously ran a
// flush AND a pull — and every one of those dials its OWN WebSocket, because each
// sync function opens a connection and closes it again. A few tab switches
// therefore produced dozens of connections, all racing, all with 15-20s deadlines.
// That is how a server log ends up with 279 accepted connections and the client
// reports "WebSocket is closed before the connection is established": the sockets
// connect fine, but the main thread is too busy to process their open events before
// the dial contexts expire, and each expiry schedules more retries.
//
// The window only has to outlast the burst, not rate-limit real use: a genuine
// second visit a second later still reconciles.
func wakeSync() {
	wakeMu.Lock()
	if time.Since(lastWakeAt) < wakeCoalesceWindow {
		wakeMu.Unlock()
		return
	}
	lastWakeAt = time.Now()
	wakeMu.Unlock()

	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
}

// wakeCoalesceWindow is how long after one wake-up another is treated as the same
// event. Comfortably longer than the gap between visibilitychange and focus, far
// shorter than any interval a person would notice.
const wakeCoalesceWindow = 750 * time.Millisecond

var (
	wakeMu     sync.Mutex
	lastWakeAt time.Time
)

// pullInFlight guards against stacking pulls. Every trigger — a wake-up, a watch
// event, a sign-in, a boot — previously started its own goroutine and its own
// dial, even while an identical pull was already running. They all fetch the same
// workspace, so the extra ones cost a connection each and change nothing.
var (
	pullMu       sync.Mutex
	pullInFlight bool
)

func pushActiveWorkspaceToBackend(dataset []byte, updatedAt time.Time) {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	if reason := pushBlockedReason(); reason != "" {
		if app := appstate.Default; app != nil {
			app.Log().Info("backend sync push skipped to protect the server copy", "reason", reason)
		}
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	hash := datasetHash(dataset)
	meta := loadSyncMeta(w.ID)
	if meta.Hash == hash {
		return
	}
	enqueueSyncMutation(queuedSyncMutation{
		WorkspaceID:     w.ID,
		Name:            w.Name,
		Color:           w.Color,
		Sort:            workspaceSort(r, w.ID),
		DeviceID:        syncDeviceID(),
		Dataset:         string(dataset),
		ClientUpdatedAt: updatedAt.UTC().Format(time.RFC3339Nano),
		Hash:            hash,
	})
	flushBackendSyncQueue()
}

func requestBackendSyncNow() {
	// Seed the queue with the CURRENT dataset before flushing. The queue is only ever
	// filled by an autosave (pushActiveWorkspaceToBackend), so on a device that has
	// signed in but not edited anything since, "Sync now" used to find an empty queue,
	// report "Synced", and upload precisely nothing — the chip claiming success for a
	// server that never received a byte. The hash guard inside
	// pushActiveWorkspaceToBackend keeps this a no-op when the server already holds
	// this exact dataset, so pressing Sync now repeatedly stays cheap.
	if app := appstate.Default; app != nil {
		if dataset, err := app.ExportJSONRedacted(); err == nil {
			pushActiveWorkspaceToBackend(dataset, time.Now().UTC())
		} else {
			logSyncError("backend sync export failed", err)
		}
	}
	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
}

// forceBackendResyncActiveWorkspace re-pushes the active workspace's current
// dataset even though its plaintext content is unchanged. It exists for the
// encryption-mode toggle: enabling/disabling the passcode lock changes the FORM the
// server should store (plaintext↔envelope) but not the plaintext hash, so the normal
// dedup guard in pushActiveWorkspaceToBackend would skip the push. Clearing the
// sync-meta hash forces the next push through; prepareBackendSyncDataset then
// (re)encrypts or (re)plaintexts per the now-current mode.
func forceBackendResyncActiveWorkspace() {
	if !uistate.LoadPrefs().Normalize().BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	meta := loadSyncMeta(w.ID)
	meta.Hash = ""
	saveSyncMeta(w.ID, meta)
	app := appstate.Default
	if app == nil {
		return
	}
	if redacted, err := app.ExportJSONRedacted(); err == nil {
		pushActiveWorkspaceToBackend(redacted, time.Now().UTC())
	}
}

// forceBackendSeedActiveWorkspace is the empty-account variant of
// forceBackendResyncActiveWorkspace. Sync metadata lives on the device and a
// user can sign into a brand-new server account while that workspace still
// carries an UpdatedAt from a previous account. Once GetWorkspace has proved
// this account has no row, clear that stale server identity as well as the
// content hash so flushBackendSyncQueue performs its create-before-blobs seed
// path instead of sending artifact blobs to a workspace that does not exist.
func forceBackendSeedActiveWorkspace() {
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	meta := loadSyncMeta(w.ID)
	meta.Hash = ""
	meta.UpdatedAt = ""
	meta.Version = 0
	saveSyncMeta(w.ID, meta)
	forceBackendResyncActiveWorkspace()
}

func flushBackendSyncQueue() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	go func() {
		yieldToBrowser() // rendering first: never start a push inside a frame
		defer timePhase("flush.total")()
		syncPushMu.Lock()
		defer syncPushMu.Unlock()
		queue := loadSyncQueue()
		if len(queue) == 0 {
			setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			return
		}
		// C696: refuse to start a push that cannot succeed. `workspace not
		// found` means the signed-in account does not own this workspace, and
		// no amount of retrying changes that — the old loop re-attempted it on
		// every tick, wake and watch event, which is why the settings page
		// could sit at "1 change waiting to upload" indefinitely with nothing
		// on either side about to change. The work is KEPT; what stops is the
		// pointless retry, replaced by a decision the user can act on.
		// Only an authoritative refusal stops the queue. An unknown identity does
		// not — see uploadDecision; gating on it broke every deployment where the
		// identity lookup does not answer.
		if uploadDecision() == syncstate.UploadRebind {
			refreshRebindStatus()
			return
		}
		// Push only what this account owns. Another identity's stranded snapshot
		// stays queued until the user decides about it (C696).
		queue = pushableQueue(queue)
		if len(queue) == 0 {
			refreshRebindStatus()
			return
		}
		setSyncStatus(syncStatus{State: "syncing", Pending: len(queue)})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		for _, item := range queue {
			maybeYield() // a queue of any length must not cost the user a click
			// A first-ever push has to create the workspace row BEFORE any artifact
			// blob is uploaded. UploadBlob deliberately refuses a blob for a workspace
			// the caller doesn't own yet (storage attribution and cross-tenant read
			// scoping both hang off that check), and PutWorkspace is the only thing
			// that creates the row — so a dataset with attachments used to deadlock on
			// its very first sync: every blob rejected NotFound, the push abandoned
			// before PutWorkspace, and therefore the row that would have unblocked it
			// never written. This put carries no Dataset, so the server creates the row
			// and skips the snapshot write; the real push follows immediately below. It
			// runs only until this workspace has synced once.
			if meta := loadSyncMeta(item.WorkspaceID); strings.TrimSpace(meta.UpdatedAt) == "" && strings.TrimSpace(item.Name) != "" {
				var ensured backendrpc.PutWorkspaceResponse
				if err := invokeAuthed(ctx, pr, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
					Workspace: backendrpc.Workspace{
						ID:       item.WorkspaceID,
						Name:     item.Name,
						Color:    item.Color,
						Sort:     item.Sort,
						DeviceID: item.DeviceID,
					},
					ClientUpdatedAt: item.ClientUpdatedAt,
				}, &ensured); err != nil {
					item.LastAttemptError = err.Error()
					upsertQueuedSyncMutation(item)
					setSyncStatus(syncStatus{State: "error", Pending: pendingSyncCount(), Message: customSyncErrorMessage(err, "sync failed"), AuthFailed: isAuthError(err)})
					logSyncError("backend workspace create failed", err)
					return
				}
				// Creating the row stamps it with the SERVER's clock, which is later than
				// the moment this mutation was queued — so the real push below would lose
				// the LWW comparison against a row it just created itself, and the dataset
				// would be backed up as a "conflict" against nothing. Carry the server's
				// timestamp forward so the push that follows is contemporaneous with it.
				// Only when the create was accepted: a rejection means the server really
				// does hold something newer (another device got there first), and that
				// push SHOULD lose — leaving the timestamp alone keeps that intact.
				if ensured.Accepted && strings.TrimSpace(ensured.UpdatedAt) != "" {
					item.ClientUpdatedAt = ensured.UpdatedAt
				}
			}
			// The connection is up; let the browser deliver its events and paint
			// before this goroutine spends the thread on import/encrypt/export.
			yieldToBrowser()
			dataset, err := prepareBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), item.WorkspaceID, []byte(item.Dataset))
			if err != nil {
				item.LastAttemptError = err.Error()
				upsertQueuedSyncMutation(item)
				setSyncStatus(syncStatus{State: "error", Pending: pendingSyncCount(), Message: customSyncErrorMessage(err, "artifact blob upload failed"), AuthFailed: isAuthError(err)})
				logSyncError("backend artifact blob upload failed", err)
				return
			}
			var resp backendrpc.PutWorkspaceResponse
			err = invokeAuthed(ctx, pr, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
				Workspace: backendrpc.Workspace{
					ID:       item.WorkspaceID,
					Name:     item.Name,
					Color:    item.Color,
					Sort:     item.Sort,
					DeviceID: item.DeviceID,
				},
				Dataset:         dataset,
				ClientUpdatedAt: item.ClientUpdatedAt,
			}, &resp)
			if err != nil {
				item.LastAttemptError = err.Error()
				upsertQueuedSyncMutation(item)
				reason := customSyncErrorMessage(err, "sync failed")
				// A workspace this account cannot address is terminal for that
				// binding, not a transient failure. Say so once and stop, so the
				// user gets a decision instead of an indefinitely spinning queue.
				if syncstate.IsWorkspaceNotFound(reason) {
					setSyncStatus(syncStatus{State: syncStateRebind, Pending: pendingSyncCount(), Message: reason})
					logSyncError("backend sync push refused: workspace belongs to another account", err)
					return
				}
				setSyncStatus(syncStatus{State: "error", Pending: pendingSyncCount(), Message: reason, AuthFailed: isAuthError(err)})
				logSyncError("backend sync push failed", err)
				return
			}
			if !resp.Accepted && resp.Workspace.DeviceID == syncDeviceID() {
				// We lost last-write-wins to a snapshot THIS device wrote. That is not
				// a conflict — there is no other writer to protect — it is an artifact
				// of when the timestamp was taken: ClientUpdatedAt is stamped when the
				// edit is QUEUED, while the server stamps its own clock when the push
				// LANDS. Any delay in between (a retry after a failed blob upload, an
				// offline queue draining, a slow link) leaves the queued edit looking
				// older than the snapshot it is meant to supersede, so the device's own
				// newer data gets rejected and filed as a conflict against itself.
				// Retry once, forcing past the comparison. Scoped strictly to our own
				// device id: a snapshot written by any OTHER device still goes through
				// the normal LWW path below, backup and all.
				var forced backendrpc.PutWorkspaceResponse
				forceErr := invokeAuthed(ctx, pr, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
					Workspace: backendrpc.Workspace{
						ID:       item.WorkspaceID,
						Name:     item.Name,
						Color:    item.Color,
						Sort:     item.Sort,
						DeviceID: item.DeviceID,
					},
					Dataset:         dataset,
					ClientUpdatedAt: item.ClientUpdatedAt,
					Force:           true,
				}, &forced)
				if forceErr == nil && forced.Accepted {
					resp = forced
					if app := appstate.Default; app != nil {
						app.Log().Info("backend sync re-pushed past our own newer snapshot", "workspace", item.WorkspaceID)
					}
				}
			}
			if !resp.Accepted {
				// LWW resolution: the server holds a newer snapshot, so this push lost.
				// C309: do NOT silently drop the local edit. Before removing it from the
				// active queue (which must happen, or it would re-push and re-lose every
				// cycle — an infinite conflict loop), stash the rejected local dataset to
				// a recoverable per-workspace backup so the user can restore it. Then tell
				// them plainly (§7.11) and pull the newer server copy so the UI is current.
				saveConflictBackup(item)
				removeQueuedSyncMutation(item.WorkspaceID, item.Hash)
				setSyncStatus(syncStatus{State: "conflict", Pending: pendingSyncCount(), Message: "newer server snapshot available"})
				uistate.PostNotice(uistate.T("sync.conflictBackedUp"), false)
				if app := appstate.Default; app != nil {
					app.Log().Warn("backend sync push rejected; local edit backed up, newer server snapshot pulled", "workspace", item.WorkspaceID)
				}
				continue
			}
			// Accepted: only now is it safe to drop the local mutation from the queue.
			removeQueuedSyncMutation(item.WorkspaceID, item.Hash)
			saveSyncMeta(item.WorkspaceID, syncMeta{UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: item.Hash})
			// The server took it — the one moment worth announcing in the top bar.
			// Deliberately NOT the setSyncStatus below, which also fires for a flush
			// that found an empty queue and uploaded nothing.
			noteSyncActivity()
		}
		setSyncStatus(syncStatus{State: "synced", Pending: pendingSyncCount(), LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	}()
}

// watchMu guards watchCancel, the cancel func for the single live watch loop.
// A cancelable, restartable watch is what makes runtime pref changes take effect
// without a full page reload: toggling the backend off cancels the loop, and
// changing the server URL/token restarts it against the new endpoint.
var (
	watchMu     sync.Mutex
	watchCancel context.CancelFunc
)

// startBackendWatch (re)starts the workspace watch loop. It cancels any prior
// loop first so there is never more than one, and starts a fresh one only when
// the backend is active — so it doubles as the restart primitive after a pref
// change. Safe to call repeatedly.
func startBackendWatch() {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
		watchCancel = nil
	}
	if !uistate.LoadPrefs().Normalize().BackendActive() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	watchCancel = cancel
	go runBackendWatch(ctx)
}

// stopBackendWatch cancels the live watch loop (if any), tearing down its stream
// and connection promptly (the loop's ctx-bound RecvMsg unblocks on cancel).
func stopBackendWatch() {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
		watchCancel = nil
	}
}

// restartBackendSync applies a runtime backend pref change (toggle / URL / token)
// without a reload: it stops the watch when the backend is now off, or flushes,
// pulls, and restarts the watch against the fresh prefs when it is on. Callers are
// the Sync page and Settings → Cloud toggles.
func restartBackendSync() {
	if !uistate.LoadPrefs().Normalize().BackendActive() {
		stopBackendWatch()
		// Backend switched off: nothing should hold a socket open to it.
		dropSharedConn()
		return
	}
	wireSyncLifecycleListeners()
	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
	mergeRemoteWorkspaces()
	startBackendWatch()
}

// runBackendWatch is the watch loop body: it dials the bridge, subscribes, and
// reads live events until cancelled, reconnecting with capped backoff+jitter. It
// re-reads prefs each iteration and binds every RPC to ctx, so a pref change
// (via restartBackendSync) or a disable takes effect immediately rather than at
// the next page reload.
func runBackendWatch(ctx context.Context) {
	// C322: exponential backoff + jitter (2s→120s cap) instead of fixed
	// 10s/3s sleeps, so a flapping network doesn't hammer the backend and many
	// clients don't reconnect in lockstep.
	const baseDelay, capDelay, jitterFrac = 2 * time.Second, 120 * time.Second, 0.3
	// healthyAfter is how long a stream must stay up (absent any received
	// message) to count as a healthy connection worth resetting the backoff for
	// — the thrash guard against a stream that opens then instantly errors.
	const healthyAfter = 30 * time.Second
	attempt := 0
	// sleepBackoff waits out the backoff, but wakes immediately if the watch is
	// cancelled — returns false when cancelled so the loop exits promptly.
	sleepBackoff := func() bool {
		d := backoff.Jitter(backoff.Delay(attempt, baseDelay, capDelay), jitterFrac, rand.Float64())
		select {
		case <-time.After(d):
			attempt++
			return true
		case <-ctx.Done():
			return false
		}
	}
	// firstConnect skips the reconcile pull on the very first successful
	// subscribe, because startBackendSync already pulled at boot — only
	// RE-connects need to reconcile the gap.
	firstConnect := true
	for {
		if ctx.Err() != nil {
			return
		}
		// Re-read prefs each iteration so a runtime URL/token change is picked up on
		// the next (re)connect, and a disable exits the loop.
		pr := uistate.LoadPrefs().Normalize()
		if !pr.BackendActive() {
			return
		}
		stream, err := openWorkerRPCStream(ctx, pr.ServerURL, effectiveServerToken(pr),
			backendrpc.MethodSyncWatchWorkspaces, backendrpc.WatchWorkspacesRequest{IncludeDeleted: true})
		if err != nil && isAuthError(err) {
			// Reactive fallback (C423) for the watch stream: one refresh
			// attempt now, so the NEXT reconnect (right below, via the
			// normal backoff loop) dials with a live token instead of
			// repeating the same failure until a proactive refresh happens
			// to land. A successful refresh here also resets the backoff
			// via the attempt=0 below, so the reconnect is prompt, not
			// delayed by whatever backoff this failed attempt earned.
			if refreshAccessToken(ctx, pr) {
				attempt = 0
			}
		}
		if err == nil {
			// Reconcile on every RE-subscribe: the server streams only FUTURE
			// events and silently drops on a full send buffer, so a client that
			// was briefly disconnected (or whose buffer overflowed) would miss
			// other devices' changes with no signal. Pulling the active workspace
			// now closes that gap — the push stream alone is best-effort.
			if !firstConnect {
				pullActiveWorkspaceFromBackend(true)
			}
			firstConnect = false
			connectedAt := time.Now()
			received := readBackendWatch(stream)
			// Reset the backoff only when the stream proved healthy (delivered a
			// message or stayed up long enough); an immediate error keeps the
			// backoff climbing instead of reconnecting at the floor forever.
			if syncstate.ShouldResetBackoff(received, time.Since(connectedAt), healthyAfter) {
				attempt = 0
			}
		} else {
			logSyncError("backend sync watch failed", err)
		}
		if !sleepBackoff() {
			return
		}
	}
}

// readBackendWatch reads live workspace events until the stream ends, pulling the
// active workspace whenever another device changes it. It returns whether it
// received at least one event, which the reconnect loop uses as a health signal
// (a stream that delivered data was healthy even if it was short-lived).
func readBackendWatch(stream *rpcworker.Stream) (received bool) {
	for {
		var event backendrpc.WatchWorkspacesResponse
		if err := stream.Recv(&event); err != nil {
			logSyncError("backend sync watch closed", err)
			return received
		}
		received = true
		if strings.TrimSpace(event.Workspace.ID) == "" || event.Workspace.DeviceID == syncDeviceID() {
			continue
		}
		r := loadRegistry()
		active, ok := r.Active()
		if ok && active.ID == event.Workspace.ID {
			pullActiveWorkspaceFromBackend(true)
		}
	}
}

// mergeRemoteWorkspaces adds any workspace this account owns on the server that
// this device has never heard of. Without it a device signing in fresh hydrates
// exactly ONE workspace — pullActiveWorkspaceFromBackend is singular, and nothing
// in the client had ever called ListWorkspaces — so a household with three
// workspaces silently became a household with one on every new browser.
//
// It only touches the REGISTRY, never the active dataset: each added workspace
// gets an explicitly empty bundle, so switching to it boots a clean slate that
// the boot-time pull then fills from the server. (An empty bundle rather than no
// bundle for the same reason createWorkspace does it: a missing dataset key makes
// boot seed the demo sample, which would look like a clone of whatever workspace
// the user just came from.)
// mergedRemoteWorkspaces ensures the workspace-list reconciliation runs at most
// once per page load. It answers "does this account own workspaces this device has
// never heard of?", which cannot change without either a sign-in or a reload — and
// it costs a dial, so running it on every restartBackendSync (a toggle, a URL
// change, a sign-in) added a connection to an already-crowded burst for an answer
// that had not changed.
var mergedRemoteWorkspaces bool

func mergeRemoteWorkspaces() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() || mergedRemoteWorkspaces {
		return
	}
	mergedRemoteWorkspaces = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var resp backendrpc.ListWorkspacesResponse
		if err := invokeAuthed(ctx, pr, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{}, &resp); err != nil {
			logSyncError("backend workspace list failed", err)
			return
		}
		r := loadRegistry()
		added := 0
		for _, w := range resp.Workspaces {
			id := strings.TrimSpace(w.ID)
			if id == "" || w.Deleted || r.Has(id) {
				continue
			}
			name := strings.TrimSpace(w.Name)
			if name == "" {
				name = id
			}
			r = r.Add(id, name)
			if strings.TrimSpace(w.Color) != "" {
				r = r.SetColor(id, w.Color)
			} else {
				r = r.SetColor(id, paletteColor(len(r.Workspaces)))
			}
			if data, err := store.Export(store.EmptyDataset()); err == nil {
				saveBlob(id, map[string]string{datasetStoreKey: string(data)})
			}
			added++
		}
		if added == 0 {
			return
		}
		saveRegistry(r)
		uistate.PostNotice(uistate.T("sync.workspacesAdded", added), false)
		if app := appstate.Default; app != nil {
			app.Log().Info("backend sync added remote workspaces to this device", "count", added)
		}
	}()
}

func pullActiveWorkspaceFromBackend(reloadOnApply bool) {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	pullMu.Lock()
	if pullInFlight {
		pullMu.Unlock()
		return // an identical pull is already running; a second one changes nothing
	}
	pullInFlight = true
	pullMu.Unlock()
	markHostedHydrationLoading()

	go func() {
		defer func() {
			pullMu.Lock()
			pullInFlight = false
			pullMu.Unlock()
		}()
		yieldToBrowser() // rendering first: never dial from inside a frame
		defer timePhase("pull.total")()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var resp backendrpc.GetWorkspaceResponse
		err := invokeAuthed(ctx, pr, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: w.ID}, &resp)
		if err != nil {
			logSyncError("backend sync pull failed", err)
			setSyncStatus(syncStatus{State: "error", Pending: pendingSyncCount(), Message: customSyncErrorMessage(err, "pull failed"), AuthFailed: isAuthError(err)})
			markHostedHydrationError(customSyncErrorMessage(err, "CashFlux could not load your server data."))
			return
		}
		if !resp.Found || len(resp.Dataset) == 0 {
			// The server holds no snapshot for this workspace at all — the shape of a
			// freshly activated device against an empty account. Seed it with what
			// this device already has instead of waiting for the user's next edit to
			// trigger the very first upload: without this, a device that signs in and
			// then only READS its data sits on "Synced" indefinitely while the server
			// stays empty, which reads as sync being broken. Safe by construction —
			// there is nothing on the server to lose an LWW race against.
			markHostedHydrationReady()
			forceBackendSeedActiveWorkspace()
			return
		}
		// Same reason as the push path: hydrating decrypts, downloads blobs and
		// imports into SQLite, all of it CPU on the renderer's thread.
		yieldToBrowser()
		dataset, err := hydrateBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), w.ID, resp.Dataset)
		if errors.Is(err, errSyncDatasetLocked) {
			// The snapshot is encrypted and the app is locked. Don't apply or drop it —
			// the server keeps it, and onAppUnlocked re-pulls once the passcode is known.
			setSyncStatus(syncStatus{State: "locked", Pending: pendingSyncCount(), Message: "unlock to sync encrypted data"})
			markHostedHydrationLocked(resp.Dataset, "")
			return
		}
		if err != nil {
			logSyncError("backend artifact blob download failed", err)
			setSyncStatus(syncStatus{State: "error", Pending: pendingSyncCount(), Message: customSyncErrorMessage(err, "artifact blob download failed"), AuthFailed: isAuthError(err)})
			markHostedHydrationError(customSyncErrorMessage(err, "CashFlux could not decrypt or download your server data."))
			return
		}
		meta := loadSyncMeta(w.ID)
		localUpdatedAt, hasLocalMeta := parseSyncMetaTime(meta)
		remoteUpdatedAt, err := time.Parse(time.RFC3339Nano, resp.Workspace.UpdatedAt)
		if err != nil {
			logSyncError("backend sync timestamp parse failed", err)
			markHostedHydrationError("CashFlux received an invalid server-data timestamp.")
			return
		}
		// The seeded demo is not "local data" for the purposes of this decision. Once
		// it has been autosaved, hadLocalDataset is true, and a device that has never
		// synced has no local meta either — the combination ShouldApplyRemote reads as
		// "unsynced local work, keep it" and refuses the remote snapshot. On a browser
		// signing in for the first time that means it sits on demo data forever while
		// the account's real records wait on the server. There is nothing to protect:
		// sample data is what the app invented, not what the user typed.
		localIsUsers := hadLocalDataset && !uistate.SampleActive()
		if !syncstate.ShouldApplyRemote(localUpdatedAt, hasLocalMeta, localIsUsers, remoteUpdatedAt, true) {
			markHostedHydrationReady()
			return
		}
		app := appstate.Default
		if app == nil {
			markHostedHydrationError("CashFlux could not open its local data store.")
			return
		}
		if err := importRemoteDataset(app, dataset); err != nil {
			logSyncError("backend sync import failed", err)
			markHostedHydrationError("CashFlux could not import your server data.")
			return
		}
		// What just replaced the local dataset is the account's real data, so the
		// "you're viewing sample data" banner — and the "Start fresh" button sitting
		// next to it — must not survive the hydrate. Nothing else in the sync path
		// cleared this flag, so a freshly signed-in browser showed a wipe affordance
		// over freshly arrived real records.
		uistate.SetSampleActive(false)
		// Deliberate same-tab dataset replacement: advance the cross-tab generation
		// (other tabs must stop overwriting) and this tab's own write entitlement.
		datasetMyGen = bumpDatasetGen()
		hadLocalDataset = true
		meta = syncMeta{UpdatedAt: resp.Workspace.UpdatedAt, Version: resp.Workspace.Version, Hash: datasetHash(dataset)}
		// A remote snapshot actually landed on this device — real traffic, same as an
		// accepted push. (Seen only when reloadOnApply is false; an applying pull that
		// reloads the page takes the flash with it, which is fine — the reload is a
		// far louder signal that something arrived.)
		noteSyncActivity()
		setSyncStatus(syncStatus{State: "synced", Pending: pendingSyncCount(), LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		if hostedHydrationRequired() {
			// The financial shell is still unmounted. Persist the authoritative
			// snapshot first, then release the hosted gate without a reload so the
			// user's just-entered App Lock passcode remains in memory.
			markHostedHydrationApplying()
			saveSyncedDatasetThen(w.ID, dataset, meta, markHostedHydrationReady)
			return
		}
		if reloadOnApply {
			// The dataset and its sync metadata are one durability unit. Reloading
			// after fire-and-forget IndexedDB writes could persist the dataset but
			// lose the metadata; the next watch event then looked like it would
			// overwrite unsynced local work and was rejected forever. Commit both,
			// in order, before allowing the document to reload.
			saveSyncedDatasetThen(w.ID, dataset, meta, reloadPage)
		} else {
			saveSyncedDatasetThen(w.ID, dataset, meta, nil)
		}
	}()
}

func workspaceSort(r workspace.Registry, id string) int {
	for i, w := range r.Workspaces {
		if w.ID == id {
			return i
		}
	}
	return 0
}

func datasetHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func syncMetaKey(workspaceID string) string { return syncMetaPrefix + workspaceID }

func loadSyncMeta(workspaceID string) syncMeta {
	var meta syncMeta
	if raw := lsGet(syncMetaKey(workspaceID)); raw != "" {
		_ = json.Unmarshal([]byte(raw), &meta)
	}
	return meta
}

func saveSyncMeta(workspaceID string, meta syncMeta) {
	if data, err := json.Marshal(meta); err == nil {
		lsSet(syncMetaKey(workspaceID), string(data))
	}
}

// saveSyncedDatasetThen persists a pulled dataset and the metadata that proves
// where it came from before continuing. They must survive or fail as a reload
// boundary together: a dataset without its server timestamp/hash is treated as
// unsynced local work and correctly protected from later remote overwrites.
func saveSyncedDatasetThen(workspaceID string, dataset []byte, meta syncMeta, done func()) {
	data, err := json.Marshal(meta)
	if err != nil {
		if done != nil {
			done()
		}
		return
	}
	payload := dataset
	if datasetEncryptionActive() {
		payload, err = encryptDatasetSync(dataset, activePasscode)
		if err != nil {
			if app := appstate.Default; app != nil {
				app.Log().Error("backend sync local encryption failed; snapshot not persisted", "err", err)
			}
			setSyncStatus(syncStatus{State: "error", Message: "could not encrypt synced data on this device"})
			markHostedHydrationError("CashFlux decrypted your server data but could not protect it with App Lock on this device.")
			return
		}
	}
	lsSetThen(datasetStoreKey, string(payload), func() {
		lsSetThen(syncMetaKey(workspaceID), string(data), done)
	})
}

// pendingCount caches the queue LENGTH so callers that only want a number never
// deserialize the queue to get it.
//
// Twelve call sites asked for pendingSyncCount() — every "Pending: N" in a
// status update, on paths that run per render and per sync event. Each one parsed
// the whole queue, and a queue entry carries a FULL COPY of the dataset as a
// string, so counting one pending change meant unmarshalling a megabyte of JSON.
// The queue is per-device state owned by this tab, so an in-memory count cannot
// drift from it the way a second persisted key could.
var (
	pendingCountMu    sync.Mutex
	pendingCountVal   int
	pendingCountKnown bool
)

// setPendingCount records the authoritative length after any queue write or read.
func setPendingCount(n int) {
	pendingCountMu.Lock()
	pendingCountVal, pendingCountKnown = n, true
	pendingCountMu.Unlock()
}

// pendingSyncCount returns how many mutations are waiting to upload, parsing the
// stored queue only on the first call of a session.
func pendingSyncCount() int {
	pendingCountMu.Lock()
	if pendingCountKnown {
		n := pendingCountVal
		pendingCountMu.Unlock()
		return n
	}
	pendingCountMu.Unlock()
	return len(loadSyncQueue()) // cold: pay the parse once, loadSyncQueue caches it
}

func loadSyncQueue() []queuedSyncMutation {
	var queue []queuedSyncMutation
	if raw := lsGet(syncQueueKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &queue)
	}
	setPendingCount(len(queue))
	return queue
}

func saveSyncQueue(queue []queuedSyncMutation) {
	// Every write is also the authoritative count, so pendingSyncCount never has to
	// re-parse to answer "how many are waiting?".
	setPendingCount(len(queue))
	if len(queue) == 0 {
		lsRemove(syncQueueKey)
		return
	}
	if data, err := json.Marshal(queue); err == nil {
		lsSet(syncQueueKey, string(data))
	}
}

// saveConflictBackup stores a rejected local mutation as the recoverable backup for
// its workspace (C309). One slot per workspace — the latest conflict overwrites the
// previous, since the user's most recent local edit is the one worth recovering.
func saveConflictBackup(item queuedSyncMutation) {
	if data, err := json.Marshal(item); err == nil {
		lsSet(syncConflictPrefix+item.WorkspaceID, string(data))
	}
}

// loadConflictBackup returns the recoverable local mutation that last lost an LWW
// conflict for a workspace, and whether one exists.
func loadConflictBackup(workspaceID string) (queuedSyncMutation, bool) {
	raw := lsGet(syncConflictPrefix + workspaceID)
	if raw == "" {
		return queuedSyncMutation{}, false
	}
	var item queuedSyncMutation
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return queuedSyncMutation{}, false
	}
	return item, true
}

// clearConflictBackup discards the recoverable backup once the user restores or
// dismisses it.
func clearConflictBackup(workspaceID string) {
	lsRemove(syncConflictPrefix + workspaceID)
}

// restoreConflictBackup re-applies a backed-up local mutation that previously lost an
// LWW conflict: it re-stamps the client timestamp to now (so it wins the next LWW
// round against the snapshot that beat it), re-enqueues it, clears the backup, and
// kicks a flush. Returns false if there is no backup for the workspace. (C309)
func restoreConflictBackup(workspaceID string) bool {
	item, ok := loadConflictBackup(workspaceID)
	if !ok {
		return false
	}
	item.ClientUpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	item.LastAttemptError = ""
	enqueueSyncMutation(item)
	clearConflictBackup(workspaceID)
	flushBackendSyncQueue()
	return true
}

// hasConflictBackup reports whether a recoverable conflict backup exists for the
// active workspace (drives the Settings restore affordance).
func hasConflictBackup(workspaceID string) bool {
	_, ok := loadConflictBackup(workspaceID)
	return ok
}

// resolveConflictKeepLocal re-pushes the stashed local dataset with Force=true
// so the server accepts it unconditionally (bypassing the LWW staleness check),
// then clears the conflict backup and marks sync as settled. Called by
// SyncConflictHost's "Keep my changes" action. (C309 / #464)
func resolveConflictKeepLocal() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	item, ok := loadConflictBackup(w.ID)
	if !ok {
		// Nothing stashed — conflict may have already been resolved; reset status.
		setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		return
	}
	go func() {
		syncPushMu.Lock()
		defer syncPushMu.Unlock()
		setSyncStatus(syncStatus{State: "syncing"})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		dataset, err := prepareBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), item.WorkspaceID, []byte(item.Dataset))
		if err != nil {
			setSyncStatus(syncStatus{State: "error", Message: customSyncErrorMessage(err, "artifact upload failed"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-keep artifact upload failed", err)
			return
		}
		var resp backendrpc.PutWorkspaceResponse
		err = invokeAuthed(ctx, pr, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
			Workspace: backendrpc.Workspace{
				ID:       item.WorkspaceID,
				Name:     item.Name,
				Color:    item.Color,
				Sort:     item.Sort,
				DeviceID: item.DeviceID,
			},
			Dataset:         dataset,
			ClientUpdatedAt: item.ClientUpdatedAt,
			Force:           true, // bypass LWW staleness check — user chose "keep local"
		}, &resp)
		if err != nil {
			setSyncStatus(syncStatus{State: "error", Message: customSyncErrorMessage(err, "force push failed"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-keep force push failed", err)
			return
		}
		// Force=true means the server always accepts; clear the backup and settle.
		clearConflictBackup(item.WorkspaceID)
		saveSyncMeta(item.WorkspaceID, syncMeta{UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: item.Hash})
		setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		uistate.PostNotice(uistate.T("sync.conflictResolvedKeepLocal"), false)
	}()
}

// resolveConflictUseServer pulls the current server snapshot, applies it
// locally, and discards the stashed local dataset ONLY after a successful
// import — so the stash is never lost due to a mid-operation failure. Called by
// SyncConflictHost's "Use server version" action. (C309 / #464)
func resolveConflictUseServer() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	wID := w.ID
	go func() {
		setSyncStatus(syncStatus{State: "syncing"})
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var resp backendrpc.GetWorkspaceResponse
		err := invokeAuthed(ctx, pr, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: wID}, &resp)
		if err != nil {
			setSyncStatus(syncStatus{State: "conflict", Message: customSyncErrorMessage(err, "pull failed"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-server pull failed", err)
			return
		}
		if !resp.Found || len(resp.Dataset) == 0 {
			// Server has no snapshot — treat as resolved (nothing to pull).
			clearConflictBackup(wID)
			setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			return
		}
		dataset, err := hydrateBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), wID, resp.Dataset)
		if errors.Is(err, errSyncDatasetLocked) {
			// Can't apply the server copy while locked — keep the conflict and tell the
			// user to unlock first; the choice re-runs once the passcode is known.
			setSyncStatus(syncStatus{State: "locked", Message: "unlock to resolve with server copy"})
			return
		}
		if err != nil {
			setSyncStatus(syncStatus{State: "conflict", Message: customSyncErrorMessage(err, "pull failed"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-server hydrate failed", err)
			return
		}
		app := appstate.Default
		if app == nil {
			setSyncStatus(syncStatus{State: "conflict"})
			return
		}
		if err := importRemoteDataset(app, dataset); err != nil {
			setSyncStatus(syncStatus{State: "conflict", Message: customSyncErrorMessage(err, "import failed"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-server import failed", err)
			return
		}
		// Deliberate same-tab dataset replacement: advance the cross-tab generation
		// (other tabs must stop overwriting) and this tab's own write entitlement.
		datasetMyGen = bumpDatasetGen()
		hadLocalDataset = true
		meta := syncMeta{
			UpdatedAt: resp.Workspace.UpdatedAt,
			Version:   resp.Workspace.Version,
			Hash:      datasetHash(dataset),
		}
		// Only discard the stash after the import has succeeded — the user's local
		// edit is recoverable until this point.
		clearConflictBackup(wID)
		setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		uistate.PostNotice(uistate.T("sync.conflictResolvedUseServer"), false)
		saveSyncedDatasetThen(wID, dataset, meta, reloadPage)
	}()
}

func enqueueSyncMutation(item queuedSyncMutation) {
	upsertQueuedSyncMutation(item)
	setSyncStatus(syncStatus{State: "syncing", Pending: pendingSyncCount()})
}

func upsertQueuedSyncMutation(item queuedSyncMutation) {
	queue := loadSyncQueue()
	// Stamp the current account on new work so the queue can later tell whose
	// it is. Entries queued while signed out stay unowned and are adopted at
	// the next sign-in (adoptIdentityForSync).
	if strings.TrimSpace(item.UserID) == "" {
		item.UserID = signedInUserID()
	}
	pending := make([]syncstate.PendingMutation, 0, len(queue))
	for _, q := range queue {
		pending = append(pending, syncstate.PendingMutation{UserID: q.UserID, WorkspaceID: q.WorkspaceID, Hash: q.Hash, UpdatedAt: q.ClientUpdatedAt})
	}
	pending = syncstate.UpsertPending(pending, syncstate.PendingMutation{UserID: item.UserID, WorkspaceID: item.WorkspaceID, Hash: item.Hash, UpdatedAt: item.ClientUpdatedAt})
	next := make([]queuedSyncMutation, 0, len(pending))
	for _, p := range pending {
		if p.UserID == item.UserID && p.WorkspaceID == item.WorkspaceID && p.Hash == item.Hash {
			next = append(next, item)
			continue
		}
		for _, q := range queue {
			if q.UserID == p.UserID && q.WorkspaceID == p.WorkspaceID && q.Hash == p.Hash {
				next = append(next, q)
				break
			}
		}
	}
	saveSyncQueue(next)
}

// removeQueuedSyncMutation drops acknowledged work. Keyed on (account,
// workspace) since C696, so acknowledging one identity's upload cannot silently
// clear another identity's unpushed work for the same workspace id — a state a
// device-account change genuinely produces.
func removeQueuedSyncMutation(workspaceID, hash string) {
	queue := loadSyncQueue()
	pending := make([]syncstate.PendingMutation, 0, len(queue))
	byKey := map[string]queuedSyncMutation{}
	key := func(userID, wsID, h string) string { return userID + "\x00" + wsID + "\x00" + h }
	for _, q := range queue {
		pending = append(pending, syncstate.PendingMutation{UserID: q.UserID, WorkspaceID: q.WorkspaceID, Hash: q.Hash, UpdatedAt: q.ClientUpdatedAt})
		byKey[key(q.UserID, q.WorkspaceID, q.Hash)] = q
	}
	pending = syncstate.RemovePending(pending, syncstate.Binding{UserID: signedInUserID(), WorkspaceID: workspaceID}, hash)
	// An entry queued before identities were recorded carries no owner, so it
	// would survive a removal keyed to the current account and then be retried
	// for ever. Acknowledge that one too: it is this device's own work, and the
	// push that just succeeded is exactly what it was waiting for.
	pending = syncstate.RemovePending(pending, syncstate.Binding{WorkspaceID: workspaceID}, hash)
	next := make([]queuedSyncMutation, 0, len(pending))
	for _, p := range pending {
		if q, ok := byKey[key(p.UserID, p.WorkspaceID, p.Hash)]; ok {
			next = append(next, q)
		}
	}
	saveSyncQueue(next)
}

func setSyncStatus(status syncStatus) {
	if status.State == "" {
		status.State = "local" // C5: unset = local-only, not "synced"
	}
	if data, err := json.Marshal(status); err == nil {
		lsSet(syncStatusKey, string(data))
	}
	// C324: make the chip reactive. setSyncStatus is called from background
	// goroutines (watch/flush/pull); bumping the captured revision atom triggers a
	// re-render so the chip reflects the new state without waiting for an unrelated
	// render. Captured during SyncChip's render; no-op until the chip has mounted.
	if syncStatusCaptured {
		capturedSyncRev.Set(capturedSyncRev.Get() + 1)
	}
}

func loadSyncStatus() syncStatus {
	var status syncStatus
	if raw := lsGet(syncStatusKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &status)
	}
	// C320 (supersedes C5): the chip reflects CLOUD sync. If no backend is configured
	// (the default local-first session, OR a backend that was configured then turned
	// off), there is nothing to report — force an empty state so SyncChip stays
	// invisible. This also discards a stale "synced" left in localStorage from a
	// previously-active backend, which would otherwise read as a false "Synced".
	if !uistate.LoadPrefs().Normalize().BackendActive() {
		status.State = ""
		return status
	}
	if status.State == "" {
		if pending := pendingSyncCount(); pending > 0 {
			status.State = "offline"
			status.Pending = pending
		} else {
			// C5: a session that never cloud-synced is LOCAL, not "synced" — defaulting
			// to "synced" rendered a misleading "Synced" chip on a local-first session
			// (and defeated SyncChip's "invisible until cloud sync is in use" intent).
			// Real cloud syncs set State="synced" explicitly (see setSyncStatus callers).
			status.State = "local"
		}
	}
	return status
}

func syncStatusLabel() string {
	status := loadSyncStatus()
	switch status.State {
	case "syncing":
		return "Syncing"
	case "offline":
		if status.Pending > 0 {
			return "Offline - " + strconv.Itoa(status.Pending) + " queued"
		}
		return "Offline"
	case "error":
		return "Sync error"
	case syncStateRebind:
		// Without its own case this fell through to the default, which says
		// "Synced" when nothing is queued and "N queued" when something is —
		// the first a flat lie, the second an implication that it is going to
		// go. It is not going to go until somebody decides.
		return "Needs your decision"
	case "conflict":
		return "Newer server copy available"
	case "local", "":
		// C320: local-first / no backend configured — no cloud "Synced" claim.
		return "Saved on this device"
	default:
		if status.Pending > 0 {
			return strconv.Itoa(status.Pending) + " queued"
		}
		return "Synced"
	}
}

func syncDeviceID() string {
	if id := strings.TrimSpace(lsGet(syncDeviceIDKey)); id != "" {
		return id
	}
	id := ""
	crypto := js.Global().Get("crypto")
	if !crypto.IsUndefined() && !crypto.IsNull() {
		// Call it AS A METHOD of crypto. Getting the function and .Invoke()ing it
		// detaches it from its receiver, and crypto.randomUUID is spec'd to throw
		// TypeError "Illegal invocation" on a null/wrong `this` — which surfaces here
		// as a Go panic that takes the whole wasm app down, on the one path that
		// reaches this function before any device id has been stored (a device's very
		// first sync push, right after it signs in). The recover below is a second
		// belt: a hostile/patched crypto is never worth crashing the app over when a
		// timestamp id does the job.
		id = randomUUIDSafe(crypto)
	}
	if strings.TrimSpace(id) == "" {
		id = "browser-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	lsSet(syncDeviceIDKey, id)
	return id
}

// randomUUIDSafe returns crypto.randomUUID() as a string, or "" if the call is
// unavailable or throws. Any JS exception from syscall/js arrives as a Go panic, so
// the recover here is what keeps an id-generation failure from killing the wasm app
// — the caller has a perfectly good timestamp fallback.
func randomUUIDSafe(crypto js.Value) (id string) {
	defer func() {
		if r := recover(); r != nil {
			id = ""
		}
	}()
	if crypto.Get("randomUUID").Type() != js.TypeFunction {
		return ""
	}
	return crypto.Call("randomUUID").String()
}

func parseSyncMetaTime(meta syncMeta) (time.Time, bool) {
	if strings.TrimSpace(meta.UpdatedAt) == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, meta.UpdatedAt)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func logSyncError(msg string, err error) {
	if app := appstate.Default; app != nil && err != nil {
		app.Log().Warn(msg, "err", err)
	}
}
