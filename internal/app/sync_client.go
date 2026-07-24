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
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
func effectiveServerToken(pr prefs.Prefs) string {
	if t := strings.TrimSpace(lsGet(authAccessTokenKey)); t != "" {
		return t
	}
	return pr.ServerToken
}

// hasRotatableSession reports whether a Custom Sync refresh token is on
// hand — the signal that this device's credential can be refreshed/degraded,
// as opposed to a static self-host token, which never rotates and is left
// entirely alone by this machinery.
func hasRotatableSession() bool {
	return strings.TrimSpace(lsGet(authRefreshTokenKey)) != ""
}

// dialAuthed dials the backend using whichever token is currently effective
// (rotated session token, or the static self-host token) — the single choke
// point every call site should dial through so a refresh is picked up
// immediately by the next dial, with no other plumbing required.
func dialAuthed(ctx context.Context, pr prefs.Prefs) (*grpc.ClientConn, error) {
	return syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: effectiveServerToken(pr)})
}

// isAuthError reports whether err is the backend rejecting the bearer token
// (codes.Unauthenticated) — the trigger for the reactive refresh fallback,
// as opposed to any other RPC failure (network, quota, validation, ...),
// which a refresh cannot fix and retrying would just waste a round trip.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.Unauthenticated
}

// invokeAuthed calls method against *conn and, on an Unauthenticated failure,
// performs the C423 reactive fallback: exactly one RefreshToken attempt via
// refreshAccessToken, then — only if that succeeded — re-dials *conn with the
// refreshed token and retries the original call exactly once more. The old
// connection is closed before being replaced, and *conn always ends up
// pointing at whichever connection is current, so a caller's own
// `defer (*conn).Close()` keeps working unmodified.
func invokeAuthed(ctx context.Context, conn **grpc.ClientConn, pr prefs.Prefs, method string, req, resp any) error {
	err := (*conn).Invoke(ctx, method, req, resp, backendrpc.JSONCallOptions()...)
	if !isAuthError(err) {
		return err
	}
	if !refreshAccessToken(ctx, pr) {
		return err
	}
	fresh := uistate.LoadPrefs().Normalize()
	newConn, dialErr := dialAuthed(ctx, fresh)
	if dialErr != nil {
		return err
	}
	old := *conn
	*conn = newConn
	_ = old.Close()
	return (*conn).Invoke(ctx, method, req, resp, backendrpc.JSONCallOptions()...)
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
	conn, err := syncbridge.Dial(dialCtx, syncbridge.Config{ServerURL: pr.ServerURL, Token: token})
	if err != nil {
		logSyncError("token refresh dial failed", err)
		return false
	}
	defer conn.Close()
	var resp backendrpc.TokenPairResponse
	err = conn.Invoke(dialCtx, backendrpc.MethodAuthRefreshToken, backendrpc.RefreshTokenRequest{RefreshToken: refreshToken}, &resp, backendrpc.JSONCallOptions()...)
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
func degradeToLocalOnly() {
	lsRemove(authAccessTokenKey)
	lsRemove(authRefreshTokenKey)
	lsRemove(authExpiresInKey)
	stopProactiveRefreshTimer()
	stopBackendWatch()
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
	wireSyncLifecycleListeners()
	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
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
			flushBackendSyncQueue()
			pullActiveWorkspaceFromBackend(true)
		}
		return nil
	})
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
		st.Pending = len(loadSyncQueue())
		setSyncStatus(st)
		return nil
	}))
}

