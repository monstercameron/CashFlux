// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
)

const cloudSetupKey = "cashflux:cloud:setup"

func cloudSetupPending() bool { return lsGet(cloudSetupKey) != "" }

// cloudCommitPreferences completes only after the v1 preferences-bearing local
// snapshot commits. PersistPrefs alone has no completion/error API. Cloud uses
// this narrow command for enabling/disabling sync, never as a new autosave path.
// The existing generation entitlement and autosave guards apply throughout.
func cloudCommitPreferences(atom state.Atom[prefs.Prefs], next prefs.Prefs, done func(error)) {
	a := appstate.Default
	if a == nil || suspendAutosave || encSaving || autosaveBlocked() || hostedHydrationRequired() || pendingEnvelopeRaw != "" || readDatasetGen() != datasetMyGen {
		done(errors.New("local data is not ready for a durable settings change"))
		return
	}
	raw, err := json.Marshal(next.Normalize())
	if err != nil {
		done(fmt.Errorf("encode cloud preferences: %w", err))
		return
	}
	suspendAutosave = true
	generation := datasetMyGen
	data, err := cloudPreferenceSnapshot(a, string(raw))
	if err != nil {
		suspendAutosave = false
		done(err)
		return
	}
	finish := func(err error) {
		if err != nil {
			suspendAutosave = false
			done(err)
			return
		}
		atom.Set(next.Normalize())
		uistate.PersistPrefs(next)
		suspendAutosave = false
		done(nil)
	}
	write := func(data []byte) {
		if readDatasetGen() != generation || datasetMyGen != generation {
			finish(errors.New("another tab changed the dataset"))
			return
		}
		cloudWritePreferenceSnapshot(a, string(raw), data, func(data []byte, done func(error)) {
			values, expected := prepareWorkspaceCommit(map[string]string{datasetStoreKey: string(data)}, generation)
			browserstore.SetSecureBatch(values, expected, func(err error) {
				if err == nil {
					datasetMyGen = values[datasetGenKey]
				}
				done(err)
			})
		}, finish)
	}
	if datasetEncryptionActive() {
		passcode, lock := activePasscode, lsGet(appLockKey)
		encryptDataset(data, passcode, func(envelope []byte, err error) {
			if err != nil {
				finish(fmt.Errorf("encrypt cloud settings: %w", err))
				return
			}
			if !datasetEncryptionActive() || activePasscode != passcode || lsGet(appLockKey) != lock {
				finish(errors.New("data protection changed while saving cloud preferences"))
				return
			}
			// The lock config is outside the dataset. Commit it before enabling
			// encrypted sync so a reload can always unlock the saved envelope.
			browserstore.SetDurable(appLockKey, lock, func(err error) {
				if err != nil {
					finish(err)
					return
				}
				write(envelope)
			})
		})
		return
	}
	write(data)
}
