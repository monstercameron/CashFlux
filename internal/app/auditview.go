// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — auditbridge.go wires the session-scoped audit feed (C78 phase 4)
// between the undo stack (internal/app) and the Activity screen (internal/screens).
//
// Because internal/app already imports internal/screens (via addhost.go), the
// screens package cannot import internal/app without creating a cycle. The shared
// bridge lives in internal/auditview: both app and screens import it.
//
//	internal/app     → internal/auditview  (writes Feed, wires UndoFunc/CanUndoFunc)
//	internal/screens → internal/auditview  (reads  Feed, calls UndoFunc/CanUndoFunc)
//
// init() in this file wires the func slots after package initialisation, which
// runs before any user interaction; by the time a user navigates to /activity the
// slots are live.
//
// Phase-2 seam: captureUndoPoint in undo.go calls RecordAuditPoint(cs) after
// pushing to undoStack.  RecordAuditPoint feeds both the in-memory auditview.Feed
// and the SQLite audit_log table (C78 phase 3).  The applyingUndo guard in
// undo.go suppresses spurious entries during undo/redo restorations.
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/money"
)

func init() {
	// Wire the undo affordance slots so the Activity screen can trigger undo
	// without importing the app package directly.
	auditview.UndoFunc = func() bool { return undoLastChange() }
	auditview.CanUndoFunc = func() bool { return undoStack.CanUndo() }
	auditview.CaptureNow = func() { captureUndoPoint() }
}

