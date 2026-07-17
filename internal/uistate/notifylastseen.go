// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strconv"

	"github.com/monstercameron/GoWebComponents/v4/state"
)

// notifyLastSeenKey is the SQLite-backed KV key that persists the unix-second
// timestamp of the last time the user viewed the Notification Center (C271).
// Lives here (not in screens) because the shell's bell badge also reads it:
// since QA CF-04 the badge counts items NEW since this stamp instead of unread
// items, so visiting the center calms the bell without touching per-item read
// state.
const notifyLastSeenKey = "cashflux:notify:lastSeen"

// UseNotifyLastSeen returns the shared atom mirroring the persisted last-seen
// stamp, so the bell badge re-renders the moment the Notification Center
// stamps a visit (KV alone wouldn't re-render subscribers).
func UseNotifyLastSeen() state.Atom[int64] {
	a := state.UseAtom("notify:lastSeenAtom", LoadNotifyLastSeen())
	capturedNotifyLastSeen = a
	notifyLastSeenCaptured = true
	return a
}

var (
	capturedNotifyLastSeen state.Atom[int64]
	notifyLastSeenCaptured bool
)

// SetNotifyLastSeen persists the stamp and pushes it into the live atom (safe
// to call from effects/callbacks — never calls a hook).
func SetNotifyLastSeen(ts int64) {
	KVSet(notifyLastSeenKey, strconv.FormatInt(ts, 10))
	if notifyLastSeenCaptured {
		capturedNotifyLastSeen.Set(ts)
	}
}

// LoadNotifyLastSeen reads the persisted last-seen timestamp. Returns 0 when
// absent/unparseable (treat as first-ever open).
func LoadNotifyLastSeen() int64 {
	raw := KVGet(notifyLastSeenKey)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
