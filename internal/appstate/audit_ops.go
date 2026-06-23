// Package appstate — audit_ops.go exposes the audit-log persistence seam (C78).
//
// RecordAudit writes a single auditlog.Entry through to the SQLite audit_log
// table so it survives page reloads.  RecentAudit reads back the most recent n
// entries (newest-first) from the store.  LoadAuditIntoFeed hydrates the
// in-memory auditview.Feed from the persisted rows — called once during app
// startup so the Activity screen is populated after a reload.
package appstate

import (
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
)

// RecordAudit persists e to the audit_log SQLite table.  The summary must
// already have been passed through auditlog.Redact by the caller (RecordAuditPoint
// in internal/app does this).  Errors are logged and swallowed so a persistence
// failure never crashes the mutation that produced the entry.
func (a *App) RecordAudit(e auditlog.Entry) {
	if err := a.store.PutAuditEntry(e); err != nil {
		a.log.Error("audit: persist entry", "id", e.ID, "err", err)
	}
}

// RecentAudit returns at most n audit entries in reverse-chronological order
// (newest first) from the store.  When n ≤ 0 all stored entries are returned.
// Errors are logged and a nil slice is returned so callers always get a safe value.
func (a *App) RecentAudit(n int) []auditlog.Entry {
	entries, err := a.store.ListAuditEntries(n)
	if err != nil {
		a.log.Error("audit: list entries", "err", err)
		return nil
	}
	return entries
}

// LoadAuditIntoFeed populates the in-memory auditview.Feed from the persisted
// audit_log rows.  Call this once after the dataset has been hydrated from
// localStorage so that the Activity screen shows history from previous sessions.
// Entries are appended in chronological order (oldest first) so auditview.Feed's
// own internal ordering (newest-first via Recent) stays correct.
func (a *App) LoadAuditIntoFeed() {
	// ListAuditEntries returns newest-first; reverse to get oldest-first for Append.
	entries, err := a.store.ListAuditEntries(0)
	if err != nil {
		a.log.Error("audit: load feed", "err", err)
		return
	}
	for i := len(entries) - 1; i >= 0; i-- {
		auditview.Feed.Append(entries[i])
	}
	a.log.Info("audit: loaded persisted entries into feed", "count", len(entries))
}
