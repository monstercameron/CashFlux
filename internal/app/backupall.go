// SPDX-License-Identifier: MIT

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
// artifact image bytes rehydrated from IndexedDB so the backup is fully
// self-contained), or "" when unavailable.
func activeDataset() string {
	app := appstate.Default
	if app == nil {
		return ""
	}
	data, err := app.ExportJSONRedactedWithBlobs()
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
	recordBackupNow() // C299: stamp so "Last backed up" reflects full backups too
	paletteNotify(uistate.T("backup.everythingDone"), false)
}

// restoreFromBackup picks a full-backup JSON file, confirms the (destructive)
// replace, writes its contents back into localStorage, and reloads so boot
// re-hydrates the restored install (L9). It's the inverse of backupEverything.
func restoreFromBackup() {
	pickFile(".json", func(data []byte) {
		env, err := backup.UnmarshalEnvelope(data)
		if err != nil {
			paletteNotify(uistate.T("backup.restoreErr"), true)
			return
		}
		confirmModal(uistate.T("backup.restoreConfirm"), true, func(ok bool) {
			if !ok {
				return
			}
			applyBackup(env)
			reloadPage()
		})
	})
}

// applyBackup writes a restored envelope back into localStorage: the workspace
// registry, the appearance side-state, and each workspace's dataset (the active
// one into the canonical key, the rest into their blobs, by registry order). The
// autosave is suspended first so the dying page can't write the old in-memory
// dataset back over what we restore.
func applyBackup(env backup.Envelope) {
	suspendAutosave = true
	if env.WorkspaceRegistry != "" {
		lsSet(workspacesKey, env.WorkspaceRegistry)
	}
	restoreAppearance(env.Appearance)

	r := loadRegistry()
	if len(r.Workspaces) == 0 {
		if len(env.Datasets) > 0 {
			lsSet(datasetStoreKey, env.Datasets[0])
			datasetMyGen = bumpDatasetGen()
		}
		return
	}
	for i, w := range r.Workspaces {
		if i >= len(env.Datasets) {
			break
		}
		if w.ID == r.ActiveID {
			lsSet(datasetStoreKey, env.Datasets[i])
			datasetMyGen = bumpDatasetGen()
		} else {
			saveBlob(w.ID, map[string]string{datasetStoreKey: env.Datasets[i]})
		}
	}
}

// restoreAppearance writes the device-local appearance keys, clearing any the
// backup didn't carry so a restore is a faithful replacement, not a merge.
func restoreAppearance(a backup.Appearance) {
	setOrClear(themeKey, a.Theme)
	setOrClear(fontsKey, a.Fonts)
	setOrClear(bannerKey, a.Banner)
	setOrClear(prefsKey, a.Prefs)
}

// setOrClear sets a localStorage key to val, or removes it when val is empty.
func setOrClear(key, val string) {
	if val != "" {
		lsSet(key, val)
		return
	}
	lsRemove(key)
}
