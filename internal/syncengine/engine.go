// SPDX-License-Identifier: MIT

package syncengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/backoff"
	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncstate"
)

// Sentinel errors returned by platform adapters so the engine can classify
// attention-required states cleanly without string matching.
var (
	ErrDataProtectionRequired = errors.New("syncengine: data protection passcode is required")
	ErrDatasetLocked          = errors.New("syncengine: dataset is locked")
	ErrWrongPasscode          = errors.New("syncengine: dataset encrypted with a different passcode")
	ErrIdentityUnknown        = errors.New("syncengine: signed-in account is unknown")
	ErrWorkspaceDeleted       = errors.New("syncengine: remote workspace has been deleted")
	ErrOffline                = errors.New("syncengine: device is offline")
	ErrNoActiveConflict       = errors.New("syncengine: no active conflict to resolve")
)

// StatusSnapshot is the observable sync status published to the UI after every
// state transition.
type StatusSnapshot struct {
	Phase        syncstate.SyncPhase `json:"state"`
	Pending      int                 `json:"pending"`
	AppliedSeq   int64               `json:"appliedSeq"`
	LastSyncedAt time.Time           `json:"lastSyncedAt,omitempty"`
	Message      string              `json:"message,omitempty"`
	ConflictID   string              `json:"conflictId,omitempty"`
	NextRetryAt  time.Time           `json:"nextRetryAt,omitempty"`
}

// Transport abstracts the cashflux.v2.BlobSync RPC surface.
type Transport interface {
	AppendChanges(ctx context.Context, req backendrpc.AppendChangesRequest) (backendrpc.AppendChangesResponse, error)
	ListChanges(ctx context.Context, req backendrpc.ListChangesRequest) (backendrpc.ListChangesResponse, error)
	PutSnapshot(ctx context.Context, req backendrpc.PutSnapshotRequest) (backendrpc.PutSnapshotResponse, error)
	GetSnapshot(ctx context.Context, req backendrpc.GetSnapshotRequest) (backendrpc.GetSnapshotResponse, error)
}

