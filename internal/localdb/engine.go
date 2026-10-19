// SPDX-License-Identifier: MIT

package localdb

import (
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/changes"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

const snapshotChangeID = "localdb-snapshot-v1"

func quarantine(store Store, raw []byte, reason string, cause error) error {
	if err := store.Quarantine(raw, reason); err != nil {
		return errors.Join(cause, fmt.Errorf("quarantine unreadable data: %w", err))
	}
	return cause
}

// Engine owns one workspace state in one scope. Callers serialize Commit and
// Compact for the lifetime of this instance.
type Engine struct {
	store      Store
	scope      Scope
	workspace  string
	key        *keyring.WorkspaceKey
	state      *changes.State
	ready      bool
	bootReport BootReport
}

// BootReport summarizes local log entries rejected during or before the most
// recent successful Boot. IDs are change IDs, with positions available in
// Records for precise recovery and support.
type BootReport struct {
	RejectedCount int
	RejectedIDs   []string
	Records       []RejectedRecord
}

// NewEngine constructs an engine for one fixed scope/workspace.
func NewEngine(store Store, scope Scope, workspaceID string) (*Engine, error) {
	if store == nil || workspaceID == "" {
		return nil, fmt.Errorf("new local engine: %w", ErrInvalid)
	}
	return &Engine{store: store, scope: scope, workspace: workspaceID}, nil
}

// Boot opens storage, imports the latest authenticated snapshot, and replays
// every later log record. Unreadable or misrouted ciphertext is quarantined
// and Boot fails closed.
func (e *Engine) Boot(wdk *keyring.WorkspaceKey) error {
	if wdk == nil || wdk.Destroyed() {
		return fmt.Errorf("boot local engine: %w", keyring.ErrInvalidKey)
	}
	if err := e.store.Open(e.scope); err != nil {
		return fmt.Errorf("open local store: %w", err)
	}
	key, err := wdk.DataKey()
	if err != nil {
		return fmt.Errorf("derive local data key: %w", err)
	}
	e.key = wdk
	e.state = changes.NewScopedState(changes.Scope{ServerID: e.scope.ServerID, AccountID: e.scope.AccountID}, e.workspace)
	snap, err := e.store.LatestSnapshot(e.workspace)
	covered := uint64(0)
	if err == nil {
		plain, openErr := keyring.Open(key, keyring.AAD(e.scope, e.workspace, snapshotChangeID, wdk.Version()), snap.Sealed)
		if openErr != nil {
			return quarantine(e.store, snap.Sealed, "snapshot authentication failed", fmt.Errorf("open local snapshot: %w", openErr))
		}
		parsed, parseErr := changes.UnmarshalSnapshot(plain)
		if parseErr != nil {
			return quarantine(e.store, snap.Sealed, "snapshot decode failed", fmt.Errorf("decode local snapshot: %w", parseErr))
		}
		if parsed.Scope != (changes.Scope{ServerID: e.scope.ServerID, AccountID: e.scope.AccountID}) || parsed.WorkspaceID != e.workspace {
			return quarantine(e.store, snap.Sealed, "snapshot scope or workspace mismatch", fmt.Errorf("snapshot scope/workspace mismatch: %w", changes.ErrScopeMismatch))
		}
		if err = e.state.ImportSnapshot(parsed); err != nil {
			return quarantine(e.store, snap.Sealed, "snapshot import failed", fmt.Errorf("import local snapshot: %w", err))
		}
		covered = snap.CoversLocalSeq
	} else if err != ErrNotFound {
		return fmt.Errorf("load local snapshot: %w", err)
	}
	entries, err := e.store.Since(covered)
	if err != nil {
		return fmt.Errorf("read local log: %w", err)
	}
	e.bootReport = BootReport{}
	for _, entry := range entries {
		if entry.WorkspaceID != e.workspace {
			continue
		}
		if entry.Rejected {
			e.addRejection(rejectionFor(e.scope, entry, entry.RejectReason))
			continue
		}
		plain, openErr := keyring.Open(key, keyring.AAD(e.scope, e.workspace, entry.ChangeID, wdk.Version()), entry.Sealed)
		if openErr != nil {
			if err := e.rejectEntry(entry, "change authentication failed"); err != nil {
				return err
			}
			continue
		}
		batch, parseErr := changes.Unmarshal(plain)
		if parseErr != nil {
			if err := e.rejectEntry(entry, "change decode failed"); err != nil {
				return err
			}
			continue
		}
		if batch.ChangeID != entry.ChangeID || batch.WorkspaceID != e.workspace || batch.Scope != (changes.Scope{ServerID: e.scope.ServerID, AccountID: e.scope.AccountID}) {
			if err := e.rejectEntry(entry, "change envelope scope mismatch"); err != nil {
				return err
			}
			continue
		}
		if applyErr := e.state.Apply(batch); applyErr != nil {
			if err := e.rejectEntry(entry, "change apply failed"); err != nil {
				return err
			}
			continue
		}
	}
	rejected, err := e.store.RejectedLogEntries()
	if err != nil {
		return fmt.Errorf("load rejected local changes: %w", err)
	}
	e.bootReport = BootReport{}
	for _, record := range rejected {
		if record.WorkspaceID == e.workspace {
			e.addRejection(record)
		}
	}
	e.ready = true
	return nil
}

func (e *Engine) rejectEntry(entry LogEntry, reason string) error {
	record, err := e.store.RejectLogEntry(entry.LocalSeq, reason)
	if err != nil {
		return fmt.Errorf("reject local change %s at sequence %d: %w", entry.ChangeID, entry.LocalSeq, err)
	}
	e.addRejection(record)
	return nil
}

func (e *Engine) addRejection(record RejectedRecord) {
	for _, existing := range e.bootReport.Records {
		if existing.LocalSeq == record.LocalSeq {
			return
		}
	}
	e.bootReport.Records = append(e.bootReport.Records, record)
	e.bootReport.RejectedIDs = append(e.bootReport.RejectedIDs, record.ChangeID)
	e.bootReport.RejectedCount = len(e.bootReport.Records)
}

// BootReport returns the rejected local log records observed at the most
// recent Boot. A nonzero count requires an explicit recovery or user notice.
func (e *Engine) BootReport() BootReport {
	out := e.bootReport
	out.Records = append([]RejectedRecord(nil), out.Records...)
	out.RejectedIDs = append([]string(nil), out.RejectedIDs...)
	return out
}

// Commit durably appends an encrypted batch before applying it in memory.
func (e *Engine) Commit(batch changes.Batch) error {
	if !e.ready {
		return fmt.Errorf("commit before boot: %w", ErrInvalid)
	}
	if batch.Scope != (changes.Scope{ServerID: e.scope.ServerID, AccountID: e.scope.AccountID}) || batch.WorkspaceID != e.workspace {
		return fmt.Errorf("commit scope/workspace mismatch: %w", changes.ErrScopeMismatch)
	}
	// Validate before the irreversible append so malformed input cannot leave
	// a durable record that this engine cannot apply until the next boot.
	validator := changes.NewScopedState(changes.Scope{ServerID: e.scope.ServerID, AccountID: e.scope.AccountID}, e.workspace)
	if err := validator.Apply(batch); err != nil {
		return fmt.Errorf("validate local change: %w", err)
	}
	plain, err := changes.Marshal(batch)
	if err != nil {
		return fmt.Errorf("encode local change: %w", err)
	}
	key, err := e.key.DataKey()
	if err != nil {
		return fmt.Errorf("derive local data key: %w", err)
	}
	sealed, err := keyring.Seal(key, keyring.AAD(e.scope, e.workspace, batch.ChangeID, e.key.Version()), plain)
	if err != nil {
		return fmt.Errorf("seal local change: %w", err)
	}
	if _, err = e.store.Append(e.workspace, batch.ChangeID, sealed, batch.HLC); err != nil {
		return fmt.Errorf("append local change: %w", err)
	}
	if err = e.state.Apply(batch); err != nil {
		return fmt.Errorf("apply durably appended change: %w", err)
	}
	return nil
}

// ApplyRemote authenticates a server change, stores it idempotently, then
// applies the decoded batch to state.
func (e *Engine) ApplyRemote(changeID string, sealed []byte, serverSeq uint64) error {
	if !e.ready {
		return fmt.Errorf("apply remote before boot: %w", ErrInvalid)
	}
	key, err := e.key.DataKey()
	if err != nil {
		return fmt.Errorf("derive local data key: %w", err)
	}
	plain, err := keyring.Open(key, keyring.AAD(e.scope, e.workspace, changeID, e.key.Version()), sealed)
	if err != nil {
		return quarantine(e.store, sealed, "remote change authentication failed", fmt.Errorf("open remote change: %w", err))
	}
	batch, err := changes.Unmarshal(plain)
	if err != nil {
		return quarantine(e.store, sealed, "remote change decode failed", fmt.Errorf("decode remote change: %w", err))
	}
	if batch.ChangeID != changeID || batch.Scope != (changes.Scope{ServerID: e.scope.ServerID, AccountID: e.scope.AccountID}) || batch.WorkspaceID != e.workspace {
		return quarantine(e.store, sealed, "remote change scope mismatch", fmt.Errorf("remote scope/workspace mismatch: %w", changes.ErrScopeMismatch))
	}
	validator := changes.NewScopedState(changes.Scope{ServerID: e.scope.ServerID, AccountID: e.scope.AccountID}, e.workspace)
	if err := validator.Apply(batch); err != nil {
		return quarantine(e.store, sealed, "remote change validation failed", fmt.Errorf("validate remote change: %w", err))
	}
	if _, err := e.store.AppendRemote(e.workspace, batch.ChangeID, sealed, serverSeq, batch.HLC); err != nil {
		return fmt.Errorf("append remote change: %w", err)
	}
	if err := e.state.Apply(batch); err != nil {
		return fmt.Errorf("apply remote change: %w", err)
	}
	return nil
}

// State returns the currently materialized state after Boot.
func (e *Engine) State() (*changes.State, error) {
	if !e.ready {
		return nil, fmt.Errorf("state before boot: %w", ErrInvalid)
	}
	return e.state, nil
}

// Compact writes an authenticated snapshot before removing covered acknowledged log entries.
func (e *Engine) Compact() error {
	if !e.ready {
		return fmt.Errorf("compact before boot: %w", ErrInvalid)
	}
	snap := e.state.ExportSnapshot()
	plain, err := changes.MarshalSnapshot(snap)
	if err != nil {
		return fmt.Errorf("encode local snapshot: %w", err)
	}
	key, err := e.key.DataKey()
	if err != nil {
		return fmt.Errorf("derive local data key: %w", err)
	}
	sealed, err := keyring.Seal(key, keyring.AAD(e.scope, e.workspace, snapshotChangeID, e.key.Version()), plain)
	if err != nil {
		return fmt.Errorf("seal local snapshot: %w", err)
	}
	entries, err := e.store.Since(0)
	if err != nil {
		return fmt.Errorf("read local log for compaction: %w", err)
	}
	var local, server uint64
	for _, x := range entries {
		if x.WorkspaceID == e.workspace {
			if x.LocalSeq > local {
				local = x.LocalSeq
			}
			if x.ServerSeq > server {
				server = x.ServerSeq
			}
		}
	}
	if err = e.store.Compact(e.workspace, sealed, local, server); err != nil {
		return fmt.Errorf("compact local store: %w", err)
	}
	return nil
}
