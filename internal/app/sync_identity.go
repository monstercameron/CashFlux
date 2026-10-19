// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

var datasetServerIDCache = struct {
	sync.Mutex
	ids map[string]string
}{ids: make(map[string]string)}

type serverVersionIdentity struct {
	ServerID string `json:"serverId"`
}

// currentDatasetOwner returns the owner identity for the configured backend.
func currentDatasetOwner() syncstate.DatasetOwner {
	return syncstate.DatasetOwner{ServerID: currentDatasetServerID(), UserID: signedInUserID()}
}

// currentDatasetServerID uses the stable server ID when supported. Older or
// unreachable servers are scoped by their normalized configured URL.
func currentDatasetServerID() string {
	return datasetServerIDForURL(uistate.LoadPrefs().Normalize().ServerURL)
}

func datasetServerIDForURL(serverURL string) string {
	endpoint := normalizedDatasetServerURL(serverURL)
	if endpoint == "" {
		return ""
	}
	datasetServerIDCache.Lock()
	if id := datasetServerIDCache.ids[endpoint]; id != "" {
		datasetServerIDCache.Unlock()
		return id
	}
	datasetServerIDCache.Unlock()

	serverID := ""
	req, err := http.NewRequest(http.MethodGet, endpoint+"/v1/version", nil)
	if err == nil {
		client := &http.Client{Timeout: 3 * time.Second}
		if resp, requestErr := client.Do(req); requestErr == nil {
			var version serverVersionIdentity
			if resp.StatusCode == http.StatusOK {
				_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&version)
				serverID = strings.TrimSpace(version.ServerID)
			}
			_ = resp.Body.Close()
		}
	}
	if serverID == "" {
		serverID = endpoint
	}
	datasetServerIDCache.Lock()
	datasetServerIDCache.ids[endpoint] = serverID
	datasetServerIDCache.Unlock()
	return serverID
}

