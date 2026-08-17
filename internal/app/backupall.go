// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/restorepreview"
	"github.com/monstercameron/CashFlux/internal/store"
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

// marshalFullBackup gathers the whole install — every workspace's dataset, the
// workspace registry, and the device-local appearance side-state — into one
// versioned backup envelope and serialises it.
//
// Extracted so the plain and the passphrase-encrypted backups (LF-2) build the
// SAME bytes. Two builders would eventually diverge, and the divergence would
// only be discovered by someone restoring a backup and finding a piece of their
// install missing — the worst possible time and the worst possible way.
func marshalFullBackup() ([]byte, error) {
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
	return backup.MarshalEnvelope(env)
}

// backupEverything gathers the whole install — every workspace's dataset, the
// workspace registry, and the device-local appearance side-state — into one
// versioned backup envelope and downloads it as a single JSON file (L9). Unlike a
// single-workspace export, this is a lossless snapshot of the entire app, for
// moving to a new device or keeping a safety copy.
func backupEverything() {
	data, err := marshalFullBackup()
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
		// WF-BACKUP: say what the restore would REPLACE before doing it.
		// Restoring is the most destructive thing this app can do, and "are you
		// sure" is a question nobody can answer without being told what they are
		// about to lose.
		confirmModal(restoreConfirmMessage(env), true, func(ok bool) {
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

// restoreConfirmMessage builds the confirmation, with a before-and-after when
// the two datasets can be compared.
//
// Falls back to the plain warning when the backup's dataset cannot be read. A
// comparison the app is not sure of would be worse than none: this is the
// message somebody decides on, and a wrong count here is a wrong decision.
func restoreConfirmMessage(env backup.Envelope) string {
	base := uistate.T("backup.restoreConfirm")
	nowSide, ok1 := currentSide()
	backupSide, ok2 := envelopeSide(env)
	if !ok1 || !ok2 {
		return base
	}
	p := restorepreview.Build(nowSide, backupSide)
	if len(p.Lines) == 0 {
		return base
	}
	parts := make([]string, 0, len(p.Lines))
	for _, l := range p.Lines {
		parts = append(parts, uistate.T("restore.line", l.Now, l.After, l.Entity))
	}
	msg := base + " " + uistate.T("restore.compare", strings.Join(parts, ", ")) + "."
	if p.AnyLoss() {
		// The loss figure leads the warning, because it is the only number here
		// that describes something that cannot be got back.
		msg = base + " " + uistate.T("restore.loses", p.LostRecords) + " " +
			uistate.T("restore.compare", strings.Join(parts, ", ")) + "."
	}
	switch {
	case p.Stale:
		msg += " " + uistate.T("restore.stale", p.GapDays)
	case p.Newer:
		msg += " " + uistate.T("restore.newer", p.GapDays)
	}
	return msg
}

// currentSide counts the live dataset.
func currentSide() (restorepreview.Side, bool) {
	app := appstate.Default
	if app == nil {
		return restorepreview.Side{}, false
	}
	txns := app.Transactions()
	var newest time.Time
	for _, t := range txns {
		if t.Date.After(newest) {
			newest = t.Date
		}
	}
	return restorepreview.Side{
		Counts: restorepreview.Counts{
			Accounts: len(app.Accounts()), Transactions: len(txns),
			Budgets: len(app.Budgets()), Goals: len(app.Goals()),
			Categories: len(app.Categories()), Tasks: len(app.Tasks()),
			Holdings: len(app.Holdings()), Rules: len(app.Rules()),
		},
		Newest: newest,
	}, true
}

// envelopeSide counts the ACTIVE workspace's dataset inside a backup.
//
// The active one only: a restore replaces every workspace, but the comparison a
// person can actually check is against the data they are looking at. Counting
// all of them together would produce a total that matches nothing on screen.
func envelopeSide(env backup.Envelope) (restorepreview.Side, bool) {
	if len(env.Datasets) == 0 {
		return restorepreview.Side{}, false
	}
	idx := 0
	r := loadRegistry()
	for i, w := range r.Workspaces {
		if w.ID == r.ActiveID && i < len(env.Datasets) {
			idx = i
			break
		}
	}
	var ds store.Dataset
	if err := json.Unmarshal([]byte(env.Datasets[idx]), &ds); err != nil {
		return restorepreview.Side{}, false
	}
	var newest time.Time
	for _, t := range ds.Transactions {
		if t.Date.After(newest) {
			newest = t.Date
		}
	}
	return restorepreview.Side{
		Counts: restorepreview.Counts{
			Accounts: len(ds.Accounts), Transactions: len(ds.Transactions),
			Budgets: len(ds.Budgets), Goals: len(ds.Goals),
			Categories: len(ds.Categories), Tasks: len(ds.Tasks),
			Holdings: len(ds.Holdings), Rules: len(ds.Rules),
		},
		Newest: newest,
	}, true
}
