// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/viewcache"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// revKey identifies the app, stored-data revision, and browser-preference revision.
// The store counter is global, so two App instances can have the same revision.
// Preferences also affect computed views without necessarily changing the store.
// Append all remaining inputs (scope, date window, clock day) before caching.
func revKey(app *appstate.App) string {
	if app == nil {
		return "none:" + strconv.Itoa(uistate.CurrentDataRevision())
	}
	return strconv.FormatUint(app.InstanceID(), 10) + ":" +
		strconv.FormatUint(app.Rev(), 10) + ":" + strconv.Itoa(uistate.CurrentDataRevision())
}

// newViewCache bounds retained page models. Heavy indexes can request capacity one;
// smaller models default to eight recent contexts so scope changes stay warm.
func newViewCache[T any](capacity ...int) *viewcache.Cache[string, T] {
	limit := 8
	if len(capacity) > 0 {
		limit = capacity[0]
	}
	return viewcache.New[string, T](limit)
}

// useAfterSettle returns false on a component's first paint and true ~300ms later (past
// the 160ms route cross-fade), via a browser timer. Gate a page's heavy secondary / below-
// the-fold content on it so the primary content paints immediately on mount and the
// rest fills in once the page has settled — keeping expensive work (full-transaction
// scans, chart builds, long lists) off the mount critical path. `key` just names the
// deferral for the effect's dependency; hooks stay unconditional either way.
func useAfterSettle(key string) bool {
	_ = key // descriptive only; the effect re-arms via the ready state below
	ready := ui.UseState(false)
	// Depend on ready itself so the effect fires on mount (ready=false → schedule the
	// flip) AND re-arms on every re-mount (navigating away and back gives a fresh
	// ready=false), then no-ops once flipped. A constant dep could fire only once and
	// leave deferred content missing on a return visit.
	ui.UseEffect(func() func() {
		if !ready.Get() {
			return afterSettleTimer(300*time.Millisecond, func() { ready.Set(true) })
		}
		return nil
	}, ready.Get())
	return ready.Get()
}

func afterSettleTimer(delay time.Duration, settled func()) func() {
	active := true
	var callback js.Func
	callback = js.FuncOf(func(js.Value, []js.Value) any {
		if active {
			active = false
			settled()
		}
		callback.Release()
		return nil
	})
	timer := js.Global().Call("setTimeout", callback, delay.Milliseconds())
	return func() {
		if !active {
			return
		}
		active = false
		js.Global().Call("clearTimeout", timer)
		callback.Release()
	}
}

// memoByRev shares one computation across tiles and keeps recently used scopes
// warm. Keys must begin with revKey and include all remaining calculation inputs.
func memoByRev[T any](cache *viewcache.Cache[string, T], key string, compute func() T) T {
	return cache.GetOrCompute(key, compute)
}
