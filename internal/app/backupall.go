//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

// Appearance side-state keys — device-local localStorage values that live outside
// any workspace's dataset, so a full backup carries them too.
const (
	themeKey  = "cashflux:theme"
	fontsKey  = "cashflux:fonts"
	bannerKey = "cashflux:banner"
	prefsKey  = "cashflux:prefs"
)

// gatherBackupDatasets returns every workspace's dataset JSON. The active
// workspace's dataset is taken live (activeDS) so the backup is current even if the
// autosave ticker hasn't flushed it to localStorage yet; an inactive one's comes
// from its saved blob. With no registry yet (a fresh install), it backs up just the
// active dataset.
func gatherBackupDatasets(r workspace.Registry, activeDS string) []string {
	datasets := []string{}
	if len(r.Workspaces) == 0 {
		if activeDS != "" {
			datasets = append(datasets, activeDS)
		}
		return datasets
	}
	for _, w := range r.Workspaces {
		ds := loadBlob(w.ID)[datasetStoreKey]
		if w.ID == r.ActiveID {
			ds = activeDS
		}
		if ds != "" {
			datasets = append(datasets, ds)
		}
	}
	return datasets
}

// activeDataset serializes the live active-workspace dataset (OpenAI key redacted,
// matching the autosave), or "" when unavailable.
func activeDataset() string {
	app := appstate.Default
	if app == nil {
		return ""
	}
	data, err := app.ExportJSONRedacted()
	if err != nil {
		return ""
	}
	return string(data)
}

// backupEverything gathers the whole install — every workspace's dataset, the
// workspace registry, and the device-local appearance side-state — into one
// versioned backup envelope and downloads it as a single JSON file (L9). Unlike a
// single-workspace export, this is a lossless snapshot of the entire app, for
// moving to a new device or keeping a safety copy.
func backupEverything() {
	r := loadRegistry()
	env := backup.Envelope{
		Datasets:          gatherBackupDatasets(r, activeDataset()),
		WorkspaceRegistry: lsGet(workspacesKey),
		Appearance: backup.Appearance{
			Theme:  lsGet(themeKey),
			Fonts:  lsGet(fontsKey),
			Banner: lsGet(bannerKey),
			Prefs:  lsGet(prefsKey),
		},
	}
	data, err := backup.MarshalEnvelope(env)
	if err != nil {
		paletteNotify(uistate.T("backup.everythingErr"), true)
		return
	}
	downloadBytes("cashflux-backup.json", "application/json", data)
	paletteNotify(uistate.T("backup.everythingDone"), false)
}
