// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/backoff"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncengine"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

const (
	v2CursorKeyPrefix   = "cashflux:v2:cursor:"
	v2OutboxStoreKey    = "cashflux:v2:outbox"
	v2ConflictKeyPrefix = "cashflux:v2:conflict:"
)

func init() {
	// Capture durable local record transitions directly inside the IndexedDB
	// commit callback so closing or reloading the browser can never drop dirty
	// state between local commit and v2 outbox enqueue.
	onLocalDatasetCommitted = enqueueLocalCommitForV2Sync
}

var v2SyncCoord = struct {
	mu          sync.Mutex
	engine      *syncengine.Engine
	scope       conflictcheckpoint.Scope
	watchCancel context.CancelFunc
	watchWake   chan struct{}
	retryTimer  *time.Timer
}{}

func v2ScopeKey(prefix string, scope conflictcheckpoint.Scope) string {
	return prefix + scope.ServerID + ":" + scope.AccountID + ":" + scope.WorkspaceID
}

func currentV2Scope() (conflictcheckpoint.Scope, bool) {
	if browserstore.Locked() {
		return conflictcheckpoint.Scope{}, false
	}
	pr := uistate.LoadPrefs().Normalize()
	if !backendSessionReady() || !backendConnectionActive(pr) {
		return conflictcheckpoint.Scope{}, false
	}
	userID := strings.TrimSpace(signedInUserID())
	if userID == "" {
		return conflictcheckpoint.Scope{}, false
	}
	serverID := strings.TrimSpace(currentDatasetServerID())
	if serverID == "" {
		return conflictcheckpoint.Scope{}, false
	}
	wsID := strings.TrimSpace(activeWorkspaceID())
	if wsID == "" {
		wsID = defaultWSID
	}
	scope := conflictcheckpoint.Scope{
		ServerID:    serverID,
		AccountID:   userID,
		WorkspaceID: wsID,
	}
	return scope, scope.Validate() == nil
}

// v2WorkerTransport adapts cashflux.v2.BlobSync RPCs over services.wasm using
// invokeAuthed (which provides single-flight reactive token refresh and
// per-endpoint connection pooling).
type v2WorkerTransport struct{}

func (v2WorkerTransport) AppendChanges(ctx context.Context, req backendrpc.AppendChangesRequest) (backendrpc.AppendChangesResponse, error) {
	pr := uistate.LoadPrefs().Normalize()
	var resp backendrpc.AppendChangesResponse
	err := invokeAuthed(ctx, pr, backendrpc.MethodBlobSyncAppendChanges, req, &resp)
	return resp, err
}

func (v2WorkerTransport) ListChanges(ctx context.Context, req backendrpc.ListChangesRequest) (backendrpc.ListChangesResponse, error) {
	pr := uistate.LoadPrefs().Normalize()
	var resp backendrpc.ListChangesResponse
	err := invokeAuthed(ctx, pr, backendrpc.MethodBlobSyncListChanges, req, &resp)
	return resp, err
}

func (v2WorkerTransport) PutSnapshot(ctx context.Context, req backendrpc.PutSnapshotRequest) (backendrpc.PutSnapshotResponse, error) {
	pr := uistate.LoadPrefs().Normalize()
	var resp backendrpc.PutSnapshotResponse
	err := invokeAuthed(ctx, pr, backendrpc.MethodBlobSyncPutSnapshot, req, &resp)
	return resp, err
}

func (v2WorkerTransport) GetSnapshot(ctx context.Context, req backendrpc.GetSnapshotRequest) (backendrpc.GetSnapshotResponse, error) {
	pr := uistate.LoadPrefs().Normalize()
	var resp backendrpc.GetSnapshotResponse
	err := invokeAuthed(ctx, pr, backendrpc.MethodBlobSyncGetSnapshot, req, &resp)
	return resp, err
}

// v2WebCryptoCipher encrypts/decrypts v2 change batches and snapshots with the
// active App Lock passcode (using the cached session PBKDF2 key in
// datasetcrypto.go) and handles incremental attachment blob offloading.
type v2WebCryptoCipher struct {
	workspaceID string
}

func (v2WebCryptoCipher) Ready() error {
	if !loadAppLock().Active() {
		return syncengine.ErrDataProtectionRequired
	}
	if activePasscode == "" {
		return syncengine.ErrDatasetLocked
	}
	return nil
}

