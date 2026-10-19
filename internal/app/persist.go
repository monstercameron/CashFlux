// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// keptOnWipeKeys are the browser-store keys preserved by a data wipe: genuine
// settings/config, sync identity, the workspace registry, and the seed bootstrap
// gate. Everything else under "cashflux:" is financial data or derived from it — the
// dataset blob, the activity feed, dashboard layout/widget config, the widget-builder
// pages, filters, period, freshness dismissals — and is removed so a wipe leaves
// nothing behind on reload. ("non-settings data" must die.)
//
// Migration note: most of the config entries below (prefs, theme, fonts, language,
// the OpenAI/web-search keys, backup cadence, the notification toggle, …) now live in
// the dataset's PRESERVED settings KV (settingskv), so they survive a wipe WITH the
// dataset (see store.preservedOnWipe) rather than via these entries. The browser-store
// keys are retained here only as legacy migration sources — kept so a pre-migration
// install that wipes before those values have been read+migrated doesn't lose them.
// The entries that genuinely CANNOT live in the dataset (the single-source exemptions)
// are the seed gate, the workspace registry, the lock gate, and device-bound
// credentials / sync identity — see internal/uistate/kvbridge.go for the rationale.
var keptOnWipeKeys = map[string]bool{
	"cashflux:prefs":                 true,
	"cashflux:theme":                 true,
	"cashflux:fonts":                 true,
	"cashflux:active-lang":           true,
	"cashflux:languages":             true,
	"cashflux:applock":               true,
	"cashflux:openai-key":            true,
	aiCredentialStoreKey:             true,
	"cashflux:websearch-key":         true,
	"cashflux:cloud-ai-key-set":      true,
	"cashflux:cloud-mention-snoozed": true,
	"cashflux:chat-system-prompt":    true,
	"cashflux:rail-collapsed":        true,
	"cashflux:rail-tool-groups":      true,
	"cashflux:backupCadence":         true,
	"cashflux:muzak":                 true,
	"cashflux:muzak-pos":             true,
	"cashflux:muzak-volume":          true,
	"cashflux:banner":                true,
	"cashflux:notify:browser":        true,
	// cashflux:sampleActive is NOT kept: it moved into the dataset's app KV, so a wipe
	// clears it with the data (a wiped dataset is correctly no longer "the sample").
	"cashflux:seeded":         true,
	"cashflux:sync-device-id": true,
	"cashflux:sync-status":    true,
	"cashflux:sync-queue":     true,
	"cashflux:workspaces":     true,
	// WebAuthn PRF passkey state (C282) — preserved so the passkey survives a
	// financial-data wipe; the credential is worthless without the authenticator.
	"cashflux:webauthn-credid": true,
	"cashflux:webauthn-salt":   true,
	"cashflux:webauthn-vault":  true,
	// Per-member PIN map (C274) — PINs are a device-level access control layer;
	// wiping financial data must not lock members out of their own profiles.
	"cashflux:member-pins": true,
}

// keptOnWipePrefixes preserves whole families: other workspaces' bundled state and
// per-workspace sync metadata stay intact (a wipe targets the active workspace's
// financial data, not the multi-workspace registry).
var keptOnWipePrefixes = []string{
	"cashflux:ws-data:",
	"cashflux:sync-meta:",
	"cashflux:ui-preference:",
}

type wipeMode uint8

const (
	wipeKeepRecoveryCopies wipeMode = iota
	wipeEraseEverything
)

