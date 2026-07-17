// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
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
}

// wipeFinancialLocalState makes a data wipe authoritative and reload-proof. Call it
// AFTER the SQLite store's tables are cleared (app.Wipe). It (1) overwrites the
// persisted dataset blob with the now-empty SQLite snapshot so a reload can't
// restore it, then (2) removes every other "cashflux:" localStorage key that holds
// financial data or anything derived from it, preserving only genuine settings and
// the workspace/sync infrastructure (see keptOnWipeKeys / keptOnWipePrefixes).
//
// The persisted dataset stays the single source of truth in SQLite; this just stops
// the satellite keys from resurrecting wiped data.
// wipeFinancialLocalState clears the non-settings browser-store keys, persists the
// emptied dataset, and then runs `then` (typically a page reload) ONCE the dataset
// write has actually committed to IndexedDB — so the reload can't race the async
// write and re-hydrate the old data.
func wipeFinancialLocalState(then func()) {
	// 1) Remove every non-preserved cashflux:* key (bootstrap/settings stay).
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
		if !kept {
			browserstore.Remove(key)
		}
	}
	// 2) Persist the emptied store, then continue (reload) after it commits. A
	// deliberate wipe also bumps the cross-tab generation so any other open tab
	// stops autosaving its pre-wipe copy back over the emptied dataset.
	if app := appstate.Default; app != nil {
		if data, err := app.ExportJSONRedacted(); err == nil {
			bumpDatasetGen()
			browserstore.SetThen(datasetStoreKey, string(data), then)
			return
		}
	}
	then()
}

// datasetGenKey is a tiny cross-tab GENERATION STAMP for the autosaved dataset,
// kept in raw localStorage (deliberately NOT browserstore, whose in-memory cache
// is per-tab and never sees another tab's writes). Every dataset write bumps it;
// a tab may only overwrite the dataset while the stamp still matches the one it
// loaded (or last wrote). This is what stops a second, older tab — whose boot
// bookkeeping (recurring advancement etc.) dirties its own serialization — from
// flushing a stale whole-dataset copy over changes saved in the active tab
// (Cam's "Income to budget with doesn't persist", 2026-07-17: last-writer-wins
// clobber from a lingering tab).
const datasetGenKey = "cashflux:dataset:gen"

// readDatasetGen reads the live cross-tab generation stamp ("" when unset).
func readDatasetGen() string {
	defer func() { _ = recover() }()
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return ""
	}
	if v := ls.Call("getItem", datasetGenKey); v.Truthy() {
		return v.String()
	}
	return ""
}

// bumpDatasetGen stamps a fresh generation and returns it — called after every
// successful dataset write so other tabs know their in-memory copy is stale.
var datasetGenSeq int

func bumpDatasetGen() string {
	datasetGenSeq++
	g := fmt.Sprintf("%d-%d", time.Now().UnixNano(), datasetGenSeq)
	defer func() { _ = recover() }()
	if ls := js.Global().Get("localStorage"); ls.Truthy() {
		ls.Call("setItem", datasetGenKey, g)
	}
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
	raw := browserstore.GetString(datasetStoreKey)
	// Encrypted-at-rest dataset (C45): defer hydration until the passcode gate is
	// satisfied. We stash the ciphertext and leave the store empty; the autosave is
	// held off (see save's pendingEnvelopeRaw guard) so it can't overwrite it, and
	// onAppUnlocked decrypts + imports once the right passcode is entered.
	if cryptobox.IsEnvelope([]byte(raw)) {
		hadLocalDataset = true
		pendingEnvelopeRaw = raw
		return
	}
	seededBefore := browserstore.GetString(seededFlagKey) != ""
	markSeeded := func() { browserstore.Set(seededFlagKey, "1") }

	switch decideHydrate(raw, seededBefore) {
	case hydrateImport:
		hadLocalDataset = true
		if err := app.ImportJSON([]byte(raw)); err != nil {
			app.Log().Error("dataset hydrate failed; seeding sample", "err", err)
			hadLocalDataset = false
			_ = app.LoadSample()
			// Fallback to sample — show the banner so the user knows (L6).
			uistate.SetSampleActive(true)
		}
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
		}
		// Mark that we are showing sample data so the banner appears (L6).
		uistate.SetSampleActive(true)
		markSeeded()
	case hydrateEmpty:
		// Set up before, intentionally empty — preserve the clean slate (L6).
		hadLocalDataset = false
		uistate.SetSampleActive(false)
	}
}

// hydrateAIKey folds the legacy standalone OpenAI-key entry into the single source of
// truth (Settings.OpenAIKey in the SQLite dataset). The key used to be stored in a
// separate browser-store entry; it now lives in the dataset like every other setting.
// An encrypted dataset isn't loaded until the user unlocks, so defer the migration to
// onAppUnlocked in that case — never adopt into (or clear against) an unloaded store.
// Call after hydrateDataset so it lands on the loaded settings.
func hydrateAIKey() {
	if pendingEnvelopeRaw != "" {
		return // encrypted dataset awaiting unlock — migrate in onAppUnlocked instead
	}
	migrateStandaloneAIKey()
}

// migrateStandaloneAIKey adopts the legacy standalone OpenAI key into the loaded
// dataset (if the dataset doesn't already carry one) and then drops the standalone
// copy, so the dataset is the single source of truth. No-op once migrated.
func migrateStandaloneAIKey() {
	app := appstate.Default
	if app == nil {
		return
	}
	legacy := strings.TrimSpace(uistate.LoadAIKey())
	if legacy == "" {
		return
	}
	s := app.Settings()
	if strings.TrimSpace(s.OpenAIKey) == "" {
		s.OpenAIKey = legacy
		if err := app.PutSettings(s); err != nil {
			app.Log().Error("migrate ai key failed", "err", err)
			return // couldn't adopt — keep the standalone so the key isn't lost
		}
		// Flush the adopted key into the (possibly encrypted) dataset now, before we
		// drop the standalone, so closing the tab before the next autosave tick can't
		// lose it. No-op at boot (the autosave hook isn't wired yet); effective on the
		// post-unlock path, where the standalone is the only copy of the key.
		uistate.RequestPersist()
	}
	uistate.ClearAIKey()
}