func normalizedDatasetServerURL(serverURL string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if endpoint == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return endpoint
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

// This file teaches the client WHO it is signed in as (TODOS.md C696).
//
// Until now nothing on this side recorded an account id. A bearer token was
// stored, sync metadata was keyed by workspace id alone, and the queue held
// mutations with no owner. That is fine while one browser has one account for
// ever, and it fails the moment the identity changes underneath: the local
// workspace id still belongs to the previous user, every push is refused with
// `workspace not found`, and the client — having no way to notice that the work
// belongs to somebody else — retries the same impossible request indefinitely.
//
// The fix is not a smarter retry. It is knowing the account, so the client can
// tell "the server is down" (retry) from "this is not my workspace" (a decision
// only the person in front of the browser can make).

// syncUserIDKey holds the account id the current session belongs to. It is kept
// alongside the token rather than inside the dataset because it describes the
// CONNECTION, not the household — switching accounts must not rewrite data.
const syncUserIDKey = "cashflux:auth:user-id"

// meResponse is the slice of GET /v1/me this layer needs.
type meResponse struct {
	UserID string `json:"userId"`
}

// signedInUserID returns the account this browser is currently authenticated
// as, or "" when unknown (signed out, or a session that predates this being
// recorded). An empty OWNER on a piece of state is adoptable by a known
// account — see syncstate.Binding.OwnedBy — but an empty SIGNED-IN identity
// authorizes nothing: since SV0.1 queued work is parked, not pushed, while
// the identity is unknown (syncstate.PushableEntry).
func signedInUserID() string { return strings.TrimSpace(lsGet(syncUserIDKey)) }

// setSignedInUserID records the account, and reports whether it CHANGED.
// The caller uses that to decide whether queued work needs re-examining: an
// unchanged identity is the ordinary case and must stay cheap.
func setSignedInUserID(userID string) (changed bool) {
	userID = strings.TrimSpace(userID)
	prev := signedInUserID()
	if prev == userID {
		return false
	}
	if userID == "" {
		lsRemove(syncUserIDKey)
	} else {
		lsSet(syncUserIDKey, userID)
	}
	return true
}

// clearSignedInUserID forgets the account on sign-out. The QUEUE is deliberately
// not cleared with it: unpushed work is the user's data, and signing out is not
// a statement that they want it discarded.
func clearSignedInUserID() { _ = setSignedInUserID("") }

// refreshSignedInUserID asks the server who this token belongs to and records
// the answer.
//
// It goes over plain HTTP to /v1/me rather than through the gRPC tunnel because
// the tunnel's calls are all workspace-scoped — and a client that cannot address
// any workspace, which is exactly the situation this exists to diagnose, can
// still ask who it is.
func refreshSignedInUserID(serverURL, token string) (string, bool) {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	token = strings.TrimSpace(token)
	if serverURL == "" || token == "" {
		return "", false
	}
	req, err := http.NewRequest(http.MethodGet, serverURL+"/v1/me", nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", false
	}
	var me meResponse
	if err := json.Unmarshal(body, &me); err != nil {
		return "", false
	}
	if strings.TrimSpace(me.UserID) == "" {
		return "", false
	}
	return strings.TrimSpace(me.UserID), true
}

// adoptIdentityForSync records the signed-in account and reconciles the queue
// with it. Called after any successful sign-in and whenever sync starts.
//
// Unowned queued work is adopted by the current account, because state written
// before this existed genuinely belongs to whoever is here now. Work belonging
// to a DIFFERENT account is left exactly where it is: it is a decision the user
// has not made yet, and a sync layer that quietly deleted it would destroy the
// very thing a rebind exists to rescue.
func adoptIdentityForSync(serverURL, token string) {
	userID, ok := refreshSignedInUserID(serverURL, token)
	if !ok {
		setSyncStatus(syncStatus{
			State:   syncStateIdentityUnknown,
			Pending: pendingSyncCount(),
			Message: uistate.T("sync.identityUnknown"),
		})
		return
	}
	changed := setSignedInUserID(userID)
	queue := loadSyncQueue()
	if len(queue) == 0 {
		return
	}
	adopted := adoptQueueOwnership(queue, userID)
	if changed || !sameQueueOwnership(queue, adopted) {
		saveSyncQueue(adopted)
	}
	refreshRebindStatus()
}

// adoptQueueOwnership stamps unowned entries with userID.
func adoptQueueOwnership(queue []queuedSyncMutation, userID string) []queuedSyncMutation {
	out := make([]queuedSyncMutation, 0, len(queue))
	for _, q := range queue {
		if strings.TrimSpace(q.UserID) == "" {
			q.UserID = userID
		}
		out = append(out, q)
	}
	return out
}

func sameQueueOwnership(a, b []queuedSyncMutation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].UserID != b[i].UserID {
			return false
		}
	}
	return true
}

// splitQueueByOwner divides the queue into work the signed-in account may push
// and work belonging to somebody else.
func splitQueueByOwner(queue []queuedSyncMutation, userID string) (mine, foreign []queuedSyncMutation) {
	for _, q := range queue {
		binding := syncstate.Binding{UserID: q.UserID, WorkspaceID: q.WorkspaceID}
		if binding.OwnedBy(userID) {
			mine = append(mine, q)
			continue
		}
		foreign = append(foreign, q)
	}
	return mine, foreign
}