// wipeFinancialLocalState commits the already-wiped SQLite snapshot, cleared
// journal head, fresh generation and financial satellite tombstones atomically.
// Failure retains the original checkpoint and journal and keeps autosave paused:
// the emptied in-memory store must not overwrite the recoverable durable copy.
// then (usually a reload) runs only after the entire wipe is durable.
func wipeFinancialLocalState(then func(), modes ...wipeMode) {
	mode := wipeKeepRecoveryCopies
	if len(modes) > 0 {
		mode = modes[0]
	}
	generation := datasetMyGen
	suspendAutosave = true
	app := appstate.Default
	if app == nil {
		return
	}
	data, err := app.ExportJSONRedacted()
	if err != nil {
		app.Log().Error("data wipe could not be serialized; reload was stopped", "err", err)
		return
	}
	values := map[string]string{datasetStoreKey: string(data)}
	var remove []string
	// Empty values are encrypted tombstones, just like a cleared journal head.
	// Nothing is removed before the generation-conditional replacement commits.
	for _, key := range browserstore.Keys() {
		if !strings.HasPrefix(key, "cashflux:") || key == datasetStoreKey || keptOnWipeKeys[key] {
			continue
		}
		kept := false
		for _, p := range keptOnWipePrefixes {
			if strings.HasPrefix(key, p) {
				kept = true
				break
			}
		}
		if mode == wipeKeepRecoveryCopies && (strings.HasPrefix(key, quarantinePrefix) || strings.HasPrefix(key, journalRecoveryPrefix)) {
			kept = true
		}
		if !kept {
			if atrest.Classify(key) == atrest.ShouldNotPersist {
				remove = append(remove, key)
			} else {
				values[key] = ""
			}
		}
	}
	values, expected := prepareWorkspaceCommit(values, generation)
	browserstore.SetSecureBatch(values, expected, func(err error) {
		if err != nil {
			app.Log().Error("data wipe could not be committed; saved data was kept", "err", err)
			return
		}
		if appstate.Default != app {
			return
		}
		datasetMyGen = values[datasetGenKey]
		if mode == wipeEraseEverything {
			clearQuarantineState()
		}
		// Transient secrets cannot be written even as tombstones. Remove them
		// afterwards; failure retains the paused page and never reloads.
		removeFinancialKeysThen(remove, func() {
			if appstate.Default == app && datasetMyGen == values[datasetGenKey] && readDatasetGen() == values[datasetGenKey] && then != nil {
				then()
			}
		})
	})
}

func removeFinancialKeysThen(keys []string, done func()) {
	if len(keys) == 0 {
		done()
		return
	}
	browserstore.RemoveSecure(keys[0], func(err error) {
		if err != nil {
			if app := appstate.Default; app != nil {
				app.Log().Error("data wipe could not remove saved state", "err", err)
			}
			return
		}
		removeFinancialKeysThen(keys[1:], done)
	})
}

// datasetGenKey is an encrypted cross-tab generation identifier. Autosave writes
// it atomically with the dataset and conditionally compares the old generation
// inside the same transaction. Broadcast cache convergence is only a fast hint;
// the durable comparison prevents a stale tab from overwriting a newer save.
const datasetGenKey = "cashflux:dataset:gen"

// readDatasetGen reads the last observed generation ("" when unset).
func readDatasetGen() string {
	return browserstore.GetString(datasetGenKey)
}

// bumpDatasetGen stamps a fresh generation and returns it — called after every
// successful dataset write so other tabs know their in-memory copy is stale.
func bumpDatasetGen() string {
	g := id.New()
	browserstore.Set(datasetGenKey, g)
	return g
}

// datasetMyGen is the generation THIS tab is entitled to overwrite: seeded from
// the store at boot, advanced by this tab's own writes (autosave, sync apply,
// restore). datasetStaleNotified gates the one-time "another tab saved" toast.
var (
	datasetMyGen         string
	datasetStaleNotified bool
)

// datasetStoreKey is the localStorage key holding the autosaved dataset, so the
// app's data survives a page reload (previously every reload reset to the sample
// dataset). The OpenAI key is redacted before saving — it stays session-only.
const datasetStoreKey = "cashflux:dataset"

// seededFlagKey records that the sample has been seeded at least once, so a later
// wipe (empty dataset) is treated as an intentional clean slate rather than a
// first run that re-seeds the stranger's household (L6).
const seededFlagKey = "cashflux:seeded"

// suspendAutosave halts the dataset autosave. A workspace switch rewrites the
// localStorage keys then reloads; without this the dying page's pagehide/ticker
// save would write the *old* in-memory dataset back over the swapped-in one.
var suspendAutosave bool
var hadLocalDataset bool

