// SPDX-License-Identifier: MIT

// Package auditview — actor.go adds the session-actor seam (AG20). A mutation
// made on the user's behalf by the assistant should be marked "via assistant" in
// the audit trail so households can see exactly what the agent touched. The actor
// is an ambient, session-scoped override that appstate.ApplyChangeset sets for
// the duration of an agent-applied changeset; internal/app's RecordAuditPoint
// reads it when stamping each entry. Kept in this shared bridge package so both
// appstate (writer) and app (reader) can reach it without an import cycle.
package auditview

// ActorAssistant is the audit Actor value stamped on agent-made mutations.
const ActorAssistant = "assistant"

// ActorRule marks mutations made by the rules engine's backfill/apply paths
// (#54): the audit trail distinguishes "a rule recategorized this" from a
// manual edit.
const ActorRule = "rule"

// ActorImport marks mutations made by a file import (#54): rows created by the
// CSV/receipt import paths rather than typed in by a person.
const ActorImport = "import"

// sessionActor overrides the recorded Actor while set. Empty means the normal
// actor ("user"). Access is single-goroutine (the UI/tool loop), so no locking.
var sessionActor string

// SetSessionActor sets (or clears, with "") the ambient actor tag applied to
// mutations captured while it is non-empty. Callers must reset it (defer
// SetSessionActor("")) so unrelated later mutations aren't mis-tagged.
func SetSessionActor(actor string) { sessionActor = actor }

// SessionActor returns the current ambient actor override ("" when none).
func SessionActor() string { return sessionActor }

// IsAssistantSession reports whether the current mutation is being made via the
// assistant.
func IsAssistantSession() bool { return sessionActor == ActorAssistant }
