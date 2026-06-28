// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// keptOnWipeKeys are the localStorage keys preserved by a data wipe: genuine
// settings/config, sync identity, the workspace registry, and the sample/seed
// control flags. Everything else under "cashflux:" is financial data or derived
// from it — the dataset blob, the activity feed, dashboard layout/widget config,
// the widget-builder pages, filters, period, freshness dismissals — and is removed
// so a wipe leaves nothing behind on reload. ("non-settings data" must die.)
var keptOnWipeKeys = map[string]bool{
	"cashflux:prefs":                   true,
	"cashflux:theme":                   true,
	"cashflux:fonts":                   true,
	"cashflux:active-lang":             true,
	"cashflux:languages":               true,
	"cashflux:applock":                 true,
	"cashflux:openai-key":              true,
	"cashflux:websearch-key":           true,
	"cashflux:cloud-ai-key-set":        true,
	"cashflux:cloud-mention-snoozed": true,
	"cashflux:chat-system-prompt":      true,
	"cashflux:rail-collapsed":          true,
	"cashflux:rail-tool-groups":        true,
	"cashflux:backupCadence":           true,
	"cashflux:muzak":                   true,
	"cashflux:muzak-pos":               true,
	"cashflux:muzak-volume":            true,
	"cashflux:banner":                  true,
	"cashflux:notify:browser":          true,
	"cashflux:sampleActive":            true,
	"cashflux:seeded":                  true,
	"cashflux:sync-device-id":          true,
	"cashflux:sync-status":             true,
	"cashflux:sync-queue":              true,
	"cashflux:workspaces":              true,
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
	// 2) Persist the emptied store, then continue (reload) after it commits.
	if app := appstate.Default; app != nil {
		if data, err := app.ExportJSONRedacted(); err == nil {
			browserstore.SetThen(datasetStoreKey, string(data), then)
			return
		}
	}
	then()
}

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

// hydrateAIKey restores the saved OpenAI key into the session when the user has
// opted into remembering it on this device (the dataset autosave redacts the
// key, so it is stored separately). No-op when the toggle is off or nothing is
// stored. Call after hydrateDataset so it lands on the loaded settings.
func hydrateAIKey() {
	app := appstate.Default
	if app == nil || !uistate.LoadPrefs().RememberAIKey {
		return
	}
	key := uistate.LoadAIKey()
	if key == "" {
		return
	}
	s := app.Settings()
	s.OpenAIKey = key
	if err := app.PutSettings(s); err != nil {
		app.Log().Error("restore ai key failed", "err", err)
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
	if hadLocalDataset {
		data, err := app.ExportJSONRedacted()
		if err != nil {
			return
		}
		last = string(data)
	}
	save := func() {
		if suspendAutosave {
			return // a workspace switch is rewriting storage; don't clobber it
		}
		if pendingEnvelopeRaw != "" {
			return // an encrypted dataset is awaiting unlock — never overwrite it
		}
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
		s := string(data)
		if s == last {
			return
		}
		captureUndoPoint() // record an undo point for this mutation (C78)
		// afterWrite mirrors the post-save bookkeeping (backend push / first-save
		// flag) shared by the plaintext and encrypted paths.
		afterWrite := func() {
			if hadLocalDataset {
				pushActiveWorkspaceToBackend(data, time.Now().UTC())
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
				defer func() {
					if r := recover(); r != nil {
						app.Log().Error("encrypted dataset write failed", "err", r)
					}
				}()
				browserstore.Set(datasetStoreKey, string(env))
				last = target
				afterWrite()
			})
			return
		}
		last = s
		browserstore.Set(datasetStoreKey, s)
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
