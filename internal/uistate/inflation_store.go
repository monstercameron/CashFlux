// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/inflation"
)

// inflationPctKey is the SettingKV key for the household's inflation assumption
// (FP-T2d).
//
// ONE value for the whole app, deliberately. Every long-horizon figure the app
// produces — a retirement pot, a twelve-month forecast, a goal reached in eight
// years — is deflated by the same rate, because a household that believes in 3%
// inflation on the planning page and 2% on the retirement card would be shown
// two answers to one question and given no way to tell which it meant. An
// assumption a caller can set independently is an assumption that will disagree
// with itself.
//
// PRESERVED, not dataset: it is a belief about the future, not a financial
// record, and losing it on a dataset wipe would cost thinking the app cannot
// reconstruct.
const inflationPctKey = "cashflux:inflation-pct"

// LoadInflationPct reads the household's inflation assumption, falling back to
// the package default when unset or unusable.
//
// An unusable STORED value falls back rather than refusing: the caller here is a
// surface with a figure to draw, and refusing would leave it with nothing to say
// about a rate the user can simply retype. The engine still refuses an unusable
// rate passed to it directly — that is the case where a nominal figure would be
// labelled as real, which is the confusion the package exists to remove.
func LoadInflationPct() float64 {
	raw := SettingKVGet(inflationPctKey)
	if raw == "" {
		return inflation.DefaultRatePct
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || !inflation.Valid(f) {
		return inflation.DefaultRatePct
	}
	return f
}

// SaveInflationPct persists the household's inflation assumption, clamping an
// unusable rate to the default so a stored value can never make a projection
// refuse on reload.
func SaveInflationPct(pct float64) {
	if !inflation.Valid(pct) {
		pct = inflation.DefaultRatePct
	}
	SettingKVSet(inflationPctKey, strconv.FormatFloat(pct, 'f', -1, 64))
	RequestPersist()
	BumpDataRevision()
}
