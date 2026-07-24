// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/state"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// syncRevAtomID backs C324's reactivity: a revision counter bumped by setSyncStatus
// (from background goroutines) that SyncChip subscribes to so it re-renders on every
// status change rather than only when some unrelated render happens to occur.
const syncRevAtomID = "sync:rev"

var (
	capturedSyncRev    state.Atom[int]
	syncStatusCaptured bool
)

// syncChipFace maps a sync-status state to its label key + chip class. Unknown or
// empty states render nothing (sync isn't configured / not signed in), so the chip
// stays invisible until Cloud sync is actually in use.
func syncChipFace(state string) (labelKey, cls string, ok bool) {
	switch state {
	case "synced":
		return "sync.synced", "sync-chip sync-ok", true
	case "syncing":
		return "sync.syncing", "sync-chip sync-busy", true
	case "offline":
		return "sync.offline", "sync-chip sync-off", true
	case "conflict":
		return "sync.conflict", "sync-chip sync-warn", true
	case "locked":
		return "sync.locked", "sync-chip sync-warn", true
	case "error":
		return "sync.error", "sync-chip sync-err", true
	default:
		return "", "", false
	}
}

// syncClockAtomID ticks periodically (see syncClockTickMS) so the "last synced"
// line stays true without a sync event to trigger a re-render. Without it a
// tooltip rendered twenty minutes ago still claims "2m ago" — a liveness
// indicator that lies about liveness is worse than one that says nothing.
const syncClockAtomID = "sync:clock"

// syncLastSyncedLabel renders a stored RFC3339Nano stamp as something a person
// reads. It gives BOTH a relative age and a wall-clock time: the relative part
// is what answers "is this recent?" at a glance, and the absolute part is what
// survives a stale render — if the tick hasn't fired, "5m ago" may be wrong but
// "4:31 pm" never is. Returns "" for an absent or unparseable stamp, so the
// caller simply omits the line rather than showing a broken one.
func syncLastSyncedLabel(raw string, now time.Time) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return ""
	}
	// Stored UTC, shown in the viewer's zone — a timestamp in a zone the reader
	// isn't in is a puzzle, not information.
	local := t.Local()
	clock := local.Format("3:04 pm")
	d := now.Sub(local)
	switch {
	case d < time.Minute: // includes a small negative skew from a fast client clock
		return uistate.T("sync.agoJustNow", clock)
	case d < time.Hour:
		return uistate.T("sync.agoMinutes", int(d/time.Minute), clock)
	case d < 24*time.Hour:
		return uistate.T("sync.agoHours", int(d/time.Hour), clock)
	default:
		return uistate.T("sync.agoDate", local.Format("Jan 2"), clock)
	}
}

// syncStatusTooltip builds the hover/AX description shared by every sync
// indicator: what the state is, when it last synced, any error detail, and which
// server this household is talking to (§3.4 — a multi-server user needs to tell
// at a glance). Shared so the rail chip and the top-bar pulse can never drift
// into describing the same state two different ways.
//
// The last-synced line is ADDED to the state, not substituted for it: knowing
// you are "Synced" and knowing when that last happened are two different
// questions, and the old version answered only the second.
func syncStatusTooltip(st syncStatus) string {
	labelKey, _, ok := syncChipFace(st.State)
	tip := st.State
	if ok {
		if label := uistate.T(labelKey); label != labelKey {
			tip = label
		}
	}
	if st.Message != "" {
		tip = tip + " — " + st.Message
	}
	if when := syncLastSyncedLabel(st.LastSyncedAt, time.Now()); when != "" {
		tip = tip + "\n" + uistate.T("sync.lastSynced", when)
	}
	if host := backendHost(uistate.UsePrefs().Get().ServerURL); host != "" {
		tip = tip + "\n" + uistate.T("sync.server", host)
	}
	return tip
}

// SyncChip renders a compact Cloud-sync status indicator for the top bar / by the
// workspace switcher (§7.11): synced / syncing / offline / conflict / error, with a
// queued-count badge and a "last synced" tooltip. Clicking it triggers a Sync-now
// and opens the global settings panel (where Cloud sync is managed). Its own
// component so the click + atom hooks stay at a stable render position.
func SyncChip() uic.Node {
	// C324: subscribe to the revision atom so background setSyncStatus calls re-render
	// this chip; capture it for those out-of-render callers.
	rev := state.UseAtom(syncRevAtomID, 0)
	capturedSyncRev = rev
	syncStatusCaptured = true
	// Re-render on the shared minute tick as well, so this chip's "last synced"
	// line ages correctly between sync events (SyncPulse owns the ticker).
	_ = state.UseAtom(syncClockAtomID, 0).Get()
	st := loadSyncStatus()
	labelKey, cls, ok := syncChipFace(st.State)
	if !ok {
		return Fragment()
	}

	label := uistate.T(labelKey)
	if label == labelKey { // i18n key missing → fall back to the raw state
		label = st.State
	}
	tip := syncStatusTooltip(st)

	onClick := uic.UseEvent(func() {
		// In conflict state the chip opens the resolve modal (C309 / #464) rather
		// than the generic settings panel — the user needs to make an explicit choice.
		if cur := loadSyncStatus(); cur.State == "conflict" {
			openSyncConflict()
			return
		}
		requestBackendSyncNow()
		uistate.OpenGlobalSettingsAt("cloud")
	})

	args := []any{
		ClassStr(cls + " " + tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
		Type("button"),
		Attr("data-testid", "sync-chip"), // C321: stable e2e hook
		Attr("data-sync-state", st.State),
		Attr("title", tip),
		Attr("aria-label", tip),
		OnClick(onClick),
		Span(css.Class("sync-dot"), Attr("aria-hidden", "true")),
		Span(label),
	}
	if st.Pending > 0 {
		args = append(args, Span(css.Class("sync-pending"), strconv.Itoa(st.Pending)))
	}
	return Button(args...)
}