// Cipher encrypts outbound v2 batches/snapshots and decrypts inbound blobs.
type Cipher interface {
	Ready() error
	Encrypt(ctx context.Context, plain []byte) ([]byte, error)
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

// LocalDataset abstracts the live in-memory SQLite + durable local checkpoint
// store. All Apply* methods run live in place without reloading the page.
type LocalDataset interface {
	ExportRedactedJSON() ([]byte, error)
	ApplyRemoteChanges(changes []store.RecordChange) error
	ApplyRemoteSnapshot(snapshotJSON []byte) error
}

// StateStore persists per-workspace v2 cursors, the durable outbox, and
// scope-isolated conflict checkpoints.
type StateStore interface {
	LoadCursor(scope conflictcheckpoint.Scope) (WorkspaceCursor, error)
	SaveCursor(cursor WorkspaceCursor) error
	LoadOutbox() ([]OutboxEntry, error)
	SaveOutbox(entries []OutboxEntry) error
	LoadConflict(scope conflictcheckpoint.Scope) (*StoredConflict, error)
	SaveConflict(conflict StoredConflict) error
	ClearConflict(scope conflictcheckpoint.Scope) error
}

// Config configures an Engine instance.
type Config struct {
	Scope            conflictcheckpoint.Scope
	DeviceID         string
	DeviceLabel      string
	SnapshotInterval int
	PageLimit        int
	BaseBackoff      time.Duration
	MaxBackoff       time.Duration
	Now              func() time.Time
	NewID            func() string
	Logger           *slog.Logger
	OnStatus         func(StatusSnapshot)
}

// Engine is the single serialized v2 BlobSync coordinator for one workspace
// scope. Push, pull, watch wakeups, and conflict resolutions all pass through
// opMu so push and pull can never race or overwrite newer local edits.
type Engine struct {
	cfg       Config
	transport Transport
	cipher    Cipher
	local     LocalDataset
	state     StateStore

	opMu sync.Mutex

	runMu               sync.Mutex
	running             bool
	pendingRun          bool
	offline             bool
	consecutiveFailures int
	status              StatusSnapshot
	deviceCounter       uint64
}

// New constructs a validated v2 sync Engine.
func New(cfg Config, transport Transport, cipher Cipher, local LocalDataset, state StateStore) (*Engine, error) {
	if err := cfg.Scope.Validate(); err != nil {
		return nil, fmt.Errorf("syncengine: invalid scope: %w", err)
	}
	if cfg.DeviceID == "" {
		return nil, errors.New("syncengine: device ID is required")
	}
	if transport == nil || cipher == nil || local == nil || state == nil {
		return nil, errors.New("syncengine: transport, cipher, local dataset, and state store are required")
	}
	if cfg.SnapshotInterval <= 0 {
		cfg.SnapshotInterval = DefaultSnapshotInterval
	}
	if cfg.PageLimit <= 0 || cfg.PageLimit > 1000 {
		cfg.PageLimit = 250
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 2 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 2 * time.Minute
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	e := &Engine{
		cfg:       cfg,
		transport: transport,
		cipher:    cipher,
		local:     local,
		state:     state,
		status:    StatusSnapshot{Phase: syncstate.PhaseLocal},
	}
	return e, nil
}

// Status returns the latest published StatusSnapshot.
func (e *Engine) Status() StatusSnapshot {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	return e.status
}

func (e *Engine) setStatus(next StatusSnapshot, force bool) {
	e.runMu.Lock()
	if !force && !e.status.Phase.CanRoutineTransitionTo(next.Phase) {
		e.runMu.Unlock()
		return
	}
	e.status = next
	cb := e.cfg.OnStatus
	e.runMu.Unlock()
	if cb != nil {
		cb(next)
	}
}

// EnqueueRecords durably appends coalesced local record changes to the v2
// outbox. It never overwrites or drops any previously queued entry.
func (e *Engine) EnqueueRecords(changeID string, changes []store.RecordChange) error {
	changes = store.CoalesceChanges(changes)
	if len(changes) == 0 {
		return nil
	}
	e.opMu.Lock()
	defer e.opMu.Unlock()

	cursor, err := e.state.LoadCursor(e.cfg.Scope)
	if err != nil {
		return err
	}
	outbox, err := e.state.LoadOutbox()
	if err != nil {
		return err
	}
	entry := OutboxEntry{
		ChangeID:    changeID,
		ServerID:    e.cfg.Scope.ServerID,
		AccountID:   e.cfg.Scope.AccountID,
		WorkspaceID: e.cfg.Scope.WorkspaceID,
		DeviceID:    e.cfg.DeviceID,
		BaseSeq:     cursor.AppliedSeq,
		Kind:        BatchKindRecords,
		Changes:     changes,
		QueuedAt:    e.cfg.Now(),
	}
	outbox = EnqueueOutbox(outbox, entry)
	if err := e.state.SaveOutbox(outbox); err != nil {
		return err
	}
	pending := len(OutboxForScope(outbox, e.cfg.Scope))
	cur := e.Status()
	cur.Pending = pending
	e.setStatus(cur, true)
	return nil
}

// EnqueueSnapshot durably queues a full encrypted snapshot batch (used for
// initial bootstrap or explicit replace/import operations).
func (e *Engine) EnqueueSnapshot(changeID string, snapshotJSON []byte) error {
	if len(snapshotJSON) == 0 || !json.Valid(snapshotJSON) {
		return errors.New("syncengine: snapshot JSON is invalid")
	}
	e.opMu.Lock()
	defer e.opMu.Unlock()

	cursor, err := e.state.LoadCursor(e.cfg.Scope)
	if err != nil {
		return err
	}
	outbox, err := e.state.LoadOutbox()
	if err != nil {
		return err
	}
	entry := OutboxEntry{
		ChangeID:     changeID,
		ServerID:     e.cfg.Scope.ServerID,
		AccountID:    e.cfg.Scope.AccountID,
		WorkspaceID:  e.cfg.Scope.WorkspaceID,
		DeviceID:     e.cfg.DeviceID,
		BaseSeq:      cursor.AppliedSeq,
		Kind:         BatchKindSnapshot,
		SnapshotJSON: append(json.RawMessage(nil), snapshotJSON...),
		QueuedAt:     e.cfg.Now(),
	}
	outbox = EnqueueOutbox(outbox, entry)
	if err := e.state.SaveOutbox(outbox); err != nil {
		return err
	}
	pending := len(OutboxForScope(outbox, e.cfg.Scope))
	cur := e.Status()
	cur.Pending = pending
	e.setStatus(cur, true)
	return nil
}

// NotifyOffline marks the device offline without clobbering attention states
// (conflict, locked, rebind, needs-data-protection, identity-unknown).
func (e *Engine) NotifyOffline() {
	e.runMu.Lock()
	e.offline = true
	cur := e.status
	e.runMu.Unlock()
	cur.Phase = syncstate.PhaseOffline
	e.setStatus(cur, false)
}

// NotifyOnline clears the offline flag and resets backoff so the next sync
// attempt runs immediately.
func (e *Engine) NotifyOnline() {
	e.runMu.Lock()
	e.offline = false
	e.consecutiveFailures = 0
	e.runMu.Unlock()
}

// NextRetryDelay returns the exponential backoff delay for the current failure
// streak, or 0 when healthy.
func (e *Engine) NextRetryDelay() time.Duration {
	e.runMu.Lock()
	failures := e.consecutiveFailures
	e.runMu.Unlock()
	if failures <= 0 {
		return 0
	}
	return backoff.Delay(failures-1, e.cfg.BaseBackoff, e.cfg.MaxBackoff)
}

// RequestSync runs SyncOnce and coalesces any concurrent requests that arrive
// while a cycle is already running (guaranteeing watch events and edits are
// never silently dropped).
func (e *Engine) RequestSync(ctx context.Context) error {
	e.runMu.Lock()
	if e.running {
		e.pendingRun = true
		e.runMu.Unlock()
		return nil
	}
	e.running = true
	e.pendingRun = false
	e.runMu.Unlock()

	defer func() {
		e.runMu.Lock()
		e.running = false
		e.runMu.Unlock()
	}()

	var lastErr error
	for {
		lastErr = e.SyncOnce(ctx)
		e.runMu.Lock()
		again := e.pendingRun && ctx.Err() == nil && lastErr == nil
		e.pendingRun = false
		e.runMu.Unlock()
		if !again {
			return lastErr
		}
	}
}

// SyncOnce executes one serialized v2 pull → reconcile → live-apply → push →
// compaction cycle.
func (e *Engine) SyncOnce(ctx context.Context) error {
	e.opMu.Lock()
	defer e.opMu.Unlock()
	return e.syncOnceLocked(ctx)
}

func (e *Engine) syncOnceLocked(ctx context.Context) error {
	e.runMu.Lock()
	isOffline := e.offline
	e.runMu.Unlock()

	outbox, err := e.state.LoadOutbox()
	if err != nil {
		return err
	}
	pending := len(OutboxForScope(outbox, e.cfg.Scope))

	cursor, err := e.state.LoadCursor(e.cfg.Scope)
	if err != nil {
		return err
	}
	if cursor.ServerID == "" {
		cursor.ServerID = e.cfg.Scope.ServerID
		cursor.AccountID = e.cfg.Scope.AccountID
		cursor.WorkspaceID = e.cfg.Scope.WorkspaceID
	}

	// Never clobber an unresolved conflict with background sync.
	existingConflict, err := e.state.LoadConflict(e.cfg.Scope)
	if err != nil {
		return err
	}
	if existingConflict != nil && existingConflict.Checkpoint.State == conflictcheckpoint.StateKept {
		e.setStatus(StatusSnapshot{
			Phase:      syncstate.PhaseConflict,
			Pending:    pending,
			AppliedSeq: cursor.AppliedSeq,
			ConflictID: existingConflict.Checkpoint.ID,
			Message:    "Your changes conflict with another device. Review copies to choose what to keep.",
		}, true)
		return nil
	}

	if err := e.cipher.Ready(); err != nil {
		phase := classifyCipherError(err)
		e.setStatus(StatusSnapshot{
			Phase:      phase,
			Pending:    pending,
			AppliedSeq: cursor.AppliedSeq,
			Message:    err.Error(),
		}, true)
		return err
	}

	if isOffline {
		e.setStatus(StatusSnapshot{
			Phase:      syncstate.PhaseOffline,
			Pending:    pending,
			AppliedSeq: cursor.AppliedSeq,
		}, false)
		return ErrOffline
	}

	e.setStatus(StatusSnapshot{
		Phase:      syncstate.PhaseSyncing,
		Pending:    pending,
		AppliedSeq: cursor.AppliedSeq,
	}, true)

	// Step 1: Pull and reconcile remote changes from v2 BlobSync.
	if err := e.pullAndReconcileLocked(ctx, &cursor); err != nil {
		return e.recordFailureLocked(cursor, err)
	}

	// If pull created a conflict checkpoint, stop before pushing.
	if conflict, loadErr := e.state.LoadConflict(e.cfg.Scope); loadErr == nil && conflict != nil && conflict.Checkpoint.State == conflictcheckpoint.StateKept {
		return nil
	}

	// Step 2: Push all outbox entries for this scope (and loop if new ones
	// arrived cleanly during push).
	if err := e.pushOutboxLocked(ctx, &cursor); err != nil {
		return e.recordFailureLocked(cursor, err)
	}

	// Step 3: Upload a compacted snapshot when needed (first sync or every
	// SnapshotInterval changes).
	if err := e.maybeUploadSnapshotLocked(ctx, &cursor); err != nil {
		// Compaction failure is non-fatal to already-appended changes, but
		// log and let the next cycle retry compaction.
		e.cfg.Logger.Warn("v2 snapshot compaction skipped", "workspace", e.cfg.Scope.WorkspaceID, "err", err)
	}

	if err := e.state.SaveCursor(cursor); err != nil {
		return err
	}

	outbox, _ = e.state.LoadOutbox()
	remaining := len(OutboxForScope(outbox, e.cfg.Scope))
	e.runMu.Lock()
	e.consecutiveFailures = 0
	e.runMu.Unlock()
	e.setStatus(StatusSnapshot{
		Phase:        syncstate.PhaseSynced,
		Pending:      remaining,
		AppliedSeq:   cursor.AppliedSeq,
		LastSyncedAt: e.cfg.Now(),
	}, true)
	return nil
}

func (e *Engine) recordFailureLocked(cursor WorkspaceCursor, err error) error {
	outbox, _ := e.state.LoadOutbox()
	pending := len(OutboxForScope(outbox, e.cfg.Scope))
	phase := classifySyncError(err)

	var nextRetry time.Time
	if phase == syncstate.PhaseError || phase == syncstate.PhaseOffline {
		e.runMu.Lock()
		e.consecutiveFailures++
		delay := backoff.Delay(e.consecutiveFailures-1, e.cfg.BaseBackoff, e.cfg.MaxBackoff)
		e.runMu.Unlock()
		nextRetry = e.cfg.Now().Add(delay)
	}
	e.setStatus(StatusSnapshot{
		Phase:       phase,
		Pending:     pending,
		AppliedSeq:  cursor.AppliedSeq,
		Message:     err.Error(),
		NextRetryAt: nextRetry,
	}, true)
	return err
}

func classifyCipherError(err error) syncstate.SyncPhase {
	switch {
	case errors.Is(err, ErrDataProtectionRequired):
		return syncstate.PhaseNeedsDataProtection
	case errors.Is(err, ErrWrongPasscode):
		return syncstate.PhaseLockedDifferentPasscode
	case errors.Is(err, ErrDatasetLocked):
		return syncstate.PhaseLocked
	default:
		return syncstate.PhaseError
	}
}

func classifySyncError(err error) syncstate.SyncPhase {
	switch {
	case errors.Is(err, ErrDataProtectionRequired):
		return syncstate.PhaseNeedsDataProtection
	case errors.Is(err, ErrWrongPasscode):
		return syncstate.PhaseLockedDifferentPasscode
	case errors.Is(err, ErrDatasetLocked):
		return syncstate.PhaseLocked
	case errors.Is(err, ErrIdentityUnknown):
		return syncstate.PhaseIdentityUnknown
	case errors.Is(err, ErrOffline):
		return syncstate.PhaseOffline
	default:
		return syncstate.PhaseError
	}
}

func (e *Engine) pullAndReconcileLocked(ctx context.Context, cursor *WorkspaceCursor) error {
	firstPage, err := e.transport.ListChanges(ctx, backendrpc.ListChangesRequest{
		Workspace: e.cfg.Scope.WorkspaceID,
		Since:     cursor.AppliedSeq,
		Limit:     e.cfg.PageLimit,
	})
	if err != nil {
		return fmt.Errorf("list v2 changes: %w", err)
	}
	if firstPage.Head <= cursor.AppliedSeq {
		return nil
	}

	outbox, err := e.state.LoadOutbox()
	if err != nil {
		return err
	}
	localPending, localHasSnapshot := FlattenedPendingChanges(outbox, e.cfg.Scope)
	localDirty := len(localPending) > 0 || localHasSnapshot

	truncated := DetectLogTruncation(cursor.AppliedSeq, firstPage.Head, firstPage.Changes)
	needsSnapshotBootstrap := truncated || cursor.AppliedSeq == 0

	if needsSnapshotBootstrap {
		snap, err := e.transport.GetSnapshot(ctx, backendrpc.GetSnapshotRequest{Workspace: e.cfg.Scope.WorkspaceID})
		if err != nil {
			return fmt.Errorf("get v2 snapshot: %w", err)
		}
		if snap.Found {
			plainSnap, err := e.cipher.Decrypt(ctx, snap.Blob)
			if err != nil {
				return fmt.Errorf("decrypt v2 snapshot: %w", err)
			}
			tailBatches, head, err := e.fetchAndDecryptChangesLocked(ctx, snap.CoversSeq, firstPage.Head, cursor)
			if err != nil {
				return err
			}
			serverJSON, err := materializeSnapshotWithBatches(plainSnap, tailBatches)
			if err != nil {
				return err
			}
			if localDirty {
				return e.enterConflictLocked(*cursor, serverJSON, head, nil, localPending)
			}
			if err := e.applyRemoteSnapshotLocked(cursor, serverJSON, head, snap.CoversSeq); err != nil {
				return err
			}
			return nil
		}
		if truncated {
			return errors.New("syncengine: remote change log was pruned and no snapshot is available")
		}
	}

	// Normal incremental catch-up: decrypt firstPage + any subsequent pages.
	// Any change already in our own outbox (e.g. a prior AppendChanges that
	// committed on the server before its reply dropped) is acknowledged
	// idempotently instead of being treated as a remote conflict.
	pendingIDs := make(map[string]bool)
	for _, entry := range OutboxForScope(outbox, e.cfg.Scope) {
		pendingIDs[entry.ChangeID] = true
	}
	remoteBatches, ackedFromPull, head, err := e.decryptAndPageChangesLocked(ctx, firstPage, cursor, pendingIDs)
	if err != nil {
		return err
	}
	if len(ackedFromPull) > 0 {
		outbox = RemoveOutboxByIDs(outbox, ackedFromPull)
		if err := e.state.SaveOutbox(outbox); err != nil {
			return err
		}
		localPending, localHasSnapshot = FlattenedPendingChanges(outbox, e.cfg.Scope)
	}
	if len(remoteBatches) == 0 {
		// All new log entries were our own already-applied changes.
		cursor.AppliedSeq = head
		return e.state.SaveCursor(*cursor)
	}

	rec := ReconcileBatches(localPending, localHasSnapshot, remoteBatches)
	if rec.HasConflict {
		serverJSON, err := e.buildServerPreviewLocked(ctx, cursor, remoteBatches)
		if err != nil {
			return err
		}
		return e.enterConflictLocked(*cursor, serverJSON, head, rec.CollidingKeys, localPending)
	}

	// Clean live-apply (either fast-forward or non-overlapping auto-merge!).
	if len(rec.ApplySnapshotJSON) > 0 {
		serverJSON, err := materializeSnapshotWithBatches(rec.ApplySnapshotJSON, []ChangeBatch{{
			Version:     BatchFormatVersion,
			ChangeID:    "tail",
			WorkspaceID: e.cfg.Scope.WorkspaceID,
			DeviceID:    "remote",
			Kind:        BatchKindRecords,
			Changes:     rec.ApplyRecords,
		}})
		if err != nil && len(rec.ApplyRecords) > 0 {
			return err
		}
		if len(rec.ApplyRecords) == 0 {
			serverJSON = rec.ApplySnapshotJSON
		}
		return e.applyRemoteSnapshotLocked(cursor, serverJSON, head, cursor.SnapshotSeq)
	}

	if len(rec.ApplyRecords) > 0 {
		if err := e.local.ApplyRemoteChanges(rec.ApplyRecords); err != nil {
			return fmt.Errorf("apply remote v2 changes: %w", err)
		}
	}
	cursor.AppliedSeq = head
	cursor.ChangesSinceSnapshot += len(remoteBatches)
	if len(localPending) == 0 {
		if exported, err := e.local.ExportRedactedJSON(); err == nil {
			cursor.LastSyncedHash = HashBytes(exported)
		}
	}
	return e.state.SaveCursor(*cursor)
}

func (e *Engine) applyRemoteSnapshotLocked(cursor *WorkspaceCursor, serverJSON []byte, head, snapshotSeq int64) error {
	prevJSON, _ := e.local.ExportRedactedJSON()
	if cursor.AppliedSeq > 0 {
		if err := CheckDestructiveGuard(prevJSON, serverJSON, false); err != nil {
			return fmt.Errorf("destructive remote snapshot blocked: %w", err)
		}
	}
	if err := e.local.ApplyRemoteSnapshot(serverJSON); err != nil {
		return fmt.Errorf("apply remote v2 snapshot: %w", err)
	}
	cursor.AppliedSeq = head
	if snapshotSeq > cursor.SnapshotSeq {
		cursor.SnapshotSeq = snapshotSeq
		cursor.ChangesSinceSnapshot = 0
	}
	cursor.LastSyncedHash = HashBytes(serverJSON)
	return e.state.SaveCursor(*cursor)
}

func (e *Engine) fetchAndDecryptChangesLocked(ctx context.Context, since, knownHead int64, cursor *WorkspaceCursor) ([]ChangeBatch, int64, error) {
	head := knownHead
	var all []ChangeBatch
	pos := since
	for {
		page, err := e.transport.ListChanges(ctx, backendrpc.ListChangesRequest{
			Workspace: e.cfg.Scope.WorkspaceID,
			Since:     pos,
			Limit:     e.cfg.PageLimit,
		})
		if err != nil {
			return nil, head, fmt.Errorf("list v2 tail changes: %w", err)
		}
		if page.Head > head {
			head = page.Head
		}
		if len(page.Changes) == 0 {
			break
		}
		for _, ch := range page.Changes {
			pos = ch.Seq
			if cursor.HasAcked(ch.ChangeID) {
				continue
			}
			plain, err := e.cipher.Decrypt(ctx, ch.Blob)
			if err != nil {
				return nil, head, fmt.Errorf("decrypt v2 change seq %d: %w", ch.Seq, err)
			}
			batch, err := DecodeChangeBatch(plain)
			if err != nil {
				return nil, head, fmt.Errorf("decode v2 change seq %d: %w", ch.Seq, err)
			}
			all = append(all, batch)
		}
		if pos >= head {
			break
		}
	}
	return all, head, nil
}

func (e *Engine) decryptAndPageChangesLocked(ctx context.Context, firstPage backendrpc.ListChangesResponse, cursor *WorkspaceCursor, pendingIDs map[string]bool) ([]ChangeBatch, map[string]bool, int64, error) {
	head := firstPage.Head
	var all []ChangeBatch
	var ackedFromPull map[string]bool
	pos := cursor.AppliedSeq
	page := firstPage
	for {
		for _, ch := range page.Changes {
			pos = ch.Seq
			if cursor.HasAcked(ch.ChangeID) {
				continue
			}
			if pendingIDs[ch.ChangeID] {
				cursor.RecordAck(ch.ChangeID, ch.Seq)
				if ackedFromPull == nil {
					ackedFromPull = make(map[string]bool)
				}
				ackedFromPull[ch.ChangeID] = true
				continue
			}
			plain, err := e.cipher.Decrypt(ctx, ch.Blob)
			if err != nil {
				return nil, ackedFromPull, head, fmt.Errorf("decrypt v2 change seq %d: %w", ch.Seq, err)
			}
			batch, err := DecodeChangeBatch(plain)
			if err != nil {
				return nil, ackedFromPull, head, fmt.Errorf("decode v2 change seq %d: %w", ch.Seq, err)
			}
			all = append(all, batch)
		}
		if pos >= head || len(page.Changes) == 0 {
			break
		}
		var err error
		page, err = e.transport.ListChanges(ctx, backendrpc.ListChangesRequest{
			Workspace: e.cfg.Scope.WorkspaceID,
			Since:     pos,
			Limit:     e.cfg.PageLimit,
		})
		if err != nil {
			return nil, ackedFromPull, head, fmt.Errorf("list next v2 change page: %w", err)
		}
		if page.Head > head {
			head = page.Head
		}
	}
	return all, ackedFromPull, head, nil
}

func (e *Engine) buildServerPreviewLocked(ctx context.Context, cursor *WorkspaceCursor, remoteBatches []ChangeBatch) ([]byte, error) {
	// Prefer authoritative server snapshot + tail so local unpushed edits
	// cannot pollute the server copy shown in the conflict diff.
	snap, err := e.transport.GetSnapshot(ctx, backendrpc.GetSnapshotRequest{Workspace: e.cfg.Scope.WorkspaceID})
	if err == nil && snap.Found {
		plainSnap, decErr := e.cipher.Decrypt(ctx, snap.Blob)
		if decErr == nil {
			tail, _, tailErr := e.fetchAndDecryptChangesLocked(ctx, snap.CoversSeq, cursor.AppliedSeq, &WorkspaceCursor{})
			if tailErr == nil {
				if preview, matErr := materializeSnapshotWithBatches(plainSnap, tail); matErr == nil {
					return preview, nil
				}
			}
		}
	}
	// Fallback when no server snapshot exists yet: start from local dataset
	// with remote batches applied on top in a disposable SQLite store.
	localJSON, err := e.local.ExportRedactedJSON()
	if err != nil {
		return nil, err
	}
	return materializeSnapshotWithBatches(localJSON, remoteBatches)
}

func (e *Engine) enterConflictLocked(cursor WorkspaceCursor, serverJSON []byte, serverHead int64, colliding []RecordKey, localPending []store.RecordChange) error {
	localJSON, err := e.local.ExportRedactedJSON()
	if err != nil {
		return fmt.Errorf("export local dataset for conflict checkpoint: %w", err)
	}
	_, counts, _ := DatasetToCheckpointSnapshot(localJSON)
	if counts == nil {
		counts = map[string]int{}
	}
	e.deviceCounter++
	checkpointID := e.nextID()
	cp := conflictcheckpoint.Checkpoint{
		ID:    checkpointID,
		Scope: e.cfg.Scope,
		CreatedAt: conflictcheckpoint.RevisionOrder{
			DatasetRevision: uint64(max(serverHead, 0)),
			DeviceCounter:   e.deviceCounter,
		},
		CreatedWallTime: e.cfg.Now(),
		Origin:          conflictcheckpoint.OriginRejectedPush,
		BaseVersion:     uint64(max(cursor.AppliedSeq, 0)),
		DeviceLabel:     e.cfg.DeviceLabel,
		RecordCounts:    counts,
		State:           conflictcheckpoint.StateKept,
	}
	stored := StoredConflict{
		Checkpoint:     cp,
		LocalJSON:      append(json.RawMessage(nil), localJSON...),
		ServerJSON:     append(json.RawMessage(nil), serverJSON...),
		ServerHeadSeq:  serverHead,
		CollidingKeys:  colliding,
		PendingChanges: localPending,
	}
	if err := e.state.SaveConflict(stored); err != nil {
		return fmt.Errorf("save conflict checkpoint: %w", err)
	}
	outbox, _ := e.state.LoadOutbox()
	pending := len(OutboxForScope(outbox, e.cfg.Scope))
	e.setStatus(StatusSnapshot{
		Phase:      syncstate.PhaseConflict,
		Pending:    pending,
		AppliedSeq: cursor.AppliedSeq,
		ConflictID: checkpointID,
		Message:    "Your changes conflict with another device. Review copies to choose what to keep.",
	}, true)
	return nil
}

func (e *Engine) pushOutboxLocked(ctx context.Context, cursor *WorkspaceCursor) error {
	for {
		outbox, err := e.state.LoadOutbox()
		if err != nil {
			return err
		}
		mine := OutboxForScope(outbox, e.cfg.Scope)
		if len(mine) == 0 {
			// Durable dirty safety net: if this workspace has never synced yet
			// (AppliedSeq == 0) and has local content differing from last synced
			// hash, seed it automatically so edits can never be stranded.
			if cursor.AppliedSeq == 0 {
				localJSON, expErr := e.local.ExportRedactedJSON()
				if expErr == nil && len(localJSON) > 0 {
					h := HashBytes(localJSON)
					if h != cursor.LastSyncedHash {
						seedEntry := OutboxEntry{
							ChangeID:     e.nextID(),
							ServerID:     e.cfg.Scope.ServerID,
							AccountID:    e.cfg.Scope.AccountID,
							WorkspaceID:  e.cfg.Scope.WorkspaceID,
							DeviceID:     e.cfg.DeviceID,
							BaseSeq:      0,
							Kind:         BatchKindSnapshot,
							SnapshotJSON: localJSON,
							QueuedAt:     e.cfg.Now(),
						}
						outbox = EnqueueOutbox(outbox, seedEntry)
						if err := e.state.SaveOutbox(outbox); err != nil {
							return err
						}
						mine = OutboxForScope(outbox, e.cfg.Scope)
					}
				}
			}
			if len(mine) == 0 {
				return nil
			}
		}

		wireBatch := make([]backendrpc.BlobChange, 0, len(mine))
		attemptedIDs := make(map[string]bool, len(mine))
		for _, entry := range mine {
			batch := entry.ToBatch()
			plain, err := batch.Encode()
			if err != nil {
				return err
			}
			enc, err := e.cipher.Encrypt(ctx, plain)
			if err != nil {
				return fmt.Errorf("encrypt v2 change batch %s: %w", entry.ChangeID, err)
			}
			wireBatch = append(wireBatch, backendrpc.BlobChange{
				ChangeID: entry.ChangeID,
				DeviceID: entry.DeviceID,
				Blob:     enc,
			})
			attemptedIDs[entry.ChangeID] = true
		}

		resp, err := e.transport.AppendChanges(ctx, backendrpc.AppendChangesRequest{
			Workspace: e.cfg.Scope.WorkspaceID,
			Changes:   wireBatch,
		})
		if err != nil {
			// Reload outbox before marking attempts so concurrent new edits are
			// never overwritten or lost.
			latest, loadErr := e.state.LoadOutbox()
			if loadErr == nil {
				_ = e.state.SaveOutbox(MarkOutboxAttempt(latest, attemptedIDs, err.Error()))
			}
			return fmt.Errorf("append v2 changes: %w", err)
		}
		if len(resp.Seqs) != len(mine) {
			return fmt.Errorf("append v2 changes: server returned %d seqs for %d changes", len(resp.Seqs), len(mine))
		}

		for i, entry := range mine {
			cursor.RecordAck(entry.ChangeID, resp.Seqs[i])
		}
		latest, err := e.state.LoadOutbox()
		if err != nil {
			return err
		}
		if err := e.state.SaveOutbox(RemoveOutboxByIDs(latest, attemptedIDs)); err != nil {
			return err
		}
		if err := e.state.SaveCursor(*cursor); err != nil {
			return err
		}
	}
}

func (e *Engine) maybeUploadSnapshotLocked(ctx context.Context, cursor *WorkspaceCursor) error {
	if cursor.AppliedSeq <= cursor.SnapshotSeq {
		return nil
	}
	if cursor.SnapshotSeq > 0 && cursor.ChangesSinceSnapshot < e.cfg.SnapshotInterval {
		return nil
	}
	localJSON, err := e.local.ExportRedactedJSON()
	if err != nil {
		return err
	}
	enc, err := e.cipher.Encrypt(ctx, localJSON)
	if err != nil {
		return err
	}
	resp, err := e.transport.PutSnapshot(ctx, backendrpc.PutSnapshotRequest{
		Workspace: e.cfg.Scope.WorkspaceID,
		CoversSeq: cursor.AppliedSeq,
		Blob:      enc,
	})
	if err != nil {
		return err
	}
	if resp.Accepted {
		cursor.SnapshotSeq = cursor.AppliedSeq
		cursor.ChangesSinceSnapshot = 0
		cursor.LastSyncedHash = HashBytes(localJSON)
	}
	return nil
}

// ResolveKeepLocal resolves an active conflict by keeping this device's copy,
// ensuring it is live-applied locally, appending a superseding v2 snapshot
// batch, and syncing cleanly.
func (e *Engine) ResolveKeepLocal(ctx context.Context) error {
	e.opMu.Lock()
	defer e.opMu.Unlock()

	conflict, err := e.state.LoadConflict(e.cfg.Scope)
	if err != nil {
		return err
	}
	if conflict == nil {
		return ErrNoActiveConflict
	}
	if err := e.cipher.Ready(); err != nil {
		return err
	}
	if err := e.local.ApplyRemoteSnapshot(conflict.LocalJSON); err != nil {
		return fmt.Errorf("apply local conflict copy: %w", err)
	}
	return e.commitResolvedDatasetLocked(ctx, conflict.LocalJSON, conflict.ServerHeadSeq)
}

// ResolveUseServer resolves an active conflict by live-applying the server copy
// in place, clearing conflicting outbox entries, and advancing the cursor.
func (e *Engine) ResolveUseServer(ctx context.Context) error {
	e.opMu.Lock()
	defer e.opMu.Unlock()

	conflict, err := e.state.LoadConflict(e.cfg.Scope)
	if err != nil {
		return err
	}
	if conflict == nil {
		return ErrNoActiveConflict
	}
	if err := e.local.ApplyRemoteSnapshot(conflict.ServerJSON); err != nil {
		return fmt.Errorf("apply server conflict copy: %w", err)
	}
	outbox, err := e.state.LoadOutbox()
	if err != nil {
		return err
	}
	if err := e.state.SaveOutbox(ClearOutboxForScope(outbox, e.cfg.Scope)); err != nil {
		return err
	}
	cursor, err := e.state.LoadCursor(e.cfg.Scope)
	if err != nil {
		return err
	}
	cursor.ServerID = e.cfg.Scope.ServerID
	cursor.AccountID = e.cfg.Scope.AccountID
	cursor.WorkspaceID = e.cfg.Scope.WorkspaceID
	if conflict.ServerHeadSeq > cursor.AppliedSeq {
		cursor.AppliedSeq = conflict.ServerHeadSeq
	}
	cursor.LastSyncedHash = HashBytes(conflict.ServerJSON)
	if err := e.state.SaveCursor(cursor); err != nil {
		return err
	}
	if err := e.state.ClearConflict(e.cfg.Scope); err != nil {
		return err
	}
	e.setStatus(StatusSnapshot{
		Phase:        syncstate.PhaseSynced,
		Pending:      0,
		AppliedSeq:   cursor.AppliedSeq,
		LastSyncedAt: e.cfg.Now(),
	}, true)
	return nil
}

// ResolveMerge resolves an active conflict by performing a record-level merge
// of local and server copies, live-applying the merged dataset in place, and
// publishing it to v2 BlobSync.
func (e *Engine) ResolveMerge(ctx context.Context, prefer MergePreference) (MergeSummary, error) {
	e.opMu.Lock()
	defer e.opMu.Unlock()

	conflict, err := e.state.LoadConflict(e.cfg.Scope)
	if err != nil {
		return MergeSummary{}, err
	}
	if conflict == nil {
		return MergeSummary{}, ErrNoActiveConflict
	}
	if err := e.cipher.Ready(); err != nil {
		return MergeSummary{}, err
	}
	mergedJSON, summary, err := MergeDatasets(conflict.LocalJSON, conflict.ServerJSON, conflict.PendingChanges, conflict.CollidingKeys, prefer)
	if err != nil {
		return MergeSummary{}, err
	}
	if err := e.local.ApplyRemoteSnapshot(mergedJSON); err != nil {
		return summary, fmt.Errorf("apply merged dataset: %w", err)
	}
	if err := e.commitResolvedDatasetLocked(ctx, mergedJSON, conflict.ServerHeadSeq); err != nil {
		return summary, err
	}
	return summary, nil
}

func (e *Engine) commitResolvedDatasetLocked(ctx context.Context, resolvedJSON []byte, serverHead int64) error {
	cursor, err := e.state.LoadCursor(e.cfg.Scope)
	if err != nil {
		return err
	}
	cursor.ServerID = e.cfg.Scope.ServerID
	cursor.AccountID = e.cfg.Scope.AccountID
	cursor.WorkspaceID = e.cfg.Scope.WorkspaceID
	if serverHead > cursor.AppliedSeq {
		cursor.AppliedSeq = serverHead
	}
	outbox, err := e.state.LoadOutbox()
	if err != nil {
		return err
	}
	outbox = ClearOutboxForScope(outbox, e.cfg.Scope)
	outbox = EnqueueOutbox(outbox, OutboxEntry{
		ChangeID:     e.nextID(),
		ServerID:     e.cfg.Scope.ServerID,
		AccountID:    e.cfg.Scope.AccountID,
		WorkspaceID:  e.cfg.Scope.WorkspaceID,
		DeviceID:     e.cfg.DeviceID,
		BaseSeq:      cursor.AppliedSeq,
		Kind:         BatchKindSnapshot,
		SnapshotJSON: append(json.RawMessage(nil), resolvedJSON...),
		QueuedAt:     e.cfg.Now(),
	})
	if err := e.state.SaveOutbox(outbox); err != nil {
		return err
	}
	if err := e.state.SaveCursor(cursor); err != nil {
		return err
	}
	if err := e.state.ClearConflict(e.cfg.Scope); err != nil {
		return err
	}
	return e.syncOnceLocked(ctx)
}

func (e *Engine) nextID() string {
	if e.cfg.NewID != nil {
		return e.cfg.NewID()
	}
	e.deviceCounter++
	return fmt.Sprintf("%s-%d-%d", e.cfg.DeviceID, e.cfg.Now().UnixNano(), e.deviceCounter)
}

func materializeSnapshotWithBatches(baseSnapshotJSON []byte, batches []ChangeBatch) ([]byte, error) {
	if len(batches) == 0 {
		return baseSnapshotJSON, nil
	}
	db, err := store.NewMemory()
	if err != nil {
		return nil, fmt.Errorf("syncengine: open preview store: %w", err)
	}
	defer db.Close()
	if len(baseSnapshotJSON) > 0 {
		if err := loadJSONIntoStore(db, baseSnapshotJSON); err != nil {
			return nil, fmt.Errorf("syncengine: import preview snapshot: %w", err)
		}
	}
	for _, b := range batches {
		if b.Kind == BatchKindSnapshot {
			if err := loadJSONIntoStore(db, b.SnapshotJSON); err != nil {
				return nil, fmt.Errorf("syncengine: import batch snapshot: %w", err)
			}
			continue
		}
		if err := db.ReplayChanges(b.Changes); err != nil {
			return nil, fmt.Errorf("syncengine: replay preview batch %s: %w", b.ChangeID, err)
		}
	}
	out, err := exportJSONFromStore(db)
	if err != nil {
		return nil, fmt.Errorf("syncengine: export preview dataset: %w", err)
	}
	return out, nil
}
