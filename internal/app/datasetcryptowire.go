// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — boot/persist wiring for dataset-at-rest encryption (C45).
//
// Design (safety first — the app must never lock a user out of their own data):
//
//   - Encryption is opt-in and rides the existing passcode lock. The dataset is
//     encrypted at rest ONLY while the lock is active (enabled and not suspended)
//     AND the session passcode is known. With no passcode the dataset stays
//     plaintext, exactly as before — IsEnvelope is an O(4) marker check, so every
//     existing (plaintext, "{"-prefixed) dataset flows through untouched.
//   - The derived AES key never leaves the JS runtime; only salt+IV+ciphertext are
//     persisted (see internal/cryptobox + datasetcrypto.go).
//   - On boot, an encrypted dataset is detected and its hydration is DEFERRED until
//     the passcode gate is satisfied. The autosave is held off while a decrypt is
//     pending so it can never overwrite the ciphertext with an empty plaintext
//     dataset.
//   - Decryption failure is non-fatal: the ciphertext is kept, an error is logged,
//     and the only data-destroying recovery remains the gate's explicit
//     "Forgot passcode → wipe" path.
package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// activePasscode holds the session passcode once the user has unlocked (or just
// set) the lock, so the autosave can derive the encryption key. It lives only in
// wasm memory — the same exposure as the in-memory dataset itself — and is never
// persisted. Cleared when the lock is removed.
var activePasscode string

// pendingEnvelopeRaw holds a not-yet-decrypted dataset envelope read at boot. While
// it is non-empty the dataset is locked: the store is empty and the autosave is
// suppressed so it can't clobber the ciphertext. It is cleared once the envelope is
// decrypted and imported (or the user wipes).
var pendingEnvelopeRaw string

// encSaving guards against overlapping async encrypt-and-write operations: a new
// autosave tick is skipped while a prior encryption is still in flight.
var encSaving bool

// resaveDataset, set by startDatasetAutosave, forces an immediate dataset rewrite
// regardless of whether the bytes changed. It is nil until the autosave starts.
var resaveDataset func()

// migrateDatasetAtRest triggers an immediate re-save so the at-rest copy switches
// between plaintext and envelope the moment the passcode (encryption mode) changes.
func migrateDatasetAtRest() {
	if resaveDataset != nil {
		resaveDataset()
	}
}

// datasetEncryptionActive reports whether the dataset should be encrypted at rest:
// the lock is active (enabled and not suspended) and the session passcode is known.
// When false the dataset is written as plaintext (the historical behaviour), which
// also performs the reverse migration after the lock is removed.
func datasetEncryptionActive() bool {
	return loadAppLock().Active() && activePasscode != ""
}

// onAppUnlocked records the verified passcode for the session and, when a dataset
// envelope is waiting, decrypts and hydrates it. Called from the passcode gate the
// moment a correct passcode is entered.
func onAppUnlocked(passcode string) {
	activePasscode = passcode
	if pendingEnvelopeRaw != "" {
		hydrateFromPasscode(passcode)
	}
}

// hydrateFromPasscode parses the pending envelope, decrypts it with the session
// passcode, and imports the recovered dataset. On success it clears the pending
// state (releasing the autosave) and re-captures the undo baseline. On failure it
// keeps the ciphertext intact and logs — it never wipes.
func hydrateFromPasscode(passcode string) {
	app := appstate.Default
	if app == nil {
		return
	}
	env, ok := cryptobox.Parse([]byte(pendingEnvelopeRaw))
	if !ok {
		app.Log().Error("encrypted dataset: envelope unreadable; keeping ciphertext")
		return
	}
	decryptDataset(env, passcode, func(plain []byte, err error) {
		if err != nil {
			app.Log().Error("encrypted dataset: decrypt failed; keeping ciphertext", "err", err)
			return
		}
		if err := app.ImportJSON(plain); err != nil {
			app.Log().Error("encrypted dataset: import after decrypt failed; keeping ciphertext", "err", err)
			return
		}
		pendingEnvelopeRaw = "" // releases the autosave and unblocks normal operation
		// Re-seed the PREFS ATOM from the just-imported settings KV. The lock screen
		// rendered before this import, so the first UsePrefs seeded the atom from an
		// EMPTY store — prefs.Default(). Without this, every passcode boot shows
		// default preferences (the budgets income basis reset to "all income", Cam
		// 2026-07-17) even though the saved values sit right here in the store — and
		// the next PersistPrefs (any theme toggle) would write those defaults back
		// over the real ones. SetPrefs also re-applies theme/accent to the document.
		uistate.SetPrefs(uistate.LoadPrefs())
		uistate.ApplyTheme(uistate.LoadTheme())
		// The dataset is loaded now, so it's safe to fold any legacy standalone OpenAI
		// key + language selection into it (encrypted users couldn't migrate at boot
		// while still locked).
		migrateStandaloneAIKey()
		uistate.MigrateLegacyLanguage()
		seedMusicFromDataset()
		initUndo()
		uistate.BumpDataRevision()
		// The OpenAI key is now decrypted and available — refresh the lock-screen
		// quote-of-the-day cache so the next lock shows a fresh AI quote (C: lock quote).
		refreshDailyLockQuote()
	})
}
