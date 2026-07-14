// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// registerDatasetKVBridge exposes window.cashfluxData{Get,Set,Remove} so vendored JS
// that owns LOW-FREQUENCY editor state — currently the widget-builder canvas node
// positions + viewport — persists it into the SQLite dataset's app KV, the single
// source of truth, so it travels with export/import + backups and hydrates on another
// client. Reads migrate any legacy browser-store value in on first access (KVGet).
//
// This is deliberately DISTINCT from the browserstore bridge (cashfluxStore*, wired by
// browserstore.RegisterJSBridge): the music player streams a position through that one
// every few seconds and only checkpoints it into the dataset coarsely (cashfluxMusicSave),
// so routing those high-frequency writes through the dataset would re-serialize (and
// re-encrypt) the whole blob on every tick. Canvas writes fire only on drag-end / a
// pan-zoom change, so writing straight to the dataset KV is cheap here.
func registerDatasetKVBridge() {
	js.Global().Set("cashfluxDataGet", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 {
			return nil
		}
		if v := uistate.KVGet(args[0].String()); v != "" {
			return v
		}
		return nil
	}))
	js.Global().Set("cashfluxDataSet", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) >= 2 {
			uistate.KVSet(args[0].String(), args[1].String())
		}
		return nil
	}))
	js.Global().Set("cashfluxDataRemove", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) >= 1 {
			uistate.KVDelete(args[0].String())
		}
		return nil
	}))
}