func (c v2WebCryptoCipher) Encrypt(ctx context.Context, plain []byte) ([]byte, error) {
	if err := c.Ready(); err != nil {
		return nil, err
	}
	// If plain is a full dataset JSON (not a ChangeBatch), offload binary
	// attachments incrementally via BlobService and strip device secrets.
	if looksLikeFullDatasetJSON(plain) {
		pr := uistate.LoadPrefs().Normalize()
		return prepareBackendSyncDataset(ctx, normalizedBackendEndpoint(pr.ServerURL), effectiveServerToken(pr), c.workspaceID, plain)
	}
	env, err := encryptDatasetSync(plain, activePasscode)
	if err != nil {
		return nil, err
	}
	if len(env) == 0 || !cryptobox.IsEnvelope(env) {
		return nil, errSyncEncryptEmpty
	}
	return env, nil
}

func (c v2WebCryptoCipher) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	if err := c.Ready(); err != nil {
		return nil, err
	}
	if !cryptobox.IsEnvelope(ciphertext) {
		return nil, errors.New("syncengine: server returned unencrypted bytes while data protection is active")
	}
	plain, err := decryptEnvelopeSync(ciphertext, activePasscode)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", syncengine.ErrWrongPasscode, err)
	}
	if looksLikeFullDatasetJSON(plain) {
		pr := uistate.LoadPrefs().Normalize()
		return hydrateBackendSyncDataset(ctx, normalizedBackendEndpoint(pr.ServerURL), effectiveServerToken(pr), c.workspaceID, plain)
	}
	return plain, nil
}

func looksLikeFullDatasetJSON(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	return json.Unmarshal(raw, &probe) == nil && probe.SchemaVersion > 0
}

// v2LocalDatasetAdapter applies v2 remote changes and snapshots LIVE IN PLACE
// without reloading the browser, committing unified record checkpoints and
// always clearing syncApplyPaused via defer on every path.
type v2LocalDatasetAdapter struct {
	scope conflictcheckpoint.Scope
}

func (v2LocalDatasetAdapter) ExportRedactedJSON() ([]byte, error) {
	a := appstate.Default
	if a == nil {
		return nil, errors.New("app state unavailable")
	}
	return a.ExportJSONRedacted()
}

func (a v2LocalDatasetAdapter) ApplyRemoteChanges(changes []store.RecordChange) error {
	if len(changes) == 0 {
		return nil
	}
	app := appstate.Default
	if app == nil {
		return errors.New("app state unavailable")
	}
	syncApplyPaused = true
	defer func() { syncApplyPaused = false }()

	if err := app.Store().ReplayChanges(changes); err != nil {
		return err
	}
	if err := commitLiveAppliedCheckpoint(app, a.scope); err != nil {
		return err
	}
	uistate.BumpDataRevision()
	markHostedHydrationReady()
	return nil
}

func (a v2LocalDatasetAdapter) ApplyRemoteSnapshot(snapshotJSON []byte) error {
	app := appstate.Default
	if app == nil {
		return errors.New("app state unavailable")
	}
	syncApplyPaused = true
	defer func() { syncApplyPaused = false }()

	if _, err := importRemoteDataset(app, snapshotJSON); err != nil {
		return err
	}
	if err := commitLiveAppliedCheckpoint(app, a.scope); err != nil {
		return err
	}
	uistate.BumpDataRevision()
	markHostedHydrationReady()
	return nil
}

// commitLiveAppliedCheckpoint durably persists the current live SQLite state
// as a unified record-checkpoint + backup in browserstore (never a legacy raw
// cryptobox.Envelope) so subsequent incremental record autosaves chain cleanly
// without requiring a page reload.
func commitLiveAppliedCheckpoint(app *appstate.App, scope conflictcheckpoint.Scope) error {
	data, err := localDatasetExport(app)
	if err != nil {
		return err
	}
	ownerRaw := ""
	if scope.ServerID != "" && scope.AccountID != "" {
		if encoded, mErr := json.Marshal(syncstate.DatasetOwner{ServerID: scope.ServerID, UserID: scope.AccountID}); mErr == nil {
			ownerRaw = string(encoded)
		}
	}
	var lastErr error
	generation := datasetMyGen
	for retries := 0; retries < 3; retries++ {
		nextGen := id.New()
		checkpoint := encodeRecordCheckpointGeneration(data, nextGen)
		updates := map[string]string{
			datasetStoreKey:            checkpoint,
			datasetHeadKey:             "",
			datasetCheckpointBackupKey: encodeCheckpointBackup(scope.WorkspaceID, checkpoint, ownerRaw),
		}
		if ownerRaw != "" {
			updates[datasetOwnerKey] = ownerRaw
		}
		values, expected := prepareWorkspaceCommit(updates, generation)
		values[datasetGenKey] = nextGen
		done := make(chan error, 1)
		browserstore.SetSecureBatch(values, expected, func(err error) { done <- err })
		err := <-done
		if err == nil {
			datasetMyGen = nextGen
			hadLocalDataset = true
			hydratedRecordApp = app
			hydratedRecordCursor, _ = app.Store().ChangeCursor()
			hydratedRecordRevision = app.Rev()
			if ownerRaw != "" {
				lastAcceptedOwnerClaim = acceptedOwnerClaim{app: app, workspace: scope.WorkspaceID, value: ownerRaw}
			}
			return nil
		}
		lastErr = err
		if errors.Is(err, atrest.ErrRecordChanged) && datasetMyGen != generation && datasetMyGen == readDatasetGen() {
			generation = datasetMyGen
			continue
		}
		break
	}
	return lastErr
}

