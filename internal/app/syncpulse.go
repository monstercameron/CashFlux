// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// syncActivityAtomID backs the top-bar pulse: a counter bumped ONLY when bytes
// actually moved to or from the server, which is a strictly narrower signal than
// syncRevAtomID (bumped by every setSyncStatus call, including the no-op flush
// that fires on each tab focus and merely re-affirms "synced"). Flashing on that
// broader signal would light the indicator up for tab switches during which
// nothing was uploaded — the same species of reassuring-but-false readout as a
// chip that says "Synced" over an empty server.
const syncActivityAtomID = "sync:activity"

// syncFlashMS is how long the pulse stays lit after one sync event. A push
// completes in tens of milliseconds — far too fast to see — so the flash is
// LATCHED here rather than tied to the real duration of the transfer: the point
// is a legible "that just happened", not a progress bar. 450ms is the motion
// spec's ceiling for routine motion (--motion-narrative), the longest the scale
// allows, chosen because this is a glanceable confirmation rather than a
// response to something the user just clicked.
const syncFlashMS = 450

// syncClockTickMS re-renders the indicators periodically so the tooltip's
// relative "last synced" age stays true between syncs. The label's resolution is
// a minute, so this bounds how stale it can get, not how precise it is: at 30s
// the relative part is never wrong by more than half a minute. (Measured at 60s
// it read "just now" for a full 70 seconds — accurate to its own resolution, but
// visibly behind.) The wall-clock time beside it is exact regardless, which is
// why the tooltip carries both.
const syncClockTickMS = 30000

var (
	capturedSyncActivity state.Atom[int]
	syncActivityCaptured bool
)

// noteSyncActivity records that a sync just moved real data, lighting the
// top-bar pulse. Called from background goroutines, so like setSyncStatus it
// goes through a captured atom and no-ops until SyncPulse has mounted once.
func noteSyncActivity() {
	if syncActivityCaptured {
		capturedSyncActivity.Set(capturedSyncActivity.Get() + 1)
	}
}

// syncPulseTone maps a sync state to the class carrying its colour, reusing the
// chip's palette so one glance at either surface means the same thing. Empty
// means "sync isn't in use on this device" — the indicator renders nothing at
// all rather than an ambiguous grey dot, matching SyncChip's own invisible-
// until-configured rule.
func syncPulseTone(state string) (cls string, ok bool) {
	switch state {
	case "synced", "syncing":
		return "sync-ok", true
	case "offline":
		return "sync-off", true
	case "conflict", "locked":
		return "sync-warn", true
	case "error":
		return "sync-err", true
	default:
		return "", false
	}
}

// SyncPulse is the top-bar liveness indicator: a small cloud that sits quiet and
// dim, and flashes once each time data actually reaches (or arrives from) the
// server. It answers the question a static "Synced" label cannot — "is this
// thing still alive?" — without ever demanding attention: no layout shift, no
// text, no motion at rest, and nothing at all when cloud sync isn't in use.
//
// Deliberately NOT a progress spinner. Sync here is a sub-second background
// push; a spinner would either never be seen or would have to be faked into
// lasting long enough to notice, which is theatre. A single latched flash is
// honest about what happened and costs the eye nothing.
//
// Clicking it does what the rail chip does — sync now, and open Cloud settings —
// so the two indicators are never a choice the user has to think about.
func SyncPulse() uic.Node {
	// Subscribe to both signals: rev re-renders on any status change (colour),
	// activity drives the flash (liveness). Capture activity for the background
	// callers of noteSyncActivity, the same way SyncChip captures rev.
	rev := state.UseAtom(syncRevAtomID, 0)
	capturedSyncRev = rev
	syncStatusCaptured = true
	_ = rev.Get()

	activity := state.UseAtom(syncActivityAtomID, 0)
	capturedSyncActivity = activity
	syncActivityCaptured = true
	n := activity.Get()

	// Shared tick with SyncChip so both tooltips' "last synced" line ages correctly
	// between sync events. Mount-scoped — the interval is torn down with the
	// component, never left running.
	clock := state.UseAtom(syncClockAtomID, 0)
	_ = clock.Get()
	uic.UseEffect(func() func() {
		tick := js.FuncOf(func(js.Value, []js.Value) any {
			clock.Set(clock.Get() + 1)
			return nil
		})
		id := js.Global().Call("setInterval", tick, syncClockTickMS)
		return func() {
			js.Global().Call("clearInterval", id)
			tick.Release()
		}
	}, "sync-pulse-clock")

	flashing := uic.UseState(false)

	// One latched flash per activity bump. The effect key is the counter, so it
	// re-runs on every event — including two events in a row, where the cleanup
	// cancels the in-flight timer and the new one restarts the full 450ms rather
	// than letting the first timer cut the second flash short.
	uic.UseEffect(func() func() {
		if n == 0 { // nothing has synced yet this session — nothing to announce
			return nil
		}
		flashing.Set(true)
		fired := false
		var clear js.Func
		clear = js.FuncOf(func(js.Value, []js.Value) any {
			fired = true
			clear.Release()
			flashing.Set(false)
			return nil
		})
		id := js.Global().Call("setTimeout", clear, syncFlashMS)
		return func() {
			js.Global().Call("clearTimeout", id)
			if !fired {
				clear.Release()
			}
		}
	}, "sync-pulse-"+strconv.Itoa(n))

	st := loadSyncStatus()
	tone, ok := syncPulseTone(st.State)
	if !ok {
		// A persistent, hidden slot — NOT an empty Fragment. A component whose first
		// render is an empty Fragment gets APPENDED to the container's end when it
		// later mounts, rather than inserted at its source position (the reconciler
		// behavior logged in DEVLOG 2026-07-19), which parked this indicator past the
		// ⋯ menu at the far right of the bar the first time sync was configured. The
		// always-present span pins the slot; LockToggle does the same for the same
		// reason.
		return Span(ClassStr("sync-pulse-slot"), Attr("style", "display:none"))
	}

	cls := "sync-pulse " + tone
	if flashing.Get() {
		cls += " sync-pulse-flash"
	}
	// A queue that is not draining is the one state worth showing continuously:
	// it means work is outstanding, not that work is happening.
	if st.State == "syncing" {
		cls += " sync-pulse-busy"
	}

	onClick := uic.UseEvent(func() {
		if cur := loadSyncStatus(); cur.State == "conflict" {
			openSyncConflict()
			return
		}
		requestBackendSyncNow()
		uistate.OpenGlobalSettingsAt("cloud")
	})

	tip := syncStatusTooltip(st)
	args := []any{
		ClassStr(cls),
		Type("button"),
		Attr("data-testid", "sync-pulse"),
		Attr("data-sync-state", st.State),
		// The flash is decorative; the STATE is what assistive tech is told, and
		// only politely — a background sync is never worth interrupting for.
		Attr("role", "status"),
		Attr("aria-live", "polite"),
		Attr("title", tip),
		Attr("aria-label", tip),
		OnClick(onClick),
		ui.Icon(icon.Cloud, css.Class(tw.W18px, tw.H18px)),
	}
	if st.Pending > 0 {
		args = append(args, Span(css.Class("sync-pulse-count"), strconv.Itoa(st.Pending)))
	}
	// Same wrapper element in both branches, so becoming visible is a change of the
	// slot's CONTENTS rather than a mount — which is what keeps it in place.
	return Span(ClassStr("sync-pulse-slot"), Button(args...))
}
