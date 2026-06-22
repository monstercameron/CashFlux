//go:build js && wasm

package app

import (
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
	ls := js.Global().Get("localStorage")
	raw := ""
	if v := ls.Call("getItem", datasetStoreKey); !v.IsNull() && !v.IsUndefined() {
		raw = v.String()
	}
	// Encrypted-at-rest dataset (C45): defer hydration until the passcode gate is
	// satisfied. We stash the ciphertext and leave the store empty; the autosave is
	// held off (see save's pendingEnvelopeRaw guard) so it can't overwrite it, and
	// onAppUnlocked decrypts + imports once the right passcode is entered.
	if cryptobox.IsEnvelope([]byte(raw)) {
		hadLocalDataset = true
		pendingEnvelopeRaw = raw
		return
	}
	seededBefore := false
	if f := ls.Call("getItem", seededFlagKey); !f.IsNull() && !f.IsUndefined() && f.String() != "" {
		seededBefore = true
	}
	markSeeded := func() { ls.Call("setItem", seededFlagKey, "1") }

	switch decideHydrate(raw, seededBefore) {
	case hydrateImport:
		hadLocalDataset = true
		if err := app.ImportJSON([]byte(raw)); err != nil {
			app.Log().Error("dataset hydrate failed; seeding sample", "err", err)
			hadLocalDataset = false
			_ = app.LoadSample()
			// Fallback to sample — show the banner so the user knows (L6).
			uistate.SetSampleActive(true)
		} else {
			// A real saved dataset means the user has personalised — clear any
			// stale sample-active flag left from a prior first run.
			uistate.SetSampleActive(false)
		}
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
				js.Global().Get("localStorage").Call("setItem", datasetStoreKey, string(env))
				last = target
				afterWrite()
			})
			return
		}
		last = s
		js.Global().Get("localStorage").Call("setItem", datasetStoreKey, s)
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