// RecordAuditPoint translates cs into an auditlog.Entry, appends it to the
// shared in-memory feed (auditview.Feed), and persists it to the SQLite
// audit_log table via appstate.Default so the entry survives a page reload.
//
// The applyingUndo replay guard (from undo.go, same package) prevents undo/redo
// restorations from generating spurious audit entries: when applyingUndo is true
// the function returns immediately without recording anything.
//
// Phase-2 wiring: captureUndoPoint in undo.go calls RecordAuditPoint(cs) after
// undoStack.Push — that wiring is already in place.
func RecordAuditPoint(cs history.ChangeSet) {
	// Replay guard: undo/redo applications set applyingUndo to suppress the
	// captureUndoPoint→RecordAuditPoint path during a restoration write-back.
	if applyingUndo {
		return
	}
	if cs.IsEmpty() {
		return
	}
	// The audit log must not audit itself: a capture whose only content is the
	// previous entry's own auditEntries write is pure noise — skip it.
	onlyAudit := true
	for _, c := range cs.Changes {
		if c.Collection != "auditEntries" {
			onlyAudit = false
			break
		}
	}
	if onlyAudit {
		return
	}
	action, entityType, entityID := inferEntryFields(cs)
	summary := buildSummary(cs, action, entityType)
	e := auditlog.Entry{
		ID:         auditEntryID(),
		At:         time.Now().UTC(),
		Actor:      "user",
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    auditlog.Redact(summary),
		Details:    buildAuditDetails(cs),
	}
	// Append to the in-memory feed for the Activity screen.
	auditview.Feed.Append(e)
	// Persist to SQLite so the entry survives a page reload (C78 phase 3).
	if appstate.Default != nil {
		appstate.Default.RecordAudit(e)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

var auditSeq int

func auditEntryID() string {
	auditSeq++
	return fmt.Sprintf("ae-%d", auditSeq)
}

// inferEntryFields derives (action, entityType, entityID) from a ChangeSet.
func inferEntryFields(cs history.ChangeSet) (action, entityType, entityID string) {
	if len(cs.Changes) == 0 {
		return "changed", "data", ""
	}
	// Count ops over REAL entity changes only — a mutation's set also carries
	// internal _meta/audit rows (always adds), which mislabeled every update as
	// "Added". Fall back to all changes when the set touches only internals.
	opCount := map[history.Op]int{}
	collCount := map[string]int{}
	realOps := 0
	for _, c := range cs.Changes {
		collCount[c.Collection]++
		if strings.HasPrefix(c.Collection, "_meta:") || c.Collection == "auditEntries" {
			continue
		}
		opCount[c.Op]++
		realOps++
	}
	if realOps == 0 {
		for _, c := range cs.Changes {
			opCount[c.Op]++
		}
	}
	// Pick the dominant collection, preferring real entity collections over the
	// internal "_meta:*" scalar buckets (settings KV, schema version, …) and the
	// audit log's own storage, so a mixed change is described by what the user
	// actually touched (C355).
	domColl, domCount := "", 0
	for coll, n := range collCount {
		if strings.HasPrefix(coll, "_meta:") || coll == "auditEntries" {
			continue
		}
		if n > domCount {
			domColl, domCount = coll, n
		}
	}
	if domColl == "" { // change touched only internal buckets
		for coll, n := range collCount {
			if n > domCount {
				domColl, domCount = coll, n
			}
		}
	}
	entityType = singularize(domColl)
	switch {
	case opCount[history.OpDelete] >= opCount[history.OpAdd] && opCount[history.OpDelete] >= opCount[history.OpUpdate]:
		action = "deleted"
	case opCount[history.OpAdd] >= opCount[history.OpUpdate]:
		action = "added"
	default:
		action = "updated"
	}
	// EntityID: the single real change's row id (ignoring internal rows).
	realID, realSeen := "", 0
	for _, c := range cs.Changes {
		if strings.HasPrefix(c.Collection, "_meta:") || c.Collection == "auditEntries" {
			continue
		}
		realID = c.ID
		realSeen++
	}
	if realSeen == 1 {
		entityID = realID
	} else if len(cs.Changes) == 1 {
		entityID = cs.Changes[0].ID
	}
	return action, entityType, entityID
}

// buildSummary produces a plain-English one-liner from the ChangeSet label (when
// set by the Phase-2 commit seam) or a generic fallback.
func buildSummary(cs history.ChangeSet, action, entityType string) string {
	if cs.Label != "" {
		return cs.Label
	}
	// Count only the changes the entry is ABOUT: internal audit rows never
	// count, and _meta rows only count when nothing real was touched (a pure
	// settings change) — "Updated 2 member records" for one member + one
	// bookkeeping row was a lie.
	hasReal := false
	for _, c := range cs.Changes {
		if c.Collection != "auditEntries" && !strings.HasPrefix(c.Collection, "_meta:") {
			hasReal = true
			break
		}
	}
	n := 0
	for _, c := range cs.Changes {
		if c.Collection == "auditEntries" {
			continue
		}
		if hasReal && strings.HasPrefix(c.Collection, "_meta:") {
			continue
		}
		n++
	}
	if n <= 1 {
		// No raw record ID — "Added transaction tx_01H…" is machine-speak; the
		// entity type alone reads as the plain-English fallback (C355). Mass
		// nouns (settings) take no article — "Updated settings", not "a settings".
		if entityType == "settings" {
			return capitalize(action) + " settings"
		}
		return capitalize(action) + " a " + entityType
	}
	return fmt.Sprintf("%s %d %s records", capitalize(action), n, entityType)
}

// singularize maps snapshot collection names (plural) to singular display names.
func singularize(coll string) string {
	switch coll {
	case "transactions":
		return "transaction"
	case "accounts":
		return "account"
	case "budgets":
		return "budget"
	case "goals":
		return "goal"
	case "tasks":
		return "task"
	case "categories":
		return "category"
	case "members":
		return "member"
	case "rules":
		return "rule"
	case "documents":
		return "document"
	case "earmarks":
		return "earmark"
	case "conversations":
		return "conversation"
	case "customPages":
		return "page"
	case "artifacts":
		return "artifact"
	case "placements":
		return "dashboard layout"
	case "recurring":
		return "recurring item"
	case "workflows":
		return "workflow"
	case "notifications":
		return "notification"
	case "sharedExpenses":
		return "shared expense"
	case "settlements":
		return "settlement"
	case "auditEntries":
		return "history entry"
	default:
		if coll == "" {
			return "record"
		}
		// Internal scalar buckets ("_meta:settingsState", "_meta:schemaVersion",
		// …) must never leak their raw names into the user-facing feed (C355).
		if strings.HasPrefix(coll, "_meta:") {
			return "settings"
		}
		return coll
	}
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// auditSkipKeys are JSON fields excluded from before → after details: identity
// and blob-scale payloads that would drown the useful diff.
var auditSkipKeys = map[string]bool{
	"id": true, "bytes": true, "rows": true, "columns": true, "custom": false,
}

// buildAuditDetails derives the field-level before → after list for a recorded
// change. Details come from the ONE real entity UPDATE in the set (a mutation's
// change set often also carries internal _meta/audit rows — those are ignored,
// mirroring inferEntryFields' dominant-collection pick). Adds carry no
// "before", and multi-row bulk updates are already described by their summary.
func buildAuditDetails(cs history.ChangeSet) []auditlog.FieldChange {
	var upd *history.Change
	for i := range cs.Changes {
		c := &cs.Changes[i]
		if strings.HasPrefix(c.Collection, "_meta:") || c.Collection == "auditEntries" {
			continue
		}
		if c.Op != history.OpUpdate {
			continue
		}
		if upd != nil {
			return nil // more than one real update — a bulk change, no field diff
		}
		upd = c
	}
	if upd == nil || len(upd.Before) == 0 || len(upd.After) == 0 {
		return nil
	}
	det := auditlog.DiffJSON(upd.Before, upd.After, fmtAuditValue, auditSkipKeys)
	const maxDetails = 8
	if len(det) > maxDetails {
		det = det[:maxDetails]
	}
	return det
}

// fmtAuditValue renders one decoded JSON value for the activity feed with
// domain awareness: money shapes as formatted amounts, RFC3339 timestamps as
// dates, and *_Id references resolved to their display names (resolved at
// record time, so history stays accurate even if the target is later renamed).
func fmtAuditValue(key string, v any) string {
	switch t := v.(type) {
	case map[string]any:
		// money.Money marshals as {"amount": minor, "currency": "USD"}.
		if amt, ok := t["amount"].(float64); ok {
			if cur, ok2 := t["currency"].(string); ok2 && len(t) == 2 {
				return money.FormatMinor(int64(amt), currency.Decimals(cur)) + " " + cur
			}
		}
	case string:
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts.Format("Jan 2, 2006")
		}
		if name := resolveAuditRef(key, t); name != "" {
			return name
		}
	}
	return auditlog.DefaultValueFormatter(key, v)
}

// resolveAuditRef maps an ID-bearing field's value to a display name ("" when
// the field isn't a reference or the target is unknown).
func resolveAuditRef(key, id string) string {
	a := appstate.Default
	if a == nil || id == "" {
		return ""
	}
	switch key {
	case "categoryId", "parentId":
		for _, c := range a.Categories() {
			if c.ID == id {
				return c.Name
			}
		}
	case "accountId", "transferAccountId", "defaultAccountId":
		for _, acct := range a.Accounts() {
			if acct.ID == id {
				return acct.Name
			}
		}
	case "memberId", "ownerId", "defaultMemberId", "payerId":
		for _, m := range a.Members() {
			if m.ID == id {
				return m.Name
			}
		}
	}
	return ""
}
