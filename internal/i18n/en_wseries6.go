// SPDX-License-Identifier: MIT

package i18n

// wseries6Keys holds the English copy for the W-series lane-6 tickets: C367
// (goal scenario "what if I add more?"), C368 (task repeat/reminder
// discoverability), C369 (visible notification snooze), C370 (token/cost
// honesty), plus the C364 changeset-apply toast carve-out.
//
// Kept in its own file and merged via init so it never touches the shared en.go.
// Because init runs in lexical filename order and this file sorts last, the keys
// here OVERRIDE any earlier definition of the same key — which is how the
// per-reply cost note is relabelled from "Used …" to "This reply: …" without
// editing en.go under concurrent work (mirrors the override precedent already in
// the catalog).
var wseries6Keys = Catalog{
	// ── C367 — goal scenario tool, surfaced ────────────────────────────────────
	// The contribution planner (slider + landing band) is the "what if I add more?"
	// scenario tool; label it in the reviewer's words on both the compact affordance
	// and the expanded disclosure so its purpose is unmistakable.
	"goals.whatIfMore":     "What if I add more?",
	"goals.whatIfMoreHide": "Hide what-if",

	// ── C364 (carve-out) — tell the undo story on changeset apply ──────────────
	// The completion toast names both reversal paths; the persistent receipt card
	// carries the working "Undo all" + "View in Activity" controls.
	"changeset.appliedUndo":  "Applied %d change(s). Undo (Ctrl+Z) · View in Activity",
	"changeset.viewActivity": "View in Activity",

	// ── C368 — task repeat/reminder discoverability ────────────────────────────
	// The Repeat + Remind controls anchor to a due date and stay hidden until one
	// is set — which is why the reviewer thought recurrence was missing. This hint
	// makes the capability discoverable in the add/edit forms.
	"todo.repeatNeedsDue": "Set a due date to add a repeat or reminder.",

	// ── C370 — token/cost honesty ──────────────────────────────────────────────
	// The per-message note now says "this reply" so one turn's spend reads as
	// distinct from the conversation-cumulative receipt ("This chat: …").
	"insights.usageCost":        "This reply: %d tokens · about %s",
	"insights.usageCostUnknown": "This reply: %d tokens · cost unavailable",
}

func init() {
	for k, v := range wseries6Keys {
		english[k] = v
	}
}