// v2BrowserStateStore persists v2 cursors, durable outbox, and scoped conflict
// checkpoints in CashFlux's encrypted browserstore.
type v2BrowserStateStore struct{}

func (v2BrowserStateStore) LoadCursor(scope conflictcheckpoint.Scope) (syncengine.WorkspaceCursor, error) {
	if browserstore.Locked() {
		return syncengine.WorkspaceCursor{}, atrest.ErrLocked
	}
	raw := lsGet(v2ScopeKey(v2CursorKeyPrefix, scope))
	if strings.TrimSpace(raw) == "" {
		return syncengine.WorkspaceCursor{
			ServerID:    scope.ServerID,
			AccountID:   scope.AccountID,
			WorkspaceID: scope.WorkspaceID,
		}, nil
	}
	var c syncengine.WorkspaceCursor
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return syncengine.WorkspaceCursor{
			ServerID:    scope.ServerID,
			AccountID:   scope.AccountID,
			WorkspaceID: scope.WorkspaceID,
		}, nil
	}
	return c, nil
}

func (v2BrowserStateStore) SaveCursor(c syncengine.WorkspaceCursor) error {
	if browserstore.Locked() {
		return atrest.ErrLocked
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	lsSet(v2ScopeKey(v2CursorKeyPrefix, c.Scope()), string(raw))
	return nil
}

func (v2BrowserStateStore) LoadOutbox() ([]syncengine.OutboxEntry, error) {
	if browserstore.Locked() {
		return nil, atrest.ErrLocked
	}
	raw := lsGet(v2OutboxStoreKey)
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []syncengine.OutboxEntry
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, nil
	}
	return out, nil
}

func (v2BrowserStateStore) SaveOutbox(entries []syncengine.OutboxEntry) error {
	if browserstore.Locked() {
		return atrest.ErrLocked
	}
	if len(entries) == 0 {
		lsRemove(v2OutboxStoreKey)
		return nil
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	lsSet(v2OutboxStoreKey, string(raw))
	return nil
}

func (v2BrowserStateStore) LoadConflict(scope conflictcheckpoint.Scope) (*syncengine.StoredConflict, error) {
	if browserstore.Locked() {
		return nil, atrest.ErrLocked
	}
	raw := lsGet(v2ScopeKey(v2ConflictKeyPrefix, scope))
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var c syncengine.StoredConflict
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, nil
	}
	return &c, nil
}

