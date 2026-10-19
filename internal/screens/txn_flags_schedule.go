// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type txnFlagsKey struct {
	app              *appstate.App
	revision         uint64
	dataRevision     int
	duplicate, spike bool
	weekStart        time.Weekday
}

type txnFlagsResult struct {
	key   txnFlagsKey
	value txnRowFlags
}

// Row flags are secondary decorations. Schedule their recomputation after the
// primary ledger paints, discard obsolete revisions, and cancel on unmount.
func useTxnRowFlags(app *appstate.App, dataRevision int, settings smart.Settings, weekStart time.Weekday) txnRowFlags {
	key := txnFlagsKey{app: app, revision: app.Rev(), dataRevision: dataRevision,
		duplicate: settings.IsEnabled(dupeFlagCode) && !settings.IsMuted(dupeFlagCode),
		spike:     settings.IsEnabled(spikeFlagCode) && !settings.IsMuted(spikeFlagCode), weekStart: weekStart}
	result := ui.UseState(txnFlagsResult{})
	ui.UseEffect(func() func() {
		cancelled := false
		timer := time.AfterFunc(300*time.Millisecond, func() {
			if cancelled || app.Rev() != key.revision {
				return
			}
			flags := txnRowFlagsMemo(app, dataRevision, settings, weekStart)
			if !cancelled && app.Rev() == key.revision {
				result.Set(txnFlagsResult{key: key, value: flags})
			}
		})
		return func() { cancelled = true; timer.Stop() }
	}, key)
	if result.Get().key != key {
		return txnRowFlags{}
	}
	return result.Get().value
}
