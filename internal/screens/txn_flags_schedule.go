// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type txnFlagsKey struct {
	app          *appstate.App
	workspace    string
	revision     uint64
	dataRevision int
	settings     string
	weekStart    time.Weekday
}

type txnFlagsResult struct {
	key   txnFlagsKey
	value txnRowFlags
}

func makeTxnFlagsKey(app *appstate.App, dataRevision int, settings smart.Settings, weekStart time.Weekday) txnFlagsKey {
	raw, _ := json.Marshal(settings)
	key := txnFlagsKey{app: app, workspace: activeImportProfileWorkspaceID(), dataRevision: dataRevision, settings: string(raw), weekStart: weekStart}
	if app != nil {
		key.revision = app.Rev()
	}
	return key
}

// scheduleTxnFlags yields after approximately 8 ms, including duplicate work.
// Its input is the complete ledger for key.app, independent of search, filters,
// ordering and pagination. Those changes cannot shrink or poison the flag map.
// Only a complete, still-current result is published. cancel stops every slice.
func scheduleTxnFlags(key txnFlagsKey, settings smart.Settings, current func() bool, publish func(txnRowFlags)) func() {
	cancelled := false
	var timer *time.Timer
	var work *txnFlagsWork
	var run func()
	run = func() {
		if cancelled || !current() {
			return
		}
		started := time.Now()
		defer txnPerfStart("flag-slice")()
		if work == nil {
			if key.app == nil {
				publish(txnRowFlags{})
				return
			}
			// Avoid the shared Smart input memo: two stores may have equal revisions.
			work = newTxnFlagsWork(txnFlagsInput(key.app, key.weekStart), settings)
		}
		for {
			if work.step() {
				if !cancelled && current() {
					publish(work.value)
				}
				return
			}
			if time.Since(started) >= 8*time.Millisecond {
				break
			}
		}
		timer = time.AfterFunc(time.Millisecond, run)
	}
	timer = time.AfterFunc(300*time.Millisecond, run)
	return func() {
		cancelled = true
		if timer != nil {
			timer.Stop()
		}
	}
}

// Row flags are secondary decorations. Keep completed flags while replacing
// them, but never carry one workspace's flags into another app or workspace.
func useTxnRowFlags(app *appstate.App, dataRevision int, settings smart.Settings, weekStart time.Weekday) txnRowFlags {
	key := makeTxnFlagsKey(app, dataRevision, settings, weekStart)
	result := ui.UseState(txnFlagsResult{})
	latest := ui.UseRef(key)
	latest.Set(key)
	ui.UseEffect(func() func() {
		// Freeze the maps with the same bytes that form the settings dependency.
		var captured smart.Settings
		_ = json.Unmarshal([]byte(key.settings), &captured)
		return scheduleTxnFlags(key, captured, func() bool {
			return latest.Get() == key && (app == nil || app.Rev() == key.revision) && activeImportProfileWorkspaceID() == key.workspace
		}, func(value txnRowFlags) { result.Set(txnFlagsResult{key: key, value: value}) })
	}, key)
	previous := result.Get()
	if previous.key.app != key.app || previous.key.workspace != key.workspace {
		return txnRowFlags{}
	}
	return previous.value
}