func pushActiveWorkspaceToBackend(dataset []byte, updatedAt time.Time) {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
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

func flushBackendSyncQueue() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	go func() {
		syncPushMu.Lock()
		defer syncPushMu.Unlock()
		queue := loadSyncQueue()
		if len(queue) == 0 {
			setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			return
		}
		setSyncStatus(syncStatus{State: "syncing", Pending: len(queue)})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		conn, err := dialAuthed(ctx, pr)
		if err != nil {
			setSyncStatus(syncStatus{State: "offline", Pending: len(queue), Message: customSyncErrorMessage(err, "backend unavailable"), AuthFailed: isAuthError(err)})
			logSyncError("backend sync dial failed", err)
			return
		}
		defer func() { conn.Close() }()
		for _, item := range queue {
			dataset, err := prepareBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), item.WorkspaceID, []byte(item.Dataset))
			if err != nil {
				item.LastAttemptError = err.Error()
				upsertQueuedSyncMutation(item)
				setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: customSyncErrorMessage(err, "artifact blob upload failed"), AuthFailed: isAuthError(err)})
				logSyncError("backend artifact blob upload failed", err)
				return
			}
			var resp backendrpc.PutWorkspaceResponse
			err = invokeAuthed(ctx, &conn, pr, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
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
				setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: customSyncErrorMessage(err, "sync failed"), AuthFailed: isAuthError(err)})
				logSyncError("backend sync push failed", err)
				return
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
				setSyncStatus(syncStatus{State: "conflict", Pending: len(loadSyncQueue()), Message: "newer server snapshot available"})
				uistate.PostNotice(uistate.T("sync.conflictBackedUp"), false)
				if app := appstate.Default; app != nil {
					app.Log().Warn("backend sync push rejected; local edit backed up, newer server snapshot pulled", "workspace", item.WorkspaceID)
				}
				continue
			}
			// Accepted: only now is it safe to drop the local mutation from the queue.
			removeQueuedSyncMutation(item.WorkspaceID, item.Hash)
			saveSyncMeta(item.WorkspaceID, syncMeta{UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: item.Hash})
		}
		setSyncStatus(syncStatus{State: "synced", Pending: len(loadSyncQueue()), LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
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
		return
	}
	wireSyncLifecycleListeners()
	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
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
		conn, err := dialAuthed(ctx, pr)
		if err != nil {
			logSyncError("backend sync watch dial failed", err)
			if !sleepBackoff() {
				return
			}
			continue
		}
		stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, backendrpc.MethodSyncWatchWorkspaces, backendrpc.JSONCallOptions()...)
		if err == nil {
			err = stream.SendMsg(&backendrpc.WatchWorkspacesRequest{IncludeDeleted: true})
		}
		if err == nil {
			err = stream.CloseSend()
		}
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
		_ = conn.Close()
		if !sleepBackoff() {
			return
		}
	}
}

