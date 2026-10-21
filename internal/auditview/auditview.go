// SPDX-License-Identifier: MIT

// Package auditview is a shared bridge between the app and screens packages
// (C78 phase 4). It holds the session-scoped audit feed and the undo-action
// callback slots so that internal/app can write to them and internal/screens can
// read from them — without creating the import cycle that would result from
// screens importing app directly (app already imports screens via addhost.go).
//
// Architecture:
//
//	internal/app    → imports auditview (writes to Feed, sets UndoFunc/CanUndoFunc)
//	internal/screens → imports auditview (reads Feed, calls UndoFunc/CanUndoFunc)
//
// Neither app nor screens imports the other through this path.
//
// The slots (UndoFunc, CanUndoFunc) are wired by internal/app/auditview.go's
// init() after the undo stack is initialised.  They default to no-ops so the
// screen is safe to render before the app wires them up.
package auditview

import "github.com/monstercameron/CashFlux/internal/auditlog"

// Feed is the session-scoped in-memory audit log. internal/app appends entries
// to it via RecordAuditPoint; internal/screens reads it via Feed.Recent.
var Feed = auditlog.New(500)

// UndoFunc is called by the Activity screen when the user clicks "Undo this
// change".  It is wired to app.undoLastChange by internal/app/auditbridge.go's
// init() function.  The default is a no-op so the screen is safe before wiring.
var UndoFunc = func() bool { return false }

// CanUndoFunc reports whether the undo stack currently has an action to undo.
// Wired to app.undoStack.CanUndo by internal/app/auditbridge.go's init().
var CanUndoFunc = func() bool { return false }

// CaptureNow records an undo point immediately for the change that just happened,
// instead of waiting up to one autosave tick (~4s). Delete/change handlers call it
// right before showing an "Undo" toast so the undo stack is ready when the user
// clicks Undo (§6.8). Wired to app.captureUndoPoint; default is a no-op.
var CaptureNow = func() {}

// CaptureAssistantBaseline flushes any already-written household change before
// an approved Assistant tool starts. The actor is still the household then.
var CaptureAssistantBaseline = func() bool { return false }

// CaptureAssistantReceipt captures the just-finished Assistant tool mutation and
// returns its undo ID only when the filtered diff created a real undo point.
var CaptureAssistantReceipt = func() (undoID string, changed bool) { return "", false }

// TopUndoID returns the ID attached to the published top undo point.
var TopUndoID = func() string { return "" }

// UndoExpected starts an undo only when expectedID is still the published top
// undo point. Completion runs after the durable replay finishes.
var UndoExpected = func(expectedID string, done func(error)) bool { return false }
