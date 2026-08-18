// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

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
// recorded). Empty is deliberately treated as "adoptable" rather than
// "foreign" everywhere downstream — see syncstate.Binding.OwnedBy — because a
// client that refused to sync state it had never labelled would strand every
// existing install on the upgrade.
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

// pendingBinding returns the binding of the first queued mutation, which is the
// one an upload decision is about.
func pendingBinding() (syncstate.PendingMutation, bool) {
	for _, q := range loadSyncQueue() {
		return syncstate.PendingMutation{
			UserID:      q.UserID,
			WorkspaceID: q.WorkspaceID,
			Hash:        q.Hash,
			UpdatedAt:   q.ClientUpdatedAt,
		}, true
	}
	return syncstate.PendingMutation{}, false
}

// uploadDecision says what the sync loop should do with the queue right now.
func uploadDecision() syncstate.UploadDecision {
	next, ok := pendingBinding()
	if !ok {
		return syncstate.UploadNothing
	}
	last := ""
	for _, q := range loadSyncQueue() {
		if q.WorkspaceID == next.WorkspaceID {
			last = q.LastAttemptError
			break
		}
	}
	return syncstate.DecideUpload(signedInUserID(), next, last)
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