// readBackendWatch reads live workspace events until the stream ends, pulling the
// active workspace whenever another device changes it. It returns whether it
// received at least one event, which the reconnect loop uses as a health signal
// (a stream that delivered data was healthy even if it was short-lived).
func readBackendWatch(stream grpc.ClientStream) (received bool) {
	for {
		var event backendrpc.WatchWorkspacesResponse
		if err := stream.RecvMsg(&event); err != nil {
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
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		conn, err := dialAuthed(ctx, pr)
		if err != nil {
			logSyncError("backend sync dial failed", err)
			return
		}
		defer func() { conn.Close() }()
		var resp backendrpc.GetWorkspaceResponse
		err = invokeAuthed(ctx, &conn, pr, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: w.ID}, &resp)
		if err != nil {
			logSyncError("backend sync pull failed", err)
			setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: customSyncErrorMessage(err, "pull failed"), AuthFailed: isAuthError(err)})
			return
		}
		if !resp.Found || len(resp.Dataset) == 0 {
			return
		}
		dataset, err := hydrateBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), w.ID, resp.Dataset)
		if errors.Is(err, errSyncDatasetLocked) {
			// The snapshot is encrypted and the app is locked. Don't apply or drop it —
			// the server keeps it, and onAppUnlocked re-pulls once the passcode is known.
			setSyncStatus(syncStatus{State: "locked", Pending: len(loadSyncQueue()), Message: "unlock to sync encrypted data"})
			return
		}
		if err != nil {
			logSyncError("backend artifact blob download failed", err)
			setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: customSyncErrorMessage(err, "artifact blob download failed"), AuthFailed: isAuthError(err)})
			return
		}
		meta := loadSyncMeta(w.ID)
		localUpdatedAt, hasLocalMeta := parseSyncMetaTime(meta)
		remoteUpdatedAt, err := time.Parse(time.RFC3339Nano, resp.Workspace.UpdatedAt)
		if err != nil {
			logSyncError("backend sync timestamp parse failed", err)
			return
		}
		if !syncstate.ShouldApplyRemote(localUpdatedAt, hasLocalMeta, hadLocalDataset, remoteUpdatedAt, true) {
			return
		}
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.ImportJSON(dataset); err != nil {
			logSyncError("backend sync import failed", err)
			return
		}
		lsSet(datasetStoreKey, string(dataset))
		// Deliberate same-tab dataset replacement: advance the cross-tab generation
		// (other tabs must stop overwriting) and this tab's own write entitlement.
		datasetMyGen = bumpDatasetGen()
		hadLocalDataset = true
		saveSyncMeta(w.ID, syncMeta{UpdatedAt: resp.Workspace.UpdatedAt, Version: resp.Workspace.Version, Hash: datasetHash(dataset)})
		setSyncStatus(syncStatus{State: "synced", Pending: len(loadSyncQueue()), LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		if reloadOnApply {
			reloadPage()
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

func loadSyncQueue() []queuedSyncMutation {
	var queue []queuedSyncMutation
	if raw := lsGet(syncQueueKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &queue)
	}
	return queue
}

func saveSyncQueue(queue []queuedSyncMutation) {
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
		conn, err := dialAuthed(ctx, pr)
		if err != nil {
			setSyncStatus(syncStatus{State: "error", Pending: 0, Message: customSyncErrorMessage(err, "backend unavailable"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-keep dial failed", err)
			return
		}
		defer func() { conn.Close() }()
		dataset, err := prepareBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), item.WorkspaceID, []byte(item.Dataset))
		if err != nil {
			setSyncStatus(syncStatus{State: "error", Message: customSyncErrorMessage(err, "artifact upload failed"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-keep artifact upload failed", err)
			return
		}
		var resp backendrpc.PutWorkspaceResponse
		err = invokeAuthed(ctx, &conn, pr, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
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
		conn, err := dialAuthed(ctx, pr)
		if err != nil {
			// Revert to conflict so the chip still offers the modal.
			setSyncStatus(syncStatus{State: "conflict", Message: customSyncErrorMessage(err, "backend unavailable"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-server dial failed", err)
			return
		}
		defer func() { conn.Close() }()
		var resp backendrpc.GetWorkspaceResponse
		err = invokeAuthed(ctx, &conn, pr, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: wID}, &resp)
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
		if err := app.ImportJSON(dataset); err != nil {
		setSyncStatus(syncStatus{State: "conflict", Message: customSyncErrorMessage(err, "import failed"), AuthFailed: isAuthError(err)})
			logSyncError("conflict resolve-server import failed", err)
			return
		}
		lsSet(datasetStoreKey, string(dataset))
		// Deliberate same-tab dataset replacement: advance the cross-tab generation
		// (other tabs must stop overwriting) and this tab's own write entitlement.
		datasetMyGen = bumpDatasetGen()
		hadLocalDataset = true
		saveSyncMeta(wID, syncMeta{
			UpdatedAt: resp.Workspace.UpdatedAt,
			Version:   resp.Workspace.Version,
			Hash:      datasetHash(dataset),
		})
		// Only discard the stash after the import has succeeded — the user's local
		// edit is recoverable until this point.
		clearConflictBackup(wID)
		setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		uistate.PostNotice(uistate.T("sync.conflictResolvedUseServer"), false)
		reloadPage()
	}()
}

func enqueueSyncMutation(item queuedSyncMutation) {
	upsertQueuedSyncMutation(item)
	setSyncStatus(syncStatus{State: "syncing", Pending: len(loadSyncQueue())})
}

func upsertQueuedSyncMutation(item queuedSyncMutation) {
	queue := loadSyncQueue()
	pending := make([]syncstate.PendingMutation, 0, len(queue))
	for _, q := range queue {
		pending = append(pending, syncstate.PendingMutation{WorkspaceID: q.WorkspaceID, Hash: q.Hash, UpdatedAt: q.ClientUpdatedAt})
	}
	pending = syncstate.UpsertPending(pending, syncstate.PendingMutation{WorkspaceID: item.WorkspaceID, Hash: item.Hash, UpdatedAt: item.ClientUpdatedAt})
	next := make([]queuedSyncMutation, 0, len(pending))
	for _, p := range pending {
		if p.WorkspaceID == item.WorkspaceID && p.Hash == item.Hash {
			next = append(next, item)
			continue
		}
		for _, q := range queue {
			if q.WorkspaceID == p.WorkspaceID && q.Hash == p.Hash {
				next = append(next, q)
				break
			}
		}
	}
	saveSyncQueue(next)
}

func removeQueuedSyncMutation(workspaceID, hash string) {
	queue := loadSyncQueue()
	pending := make([]syncstate.PendingMutation, 0, len(queue))
	byKey := map[string]queuedSyncMutation{}
	for _, q := range queue {
		pending = append(pending, syncstate.PendingMutation{WorkspaceID: q.WorkspaceID, Hash: q.Hash, UpdatedAt: q.ClientUpdatedAt})
		byKey[q.WorkspaceID+"\x00"+q.Hash] = q
	}
	pending = syncstate.RemovePending(pending, workspaceID, hash)
	next := make([]queuedSyncMutation, 0, len(pending))
	for _, p := range pending {
		if q, ok := byKey[p.WorkspaceID+"\x00"+p.Hash]; ok {
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
		if pending := len(loadSyncQueue()); pending > 0 {
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
		randomUUID := crypto.Get("randomUUID")
		if randomUUID.Type() == js.TypeFunction {
			id = randomUUID.Invoke().String()
		}
	}
	if strings.TrimSpace(id) == "" {
		id = "browser-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	lsSet(syncDeviceIDKey, id)
	return id
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