// startDatasetAutosave persists the dataset (OpenAI key redacted) to localStorage
// so it survives a reload. It snapshots on a short ticker — which catches every
// mutation regardless of code path, without instrumenting each write — and on
// page hide, writing only when the serialized bytes change.
// localDatasetExport serializes the dataset for ON-DEVICE persistence. It keeps the
// OpenAI key with the data when the user has opted into remembering it (so the key
// rides the reliable autosave and is restored on the next boot alongside everything
// else, instead of depending on a single fire-and-forget key write). When "remember
// key" is off, the key is redacted so it stays session-only. The BACKEND push always
// uses ExportJSONRedacted — the key never leaves the device.
func localDatasetExport(app *appstate.App) ([]byte, error) {
	if uistate.LoadPrefs().RememberAIKey {
		return app.ExportJSON()
	}
	return app.ExportJSONRedacted()
}

func startDatasetAutosave() {
	app := appstate.Default
	if app == nil {
		return
	}
	last := ""
	if hadLocalDataset {
		data, err := localDatasetExport(app)
		if err != nil {
			return
		}
		last = string(data)
	}
	// Seed this tab's write entitlement from the store's current generation.
	datasetMyGen = readDatasetGen()
	save := func() {
		if suspendAutosave {
			return // a workspace switch is rewriting storage; don't clobber it
		}
		if pendingEnvelopeRaw != "" {
			return // an encrypted dataset is awaiting unlock — never overwrite it
		}
		if g := readDatasetGen(); g != datasetMyGen {
			// Another tab saved after this one loaded. Overwriting would silently
			// revert its changes (whole-dataset last-writer-wins), so this tab
			// stops persisting and says so once — reload to pick up the latest.
			if !datasetStaleNotified {
				datasetStaleNotified = true
				app.Log().Warn("dataset updated by another tab; this tab will not overwrite — reload to continue saving here")
				uistate.PostNotice(uistate.T("app.staleTabNotice"), true)
			}
			return
		}
		// localStorage.setItem can throw (e.g. quota exceeded on a very large
		// dataset), which surfaces as a Go panic — don't let it crash the app.
		defer func() {
			if r := recover(); r != nil {
				app.Log().Error("dataset autosave failed", "err", r)
			}
		}()
		data, err := localDatasetExport(app)
		if err != nil {
			return
		}
		s := string(data)
		if s == last {
			return
		}
		captureUndoPoint() // record an undo point for this mutation (C78)
		// afterWrite mirrors the post-save bookkeeping (backend push / first-save
		// flag) shared by the plaintext and encrypted paths. The cloud copy is always
		// redacted so the OpenAI key never leaves the device.
		afterWrite := func() {
			if hadLocalDataset {
				if redacted, rErr := app.ExportJSONRedacted(); rErr == nil {
					pushActiveWorkspaceToBackend(redacted, time.Now().UTC())
				}
			} else {
				hadLocalDataset = true
			}
		}
		if datasetEncryptionActive() {
			// Encrypt at rest (C45). Encryption is async; skip a tick if a prior one
			// is still running. `last` advances only on a successful write, so a
			// failed encrypt is retried on the next change.
			if encSaving {
				return
			}
			encSaving = true
			plaintext := append([]byte(nil), data...)
			target := s
			encryptDataset(plaintext, activePasscode, func(env []byte, encErr error) {
				encSaving = false
				if encErr != nil {
					app.Log().Error("dataset encrypt failed; not writing this tick", "err", encErr)
					return
				}
				// Encryption is async — re-check the stamp at write time so a save
				// that raced another tab's write still refuses to clobber it.
				if g := readDatasetGen(); g != datasetMyGen {
					return
				}
				defer func() {
					if r := recover(); r != nil {
						app.Log().Error("encrypted dataset write failed", "err", r)
					}
				}()
				browserstore.Set(datasetStoreKey, string(env))
				datasetMyGen = bumpDatasetGen()
				last = target
				afterWrite()
			})
			return
		}
		last = s
		browserstore.Set(datasetStoreKey, s)
		datasetMyGen = bumpDatasetGen()
		afterWrite()
	}
	// resaveDataset forces an immediate rewrite even when the dataset bytes are
	// unchanged — used when the encryption mode flips (passcode set/removed) so the
	// at-rest copy migrates plaintext↔envelope right away instead of waiting for the
	// next edit (C45).
	resaveDataset = func() {
		last = ""
		save()
	}
	// Expose an immediate-persist trigger to screens that can't reach this
	// unexported closure — used to flush a freshly loaded sample before a fast
	// reload can race the ticker and lose it (C2).
	uistate.CapturePersistNow(resaveDataset)
	cb := js.FuncOf(func(js.Value, []js.Value) any { save(); return nil })
	js.Global().Call("addEventListener", "pagehide", cb)
	js.Global().Call("addEventListener", "visibilitychange", cb)
	// Persist once immediately so a freshly seeded/imported dataset reaches the
	// (async) IndexedDB store right after boot — otherwise a reload within the 4s
	// tick could race the write and lose the seed. Subsequent saves run on the ticker.
	save()
	go func() {
		for {
			time.Sleep(4 * time.Second)
			save()
		}
	}()
}