// uploadDecision says what the sync loop should do with the queue right now.
//
// It asks about the work the signed-in account OWNS, not about whatever happens
// to sit at the head of the queue. Those differ in exactly the case this
// feature exists for: after an identity change the old account's entry is still
// queued, and the user then edits something, which appends an entry owned by
// the NEW account. Deciding from the head would block that new, perfectly
// pushable work behind a stranded entry the user has not resolved yet — turning
// a recoverable mismatch into a permanent stall.
func uploadDecision() syncstate.UploadDecision {
	queue := loadSyncQueue()
	if len(queue) == 0 {
		return syncstate.UploadNothing
	}
	userID := signedInUserID()
	if strings.TrimSpace(userID) == "" {
		// We do not know who this session is, so nothing may be pushed
		// (SV0.1, fail closed). This deliberately reverses the older
		// fail-open rule, under which an unknown identity classified nothing
		// as foreign: sign-out clears the recorded user ID, so in the window
		// between signing out of account A and the next identity resolving,
		// A's queued snapshots were pushable under account B's fresh token.
		// The server rejecting a foreign workspace was the only guard, and it
		// does not hold when both accounts own a workspace with the same ID
		// (every install starts with "default").
		//
		// The work is parked, not dropped: once adoptIdentityForSync resolves
		// the account (at boot, after the identity-change reload), the same
		// entries are re-examined against a known owner. A workspace the
		// server has already refused is still rebind-terminal rather than a
		// mere wait.
		for _, q := range queue {
			if syncstate.IsWorkspaceNotFound(q.LastAttemptError) {
				return syncstate.UploadRebind
			}
		}
		return syncstate.UploadSignIn
	}
	mine, foreign := splitQueueByOwner(queue, userID)
	if len(mine) == 0 {
		if len(foreign) > 0 {
			return syncstate.UploadRebind
		}
		return syncstate.UploadNothing
	}
	// Among my own work, a workspace the server has already said I cannot
	// address is terminal — retrying it is what used to spin for ever.
	for _, q := range mine {
		if syncstate.IsWorkspaceNotFound(q.LastAttemptError) {
			return syncstate.UploadRebind
		}
	}
	return syncstate.UploadProceed
}

// blockedWorkspaceID names the workspace a rebind decision is actually about.
//
// The recovery card used to assume this was the ACTIVE workspace, which is only
// true by coincidence. The stranded entry can belong to a workspace the user is
// not currently looking at — or to one they have since deleted locally, in which
// case the active id can never equal it and the card could never target the
// thing that was stuck. Reported by adversarial review, 2026-08-17.
//
// Preference order: an entry of MINE the server has refused, then the oldest
// foreign entry, then the active workspace as a last resort.
func blockedWorkspaceID() string {
	queue := loadSyncQueue()
	mine, foreign := splitQueueByOwner(queue, signedInUserID())
	for _, q := range mine {
		if syncstate.IsWorkspaceNotFound(q.LastAttemptError) {
			return q.WorkspaceID
		}
	}
	if len(foreign) > 0 {
		return foreign[0].WorkspaceID
	}
	return ""
}

// scopedEntry projects a queued mutation onto the pure scope type the
// pushability decisions are made with (syncstate.ScopedEntry). The queue's
// UserID field is the entry's owning account — captured when the edit was
// queued, never re-read from the session at send time.
func scopedEntry(q queuedSyncMutation) syncstate.ScopedEntry {
	return syncstate.ScopedEntry{
		WorkspaceID: q.WorkspaceID,
		OwnerUserID: q.UserID,
		BaseVersion: q.BaseVersion,
		Hash:        q.Hash,
	}
}

// splitPushableQueue divides queued work into what the signed-in account may
// send and what stays parked, by the scope captured at enqueue time
// (syncstate.PushableEntry). An unknown identity parks everything.
func splitPushableQueue(queue []queuedSyncMutation) (pushable, parked []queuedSyncMutation) {
	userID := signedInUserID()
	for _, q := range queue {
		if syncstate.PushableEntry(scopedEntry(q), userID) {
			pushable = append(pushable, q)
			continue
		}
		parked = append(parked, q)
	}
	return pushable, parked
}

// pushableQueue is the subset of queued work the signed-in account may send.
// The flush loop iterates this rather than the raw queue, so another identity's
// stranded snapshot is never pushed under the current token.
func pushableQueue(queue []queuedSyncMutation) []queuedSyncMutation {
	pushable, _ := splitPushableQueue(queue)
	return pushable
}