// hydrateDataset loads the saved dataset from localStorage into the store, or
// seeds the sample dataset on first run (nothing saved yet) so a new household
// has something to explore. Call it after appstate.Init (with seed=false) and
// before mounting, so the first paint shows the user's real data.
func hydrateDataset() {
	app := appstate.Default
	if app == nil {
		return
	}
	hydrationSucceeded, hadLocalDataset = false, false
	if err := browserstore.InitError(); err != nil {
		app.Log().Error("dataset hydration requires protected storage recovery", "err", err)
		return
	}
	snapshot, snapshotErr := readDurableDatasetSnapshot()
	if snapshotErr != nil {
		pauseDatasetHydration(app, "snapshot read", snapshotErr)
		return
	}
	loadSnapshotQuarantine(snapshot)
	clearEmptyQuarantineState()
	raw := snapshot.Values[datasetStoreKey]
	if raw == "" && (snapshot.Errors[datasetStoreKey] != nil || snapshot.Values[datasetCheckpointBackupKey] != "") {
		err := hydrateDurableDataset(app, snapshot)
		if err == nil {
			hadLocalDataset, hydrationSucceeded = true, true
			loadJournalRecoveryCopies(hydratedDatasetState.snapshot)
			clearEmptyQuarantineState()
			initializeHostedHydration(false)
			return
		}
		if !errors.Is(err, errNoDatasetCheckpoint) {
			hadLocalDataset = false
			pauseDatasetHydration(app, "checkpoint recovery", err)
			return
		}
		quarantineDataset(raw, "No authenticated dataset checkpoint could be read. The protected records have been kept.")
		return
	}
	if raw == "" {
		head, _, err := snapshot.Read(datasetHeadKey)
		if err != nil || head != "" || hasUnclaimedJournal(snapshot, nil) {
			quarantineDataset(raw, "Saved changes are missing their dataset checkpoint. The saved records have been kept.")
			uistate.SetSampleActive(false)
			initializeHostedHydration(hostedDocument())
			return
		}
	}
	// Encrypted-at-rest dataset (C45): defer hydration until the passcode gate is
	// satisfied. We stash the ciphertext and leave the store empty; the autosave is
	// held off (see save's pendingEnvelopeRaw guard) so it can't overwrite it, and
	// onAppUnlocked decrypts + imports once the right passcode is entered.
	if cryptobox.IsEnvelope([]byte(raw)) {
		if _, ok := cryptobox.Parse([]byte(raw)); !ok {
			quarantineDataset(raw, unreadableDatasetReason([]byte(raw)))
			uistate.SetSampleActive(false)
			initializeHostedHydration(hostedDocument())
			return
		}
		hadLocalDataset = true
		pendingEnvelopeRaw = raw
		initializeHostedHydration(false)
		return
	}
	seededBefore := browserstore.GetString(seededFlagKey) != ""
	markSeeded := func() { browserstore.Set(seededFlagKey, "1") }
	allowSample := !hostedDocument()

	switch decideHydrate(raw, seededBefore, allowSample) {
	case hydrateImport:
		hadLocalDataset = true
		err := hydrateDurableDataset(app, snapshot)
		if err != nil {
			app.Log().Error("dataset hydrate failed", "err", err)
			hadLocalDataset = false
			if !errors.Is(err, errNoDatasetCheckpoint) {
				pauseDatasetHydration(app, "journal recovery", err)
				initializeHostedHydration(hostedDocument())
				return
			}
			reason := unreadableDatasetReason([]byte(raw))
			if reason == "" {
				reason = err.Error()
			}
			quarantineDataset(raw, reason)
			uistate.SetSampleActive(false)
			initializeHostedHydration(hostedDocument())
			return
		}
		hydrationSucceeded = true
		// NOTE (C1): do NOT force the sample-active flag off here. Autosave persists
		// the seeded sample as a real dataset, so a reload lands on hydrateImport
		// even when the user never personalised — clearing the flag here made the
		// "viewing sample data" banner vanish permanently after one reload. The
		// persisted localStorage flag is authoritative: it's set on seed and cleared
		// on personalise/dismiss/wipe/own-import, so we let it stand.
		markSeeded()
	case hydrateSeed:
		hadLocalDataset = false
		if err := app.LoadSample(); err != nil {
			app.Log().Error("seed sample failed", "err", err)
		} else {
			hydrationSucceeded = true
		}
		// Mark that we are showing sample data so the banner appears (L6).
		uistate.SetSampleActive(true)
		markSeeded()
	case hydrateEmpty:
		// Set up before, intentionally empty — preserve the clean slate (L6).
		hadLocalDataset = false
		hydrationSucceeded = true
		uistate.SetSampleActive(false)
	}
	if hydrationSucceeded && uistate.SampleActive() {
		if current, err := readDurableDatasetSnapshot(); err == nil {
			tryRecoverQuarantinedDataset(app, current)
		}
	}
	if hydratedDatasetState.app == app {
		loadJournalRecoveryCopies(hydratedDatasetState.snapshot)
	} else {
		loadJournalRecoveryCopies(snapshot)
	}
	clearEmptyQuarantineState()
	// A hosted first boot (including a sample cached by an older build) must not
	// mount financial screens until the account's server workspace is resolved.
	initializeHostedHydration(hostedDocument() && (!hadLocalDataset || uistate.SampleActive()))
}

