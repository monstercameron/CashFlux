//go:build js && wasm

package app

import (
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

// datasetStoreKey is the localStorage key holding the autosaved dataset, so the
// app's data survives a page reload (previously every reload reset to the sample
// dataset). The OpenAI key is redacted before saving — it stays session-only.
const datasetStoreKey = "cashflux:dataset"

// hydrateDataset loads the saved dataset from localStorage into the store, or
// seeds the sample dataset on first run (nothing saved yet) so a new household
// has something to explore. Call it after appstate.Init (with seed=false) and
// before mounting, so the first paint shows the user's real data.
func hydrateDataset() {
	app := appstate.Default
	if app == nil {
		return
	}
	v := js.Global().Get("localStorage").Call("getItem", datasetStoreKey)
	if v.IsNull() || v.IsUndefined() || v.String() == "" {
		if err := app.LoadSample(); err != nil {
			app.Log().Error("seed sample failed", "err", err)
		}
		return
	}
	if err := app.ImportJSON([]byte(v.String())); err != nil {
		app.Log().Error("dataset hydrate failed; seeding sample", "err", err)
		_ = app.LoadSample()
	}
}

// startDatasetAutosave persists the dataset (OpenAI key redacted) to localStorage
// so it survives a reload. It snapshots on a short ticker — which catches every
// mutation regardless of code path, without instrumenting each write — and on
// page hide, writing only when the serialized bytes change.
func startDatasetAutosave() {
	app := appstate.Default
	if app == nil {
		return
	}
	last := ""
	save := func() {
		// localStorage.setItem can throw (e.g. quota exceeded on a very large
		// dataset), which surfaces as a Go panic — don't let it crash the app.
		defer func() {
			if r := recover(); r != nil {
				app.Log().Error("dataset autosave failed", "err", r)
			}
		}()
		data, err := app.ExportJSONRedacted()
		if err != nil {
			return
		}
		if s := string(data); s != last {
			last = s
			js.Global().Get("localStorage").Call("setItem", datasetStoreKey, s)
		}
	}
	cb := js.FuncOf(func(js.Value, []js.Value) any { save(); return nil })
	js.Global().Call("addEventListener", "pagehide", cb)
	js.Global().Call("addEventListener", "visibilitychange", cb)
	go func() {
		for {
			time.Sleep(4 * time.Second)
			save()
		}
	}()
}
