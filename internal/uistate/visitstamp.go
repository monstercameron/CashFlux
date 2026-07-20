// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strconv"

	"github.com/monstercameron/GoWebComponents/v4/state"
)

// Visit-baseline state for the dashboard "What changed since your last visit"
// card (E-DB): the baseline is the unix-second stamp everything is diffed
// against. It rolls forward only when a NEW visit starts — a reload minutes
// later keeps the same baseline, so the card keeps answering "since Friday"
// instead of "since two minutes ago" — and the user can also collapse it
// explicitly with MarkVisitCaughtUp ("Got it" — baseline := now).
//
// Two SQLite-KV keys (dataset appkv, like the notify last-seen stamp):
//   - cashflux:visit:baseline — the diff baseline the card renders against.
//   - cashflux:visit:lastOpen — when the app was last opened; used only to
//     decide whether this open starts a new visit.
const (
	visitBaselineKey = "cashflux:visit:baseline"
	visitLastOpenKey = "cashflux:visit:lastOpen"
)

// VisitGapSeconds is how long the app must have been closed for the next open
// to count as a new visit (and roll the baseline forward). 45 minutes: long
// enough that a tab refresh or a quick bounce keeps its context, short enough
// that a morning check and an evening check are distinct visits.
const VisitGapSeconds int64 = 45 * 60

// visitRolled guards the once-per-session roll (in-memory; resets on reload,
// which is exactly the cadence RollVisitBaseline needs).
var visitRolled bool

// UseVisitBaseline returns the shared atom mirroring the persisted baseline so
// the card re-renders when MarkVisitCaughtUp moves it.
func UseVisitBaseline() state.Atom[int64] {
	a := state.UseAtom("visit:baselineAtom", loadVisitStamp(visitBaselineKey))
	capturedVisitBaseline = a
	visitBaselineCaptured = true
	return a
}

var (
	capturedVisitBaseline state.Atom[int64]
	visitBaselineCaptured bool
)

// RollVisitBaseline is called once per session (any caller after the store is
// ready; the dashboard card does it on first render). If the app was last
// opened more than VisitGapSeconds ago, the previous open becomes the new
// baseline — "what changed since you were last here". Always stamps this open.
// Returns the effective baseline (0 = first-ever open, no baseline yet).
func RollVisitBaseline(now int64) int64 {
	lastOpen := loadVisitStamp(visitLastOpenKey)
	baseline := loadVisitStamp(visitBaselineKey)
	if !visitRolled {
		visitRolled = true
		if lastOpen > 0 && now-lastOpen >= VisitGapSeconds {
			baseline = lastOpen
			setVisitStamp(visitBaselineKey, baseline)
		}
		setVisitStamp(visitLastOpenKey, now)
		RequestPersist()
	}
	if visitBaselineCaptured && capturedVisitBaseline.Get() != baseline {
		capturedVisitBaseline.Set(baseline)
	}
	return baseline
}

// MarkVisitCaughtUp acknowledges the card ("Got it"): the baseline moves to
// now, so the card stays quiet until something new actually happens. Safe to
// call from event callbacks (never calls a hook).
func MarkVisitCaughtUp(now int64) {
	setVisitStamp(visitBaselineKey, now)
	RequestPersist()
	if visitBaselineCaptured {
		capturedVisitBaseline.Set(now)
	}
}

func loadVisitStamp(key string) int64 {
	raw := KVGet(key)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func setVisitStamp(key string, ts int64) {
	KVSet(key, strconv.FormatInt(ts, 10))
}