func (v2BrowserStateStore) SaveConflict(c syncengine.StoredConflict) error {
	if browserstore.Locked() {
		return atrest.ErrLocked
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	lsSet(v2ScopeKey(v2ConflictKeyPrefix, c.Checkpoint.Scope), string(raw))
	return nil
}

func (v2BrowserStateStore) ClearConflict(scope conflictcheckpoint.Scope) error {
	if browserstore.Locked() {
		return atrest.ErrLocked
	}
	lsRemove(v2ScopeKey(v2ConflictKeyPrefix, scope))
	return nil
}

func activeV2Conflict() *syncengine.StoredConflict {
	scope, ok := currentV2Scope()
	if !ok {
		return nil
	}
	c, _ := (v2BrowserStateStore{}).LoadConflict(scope)
	return c
}

func activeV2ConflictComparison() (conflictcheckpoint.Comparison, bool) {
	c := activeV2Conflict()
	if c == nil {
		return conflictcheckpoint.Comparison{}, false
	}
	localSnap, _, errL := syncengine.DatasetToCheckpointSnapshot(c.LocalJSON)
	serverSnap, _, errS := syncengine.DatasetToCheckpointSnapshot(c.ServerJSON)
	if errL != nil || errS != nil {
		return conflictcheckpoint.Comparison{}, false
	}
	cmp, err := conflictcheckpoint.Compare(localSnap, serverSnap)
	if err != nil || !cmp.Available {
		return conflictcheckpoint.Comparison{}, false
	}
	return cmp, true
}

func ensureV2Engine() (*syncengine.Engine, bool) {
	scope, ok := currentV2Scope()
	if !ok {
		return nil, false
	}
	v2SyncCoord.mu.Lock()
	defer v2SyncCoord.mu.Unlock()
	if v2SyncCoord.engine != nil && v2SyncCoord.scope == scope {
		return v2SyncCoord.engine, true
	}
	eng, err := syncengine.New(syncengine.Config{
		Scope:       scope,
		DeviceID:    syncDeviceID(),
		DeviceLabel: "This browser",
		NewID:       id.New,
		OnStatus:    publishV2SyncStatus,
	}, v2WorkerTransport{}, v2WebCryptoCipher{workspaceID: scope.WorkspaceID}, v2LocalDatasetAdapter{scope: scope}, v2BrowserStateStore{})
	if err != nil {
		return nil, false
	}
	v2SyncCoord.engine = eng
	v2SyncCoord.scope = scope
	return eng, true
}

func publishV2SyncStatus(snap syncengine.StatusSnapshot) {
	cur := loadSyncStatus()
	next := syncStatus{
		State:   string(snap.Phase),
		Pending: snap.Pending,
		Message: snap.Message,
	}
	if !snap.LastSyncedAt.IsZero() {
		next.LastSyncedAt = snap.LastSyncedAt.UTC().Format(time.RFC3339Nano)
	} else {
		next.LastSyncedAt = cur.LastSyncedAt
	}
	if snap.Phase == syncstate.PhaseSynced {
		noteSyncActivity()
	}
	setSyncStatus(next)
	if snap.Phase == syncstate.PhaseError && !snap.NextRetryAt.IsZero() {
		scheduleV2Retry(time.Until(snap.NextRetryAt))
	} else if snap.Phase == syncstate.PhaseSynced || snap.Phase == syncstate.PhaseConflict {
		cancelV2Retry()
	}
}

func scheduleV2Retry(delay time.Duration) {
	if delay < time.Second {
		delay = time.Second
	}
	v2SyncCoord.mu.Lock()
	if v2SyncCoord.retryTimer != nil {
		v2SyncCoord.retryTimer.Stop()
	}
	v2SyncCoord.retryTimer = time.AfterFunc(delay, func() {
		requestV2Sync()
	})
	v2SyncCoord.mu.Unlock()
}

func cancelV2Retry() {
	v2SyncCoord.mu.Lock()
	if v2SyncCoord.retryTimer != nil {
		v2SyncCoord.retryTimer.Stop()
		v2SyncCoord.retryTimer = nil
	}
	v2SyncCoord.mu.Unlock()
}

func enqueueLocalCommitForV2Sync(workspace string, full bool, changes []store.RecordChange, fullDataset []byte) {
	if browserstore.Locked() || syncApplyPaused {
		return
	}
	eng, ok := ensureV2Engine()
	if !ok {
		return
	}
	if pushBlockedReason() != "" {
		return
	}
	if workspace != activeWorkspaceID() {
		return
	}
	if len(changes) > 0 {
		if err := eng.EnqueueRecords(id.New(), changes); err != nil {
			logSyncError("enqueue v2 record batch failed", err)
			return
		}
	} else if full && len(fullDataset) > 0 {
		if err := eng.EnqueueSnapshot(id.New(), fullDataset); err != nil {
			logSyncError("enqueue v2 snapshot batch failed", err)
			return
		}
	} else {
		return
	}
	requestV2Sync()
}

func requestV2Sync() {
	if browserstore.Locked() || syncApplyPaused {
		return
	}
	eng, ok := ensureV2Engine()
	if !ok {
		return
	}
	if pushBlockedReason() != "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := eng.RequestSync(ctx); err != nil {
			if rpcprotocol.IsCode(err, "Unimplemented") {
				// Fallback gracefully if pointed at an older v1-only server.
				flushBackendSyncQueue()
				pullActiveWorkspaceFromBackend(true)
				return
			}
			logSyncError("v2 sync cycle failed", err)
		}
	}()
}