// refreshParkedQueueStatus reports parked work in the sync status instead of
// letting it read as "synced" or as an ordinary backlog. Work owned by a
// different account is waiting for that account to sign in again (or for a
// rebind decision); work parked only because the identity has not resolved
// yet says so. A no-op when nothing is parked, so the ordinary terminal
// status of a flush is untouched.
func refreshParkedQueueStatus() {
	_, parked := splitPushableQueue(loadSyncQueue())
	if len(parked) == 0 {
		return
	}
	scoped := make([]syncstate.ScopedEntry, 0, len(parked))
	for _, q := range parked {
		scoped = append(scoped, scopedEntry(q))
	}
	msg := "waiting for sign-in to be confirmed"
	switch owners := syncstate.ParkedOwners(scoped); {
	case len(owners) == 1 && owners[0] != signedInUserID():
		msg = "waiting for " + owners[0]
	case len(owners) > 1:
		msg = "waiting for another account"
	}
	setSyncStatus(syncStatus{State: syncStateRebind, Pending: pendingSyncCount(), Message: msg})
}

// refreshRebindStatus puts the client into the blocked state when the queue
// cannot possibly drain, so the Cloud settings pane can offer a decision
// instead of a spinner.
//
// This is the behavioural heart of C696. The old loop treated `workspace not
// found` as a transient failure and re-attempted it on every tick, wake and
// watch event — for ever, since nothing on either side was going to change.
func refreshRebindStatus() {
	if uploadDecision() != syncstate.UploadRebind {
		return
	}
	st := loadSyncStatus()
	st.State = syncStateRebind
	st.Pending = pendingSyncCount()
	if strings.TrimSpace(st.Message) == "" {
		st.Message = uistate.T("sync.rebindReason")
	}
	setSyncStatus(st)
}

// syncStateRebind is the status state meaning "this cannot be uploaded as the
// account you are signed in as". It is deliberately not "error": an error
// invites a retry, and retrying is the one thing that cannot work here.
const syncStateRebind = "rebind"

// syncStateIdentityUnknown means the client could not confirm the account
// identity with the server. Queued writes stay paused until a later retry can
// establish which account owns them.
const syncStateIdentityUnknown = "identity-unknown"

// reloadAfterIdentityChange ends every sign-in, sign-out and account switch
// in a full page reload (SV0.1).
//
// Identity used to be re-resolved only at boot, in startBackendSync. Every
// in-page sign-in path instead called restartBackendSync, which flushed the
// queue and pulled without ever learning who the new session belonged to —
// so queued work, the loaded dataset and the sync bookmarks kept the PREVIOUS
// account's scope while the token was already the new account's. That is how
// one account's data gets pushed into another's server copy, and no amount of
// care inside the flush can fix a scope it was never told about. A reload is
// the honest fix: boot re-runs startBackendSync → adoptIdentityForSync before
// anything is pushed, with exactly the credentials now in storage and no
// in-memory state left over from the old identity.
//
// The reload waits for durability first. The caller has just written the new
// credentials (or cleared the old ones) through browserstore, whose writes
// commit asynchronously; reloading before they land could boot the page on
// the identity that was just replaced. browserstore.Barrier blocks until
// every write issued before it has committed to IndexedDB — the same awaited
// mechanism doRefreshAccessToken uses before releasing the cross-tab token
// lock — so it runs in a goroutine (Barrier must never block a JS callback
// stack) and the reload fires only from inside it. The sync watch and the
// proactive-refresh countdown are stopped first so nothing can push, pull or
// rotate a token under the outgoing identity while the barrier waits.
func reloadAfterIdentityChange(reason string) {
	log := slog.Default()
	if app := appstate.Default; app != nil {
		log = app.Log()
	}
	log.Info("reloading page after identity change", "reason", reason)
	stopBackendWatch()
	stopProactiveRefreshTimer()
	dropSharedConn()
	go func() {
		browserstore.Barrier()
		reloadPage()
	}()
}