// hydrateAIKey loads the device-global OpenAI credential after dataset hydration.
// An encrypted dataset isn't available until unlock, so defer migration until then.
func hydrateAIKey() {
	if pendingEnvelopeRaw != "" {
		return
	}
	migrateStandaloneAIKey()
}

// migrateStandaloneAIKey is retained for the post-unlock call site; it now loads
// or migrates the device credential instead of moving it into dataset state.
func migrateStandaloneAIKey() {
	hydrateDeviceAIKey()
}

// startDatasetAutosave flushes changed records through encrypted strict storage.
// Revision and cursor guards make repeated idle requests no-ops. Checkpoints are
// written for bootstrap, explicit rewrites and the one-time format upgrade.
func startDatasetAutosave() {
	app := appstate.Default
	if app == nil {
		return
	}
	save, force := newDatasetAutosave(app, browserstore.SetSecureBatch, pushActiveWorkspaceToBackend)
	resaveDataset = force
	uistate.CapturePersistNow(save)
	cb := js.FuncOf(func(js.Value, []js.Value) any {
		uistate.FlushPendingSettingsPersist()
		save()
		return nil
	})
	js.Global().Call("addEventListener", "pagehide", cb)
	js.Global().Call("addEventListener", "visibilitychange", cb)
	app.Log().Info("dataset hydration stage", "stage", "autosave armed", "paused", autosaveBlocked())
	save()
	go func() {
		for {
			time.Sleep(4 * time.Second)
			save()
		}
	}()
	go watchDataRevision(save)
}

// newDatasetAutosave owns the serial save policy. Its commit boundary is injected
// so the real production path can be tested with a delayed or rejected durable
// commit, without mounting a page or starting perpetual watcher goroutines.
func newDatasetAutosave(app *appstate.App,
	commit func(map[string]string, map[string]string, func(error)),
	push func([]byte, time.Time),
) (func(), func()) {
	return newRecordAutosave(app, commit, push)
}

// dataRevisionTick is how often the watcher looks for new edits. It is also the
// worst-case delay before a mutation is written, because the watcher writes on
// the tick that notices a change rather than waiting for one of silence.
const dataRevisionTick = 300 * time.Millisecond

// watchDataRevision persists the dataset shortly after every mutation instead of
// waiting for the 4-second ticker (C584).
//
// The ticker alone leaves a just-saved budget — or account, or transaction —
// unwritten for up to four seconds. The pagehide/visibilitychange handlers do not
// rescue it: the store is IndexedDB, whose writes are asynchronous, so a page
// that is closing cannot flush one. Add a budget and refresh straight away and it
// is simply gone, which is exactly the "it appeared to succeed" report behind
// C584 — and it was never specific to new categories; the existing-category path
// loses the same write on the same timing.
//
// Every mutation path already bumps the shared data revision to re-render the
// screens, so that counter is the app's own "something changed" signal and needs
// no new instrumentation at each write site. revisionWatcher owns the timing
// policy — leading edge then trailing edge — and carries the reasoning.
func watchDataRevision(save func()) {
	w := revisionWatcher{seen: uistate.CurrentDataRevision()}
	for {
		time.Sleep(dataRevisionTick)
		if w.tick(uistate.CurrentDataRevision()) {
			save()
		}
	}
}

func pauseDatasetHydration(app *appstate.App, stage string, err error) {
	hadLocalDataset, hydrationSucceeded = false, false
	app.Log().Error("dataset hydration paused; saved data was kept", "stage", stage, "err", err)
	uistate.ReportPersistFailure("dataset", nil)
	uistate.PostNotice(uistate.T("boot2.savedDataKept"), true)
}
