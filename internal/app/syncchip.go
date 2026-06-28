// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	uic "github.com/monstercameron/GoWebComponents/ui"
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
	case "error":
		return "sync.error", "sync-chip sync-err", true
	default:
		return "", "", false
	}
}

// SyncChip renders a compact Cloud-sync status indicator for the top bar / by the
// workspace switcher (§7.11): synced / syncing / offline / conflict / error, with a
// queued-count badge and a "last synced" tooltip. Clicking it triggers a Sync-now
// and opens the global settings panel (where Cloud sync is managed). Its own
// component so the click + atom hooks stay at a stable render position.
func SyncChip() uic.Node {
	settings := uistate.UseSettings()
	// C324: subscribe to the revision atom so background setSyncStatus calls re-render
	// this chip; capture it for those out-of-render callers.
	rev := state.UseAtom(syncRevAtomID, 0)
	capturedSyncRev = rev
	syncStatusCaptured = true
	st := loadSyncStatus()
	labelKey, cls, ok := syncChipFace(st.State)
	if !ok {
		return Fragment()
	}

	label := uistate.T(labelKey)
	if label == labelKey { // i18n key missing → fall back to the raw state
		label = st.State
	}
	tip := label
	if st.LastSyncedAt != "" {
		tip = uistate.T("sync.lastSynced", st.LastSyncedAt)
	}
	if st.Message != "" {
		tip = tip + " — " + st.Message
	}
	// Name the active server in the tooltip so a multi-server user can tell at a
	// glance which backend this household is syncing with (§3.4).
	if host := backendHost(uistate.UsePrefs().Get().ServerURL); host != "" {
		tip = tip + "\n" + uistate.T("sync.server", host)
	}

	onClick := uic.UseEvent(func() {
		// In conflict state the chip opens the resolve modal (C309 / #464) rather
		// than the generic settings panel — the user needs to make an explicit choice.
		if cur := loadSyncStatus(); cur.State == "conflict" {
			openSyncConflict()
			return
		}
		requestBackendSyncNow()
		settings.Set(uistate.Global())
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
