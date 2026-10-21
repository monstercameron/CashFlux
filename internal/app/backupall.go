// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/id"
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
func gatherBackupDatasets(r workspace.Registry, activeDS string) ([]string, error) {
	datasets := []string{}
	if len(r.Workspaces) == 0 {
		if activeDS != "" {
			datasets = append(datasets, activeDS)
		}
		return datasets, nil
	}
	app := appstate.Default
	for _, w := range r.Workspaces {
		ds := loadBlob(w.ID)[datasetStoreKey]
		if w.ID == r.ActiveID {
			ds = activeDS
		} else if ds != "" {
			if app == nil {
				return nil, errors.New("active dataset is unavailable")
			}
			complete, err := app.CompleteArtifactSnapshot([]byte(ds))
			if err != nil {
				return nil, fmt.Errorf("complete backup for workspace %s: %w", w.ID, err)
			}
			ds = string(complete)
		}
		if ds != "" {
			datasets = append(datasets, ds)
		}
	}
	return datasets, nil
}

// activeDataset serializes the live active-workspace dataset (OpenAI key redacted,
// artifact image bytes rehydrated from IndexedDB so the backup is fully
// self-contained), or "" when unavailable.
func activeDataset() (string, error) {
	app := appstate.Default
	if app == nil {
		return "", errors.New("active dataset is unavailable")
	}
	data, err := app.ExportJSONRedactedWithBlobs()
	if err != nil {
		return "", err
	}
	return string(data), nil
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
	active, err := activeDataset()
	if err != nil {
		return nil, err
	}
	datasets, err := gatherBackupDatasets(r, active)
	if err != nil {
		return nil, err
	}
	env := backup.Envelope{
		Datasets:          datasets,
		WorkspaceRegistry: lsGet(workspacesKey),
		Appearance: backup.Appearance{
			Theme:  lsGet(themeKey),
			Fonts:  lsGet(fontsKey),
			Banner: lsGet(bannerKey),
			// Stripped of the server URL, bearer token and CSRF token. A full
			// backup is the file most likely to end up in a sync folder, and it
			// carried a live access token in the clear.
			Prefs: strippedPrefsForBackup(),
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
	go func() {
		data, err := marshalFullBackup()
		if err != nil {
			paletteNotify(uistate.T("backup.everythingErr"), true)
			return
		}
		downloadBytes("cashflux-backup.json", "application/json", data)
		recordBackupNow() // C299: stamp so "Last backed up" reflects full backups too
		paletteNotify(uistate.T("backup.everythingDone"), false)
	}()
}

// restoreFromBackup picks a full-backup JSON file, confirms the (destructive)
// replace, writes its contents back into localStorage, and reloads so boot
// re-hydrates the restored install (L9). It's the inverse of backupEverything.
func restoreFromBackup() {
	restoreFromBackupEncrypted()
}

// applyBackup writes every restored key durably before reloading. The dataset
// generation is adopted only after the active dataset has committed.
func applyBackup(env backup.Envelope) {
	if datasetQuarantined {
		paletteNotify("This data is quarantined. Download or recover it before restoring a backup.", true)
		return
	}
	suspendAutosave = true
	var writes []workspaceKeyValue
	if env.WorkspaceRegistry != "" {
		writes = append(writes, workspaceKeyValue{workspacesKey, env.WorkspaceRegistry})
	}
	for _, pair := range []struct{ key, value string }{
		{themeKey, env.Appearance.Theme}, {fontsKey, env.Appearance.Fonts},
		{bannerKey, env.Appearance.Banner}, {prefsKey, stripConnectionPrefsJSON(env.Appearance.Prefs)},
	} {
		writes = append(writes, workspaceKeyValue{pair.key, pair.value})
	}
	r := loadRegistry()
	if env.WorkspaceRegistry != "" {
		var restored workspace.Registry
		if json.Unmarshal([]byte(env.WorkspaceRegistry), &restored) == nil {
			r = restored
		}
	}
	activeDataset := ""
	if len(r.Workspaces) == 0 {
		if len(env.Datasets) > 0 {
			activeDataset = env.Datasets[0]
			writes = append(writes, workspaceKeyValue{datasetStoreKey, activeDataset})
		}
	} else {
		for i, w := range r.Workspaces {
			if i >= len(env.Datasets) {
				break
			}
			if w.ID == r.ActiveID {
				activeDataset = env.Datasets[i]
				writes = append(writes, workspaceKeyValue{datasetStoreKey, activeDataset})
			} else {
				b, _ := json.Marshal(map[string]string{datasetStoreKey: env.Datasets[i]})
				writes = append(writes, workspaceKeyValue{wsBlobKey(w.ID), string(b)})
			}
		}
	}
	generation := id.New()
	if activeDataset != "" {
		writes = append(writes, workspaceKeyValue{datasetGenKey, generation})
	}
	reloadAfterDurableWrites(func(done func()) { writeKeyValuesThen(writes, done) }, func() {
		if activeDataset != "" {
			datasetMyGen = generation
		}
	}, reloadPage)
}

// strippedPrefsForBackup reads this device's prefs with the connection fields
// removed, ready to travel in a backup file.
func strippedPrefsForBackup() string {
	return stripConnectionPrefsJSON(lsGet(prefsKey))
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
