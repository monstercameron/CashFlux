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
// GAP — RecordAuditPoint is not yet called automatically: captureUndoPoint (undo.go,
// an existing file that cannot be edited) does not call RecordAuditPoint.  As a
// result the audit feed (auditview.Feed) is empty today and the Activity screen
// falls back to its entity-synthesis path (transactions + tasks by timestamp).
//
// To wire it up properly, Phase 2 must either:
//
//	a) Edit undo.go's captureUndoPoint to call app.RecordAuditPoint(cs) after
//	   the undoStack.Push, OR
//	b) Introduce the appstate.App.commit(label, actor, mutate) seam (C78 phase 2)
//	   and call RecordAuditPoint from inside it.
package app

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/history"
)

func init() {
	// Wire the undo affordance slots so the Activity screen can trigger undo
	// without importing the app package directly.
	auditview.UndoFunc = func() bool { return undoLastChange() }
	auditview.CanUndoFunc = func() bool { return undoStack.CanUndo() }
}

// RecordAuditPoint translates cs into an auditlog.Entry and appends it to the
// shared audit feed (auditview.Feed).  It is called after a ChangeSet is pushed
// onto undoStack.
//
// Phase-2 wiring: edit captureUndoPoint in undo.go to call
// RecordAuditPoint(cs) right after undoStack.Push(cs).
func RecordAuditPoint(cs history.ChangeSet) {
	if cs.IsEmpty() {
		return
	}
	action, entityType, entityID := inferEntryFields(cs)
	summary := buildSummary(cs, action, entityType)
	auditview.Feed.Append(auditlog.Entry{
		ID:         auditEntryID(),
		Actor:      "user",
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    auditlog.Redact(summary),
	})
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
	opCount := map[history.Op]int{}
	collCount := map[string]int{}
	for _, c := range cs.Changes {
		opCount[c.Op]++
		collCount[c.Collection]++
	}
	domColl, domCount := "", 0
	for coll, n := range collCount {
		if n > domCount {
			domColl, domCount = coll, n
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
	if len(cs.Changes) == 1 {
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
	n := len(cs.Changes)
	if n == 1 {
		return capitalize(action) + " " + entityType + " " + cs.Changes[0].ID
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
	default:
		if coll == "" {
			return "record"
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
