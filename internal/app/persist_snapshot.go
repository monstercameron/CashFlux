// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "github.com/monstercameron/CashFlux/internal/appstate"

// The local, undo and backend paths all consume the same credential-free JSON.
// Retain a single immutable export per revision instead of reading every table
// again for each consumer. A failed export never advances the cache key.
var datasetExportMemo struct {
	app    *appstate.App
	rev    uint64
	cursor uint64
	data   []byte
}

var persistPerfObserve func(stage string)

func localDatasetExport(app *appstate.App) ([]byte, error) {
	cursor, err := app.Store().ChangeCursor()
	if err != nil {
		return nil, err
	}
	if datasetExportMemo.app == app && datasetExportMemo.rev == app.Rev() && datasetExportMemo.cursor == cursor && datasetExportMemo.data != nil {
		return datasetExportMemo.data, nil
	}
	if persistPerfObserve != nil {
		persistPerfObserve("dataset-export")
	}
	data, err := app.ExportJSONRedacted()
	if err != nil {
		return nil, err
	}
	datasetExportMemo.app, datasetExportMemo.rev, datasetExportMemo.data, datasetExportMemo.cursor = app, app.Rev(), data, cursor
	return data, nil
}