func startV2Watch() {
	scope, ok := currentV2Scope()
	if !ok {
		return
	}
	stopV2Watch()
	ctx, cancel := context.WithCancel(context.Background())
	wake := make(chan struct{}, 1)
	v2SyncCoord.mu.Lock()
	v2SyncCoord.watchCancel = cancel
	v2SyncCoord.watchWake = wake
	v2SyncCoord.mu.Unlock()
	go runV2WatchLoop(ctx, scope, wake)
}

func stopV2Watch() {
	v2SyncCoord.mu.Lock()
	cancel := v2SyncCoord.watchCancel
	v2SyncCoord.watchCancel = nil
	v2SyncCoord.watchWake = nil
	v2SyncCoord.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func wakeV2Watch() {
	v2SyncCoord.mu.Lock()
	wake := v2SyncCoord.watchWake
	v2SyncCoord.mu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

func runV2WatchLoop(ctx context.Context, scope conflictcheckpoint.Scope, wake <-chan struct{}) {
	attempt := 0
	for {
		if ctx.Err() != nil || browserstore.Locked() {
			return
		}
		pr := uistate.LoadPrefs().Normalize()
		if !backendSessionReady() || !backendConnectionActive(pr) {
			return
		}
		cursor, _ := (v2BrowserStateStore{}).LoadCursor(scope)
		stream, err := openWorkerRPCStream(ctx, normalizedBackendEndpoint(pr.ServerURL), effectiveServerToken(pr),
			backendrpc.MethodBlobSyncWatch, backendrpc.WatchRequest{
				Workspace: scope.WorkspaceID,
				Since:     cursor.AppliedSeq,
			})
		if err != nil {
			if isAuthError(err) && refreshAccessToken(ctx, pr) {
				continue
			}
			if rpcprotocol.IsCode(err, "Unimplemented") {
				// Older server without v2 BlobSync; stop v2 watch cleanly.
				return
			}
		} else {
			attempt = 0
			for {
				var ev backendrpc.WatchEvent
				recvErr := stream.Recv(&ev)
				if recvErr != nil {
					stream.Cancel()
					if isAuthError(recvErr) && refreshAccessToken(ctx, pr) {
						attempt = 0
					}
					break
				}
				if ev.Workspace == scope.WorkspaceID {
					requestV2Sync()
				}
			}
		}
		delay := backoff.Delay(attempt, 2*time.Second, 2*time.Minute)
		attempt++
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-wake:
			timer.Stop()
			attempt = 0
		case <-timer.C:
		}
	}
}

func resolveV2ConflictKeepLocal() bool {
	if activeV2Conflict() == nil {
		return false
	}
	eng, ok := ensureV2Engine()
	if !ok {
		return false
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := eng.ResolveKeepLocal(ctx); err != nil {
			logSyncError("resolve v2 conflict keep local failed", err)
			uistate.PostNotice(uistate.T("cloud1.keepLocalFailed", err.Error()), true)
			return
		}
		uistate.PostNotice(uistate.T("cloud1.keepLocalDone"), false)
	}()
	return true
}

func resolveV2ConflictUseServer() bool {
	if activeV2Conflict() == nil {
		return false
	}
	eng, ok := ensureV2Engine()
	if !ok {
		return false
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := eng.ResolveUseServer(ctx); err != nil {
			logSyncError("resolve v2 conflict use server failed", err)
			uistate.PostNotice(uistate.T("cloud1.useServerFailed", err.Error()), true)
			return
		}
		uistate.PostNotice(uistate.T("cloud1.useServerDone"), false)
	}()
	return true
}

func resolveV2ConflictMerge(prefer syncengine.MergePreference) bool {
	if activeV2Conflict() == nil {
		return false
	}
	eng, ok := ensureV2Engine()
	if !ok {
		return false
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if _, err := eng.ResolveMerge(ctx, prefer); err != nil {
			logSyncError("resolve v2 conflict merge failed", err)
			uistate.PostNotice(uistate.T("cloud1.mergeFailed", err.Error()), true)
			return
		}
		uistate.PostNotice(uistate.T("cloud1.mergeDone"), false)
	}()
	return true
}

func clearV2ScopeState(workspaceID string) {
	scope, ok := currentV2Scope()
	if !ok {
		return
	}
	scope.WorkspaceID = strings.TrimSpace(workspaceID)
	if scope.Validate() != nil {
		return
	}
	lsRemove(v2ScopeKey(v2CursorKeyPrefix, scope))
	lsRemove(v2ScopeKey(v2ConflictKeyPrefix, scope))
	st := v2BrowserStateStore{}
	if outbox, err := st.LoadOutbox(); err == nil {
		_ = st.SaveOutbox(syncengine.ClearOutboxForScope(outbox, scope))
	}
}
