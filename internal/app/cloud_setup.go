// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/state"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

type cloudSetupController struct {
	Step                                             cloudStep
	Saved, Error                                     string
	Busy                                             bool
	Start, Continue, Enable, SyncNow, Protect, Pause func()
	PauseThen                                        func(func(error))
}

func useCloudSetup(atom state.Atom[prefs.Prefs], status syncStatus, signedIn, protected bool, bump func()) cloudSetupController {
	saved := uic.UseState(lsGet(cloudSetupKey))
	busy := uic.UseState(false)
	errText := uic.UseState("")
	failed := func(err error) {
		busy.Set(false)
		if err != nil {
			logSyncError("cloud settings not saved", err)
			errText.Set(uistate.T("cloud1.saveFailed"))
		}
	}
	setStep := func(s cloudStep, after func()) {
		busy.Set(true)
		errText.Set("")
		browserstore.SetDurable(cloudSetupKey, string(s), func(err error) {
			failed(err)
			if err == nil {
				saved.Set(string(s))
				if after != nil {
					after()
				}
			}
		})
	}
	pauseThen := func(done func(error)) {
		if busy.Get() {
			done(errors.New("cloud settings change already in progress"))
			return
		}
		busy.Set(true)
		errText.Set("")
		p := atom.Get()
		p.BackendDisabled = true
		cloudCommitPreferences(atom, p, func(err error) {
			if err != nil {
				failed(err)
				done(err)
				return
			}
			restartBackendSync()
			browserstore.SetDurable(cloudSetupKey, "", func(err error) {
				failed(err)
				if err == nil {
					saved.Set("")
				}
				done(err)
			})
		})
	}
	protect := func() {
		if datasetEncryptionActive() {
			bump()
			return
		}
		c := loadAppLock()
		if c.Enabled && c.Suspended {
			setLockSuspended(false)
			showAppLockGate()
			bump()
			return
		}
		if c.Active() && activePasscode == "" {
			showAppLockGate()
			return
		}
		showAppLockSetup(bump)
	}
	start := func() {
		if busy.Get() {
			return
		}
		if atom.Get().BackendDisabled {
			setStep(cloudWhere, nil)
			return
		}
		uistate.ConfirmModalLabeled(uistate.T("cloud1.changeConnectionConfirm"), uistate.T("cloud1.changeConnection"), false, func(ok bool) {
			if ok {
				pauseThen(func(err error) {
					if err == nil {
						setStep(cloudWhere, nil)
					}
				})
			}
		})
	}
	enable := func() {
		p := atom.Get()
		if busy.Get() || !datasetEncryptionActive() || !uistate.Session(p.ServerToken).Present() {
			return
		}
		setStep(cloudFirst, func() {
			busy.Set(true)
			p = atom.Get()
			p.BackendDisabled = false
			cloudCommitPreferences(atom, p, func(err error) {
				if err != nil {
					failed(err)
					return
				}
				st := loadSyncStatus()
				st.State = "syncing"
				setSyncStatus(st)
				failed(nil)
				restartBackendSync()
				requestBackendSyncNow()
			})
		})
	}
	// A click or a successful preferences save is not a completed first sync.
	// Clear the resumable marker only when the engine actually reports completion.
	uic.UseEffect(func() func() {
		current := loadSyncStatus()
		if cloudSetupFinished(saved.Get(), atom.Get().BackendDisabled, busy.Get(), current.State, current.Pending) {
			browserstore.SetDurable(cloudSetupKey, "", func(err error) {
				if err != nil {
					failed(err)
					return
				}
				saved.Set("")
			})
		}
		return nil
	}, status.State, status.Pending, saved.Get(), busy.Get())
	facts := cloudFacts{Disabled: atom.Get().BackendDisabled, SignedIn: signedIn, Protected: protected, Setup: saved.Get(), State: status.State}
	return cloudSetupController{Step: cloudNextStep(facts), Saved: saved.Get(), Busy: busy.Get(), Error: errText.Get(), Start: start,
		Continue: func() {
			if !busy.Get() {
				setStep(cloudSignIn, nil)
			}
		}, Enable: enable, Protect: protect,
		SyncNow: func() {
			p := atom.Get()
			if !p.BackendDisabled && datasetEncryptionActive() && uistate.Session(p.ServerToken).Present() {
				requestBackendSyncNow()
			}
		},
		Pause: func() {
			uistate.ConfirmModalLabeled(uistate.T("cloud1.pauseConfirm", cloudHouseholdName()), uistate.T("cloud1.pause"), true, func(ok bool) {
				if ok {
					pauseThen(func(error) {})
				}
			})
		}, PauseThen: pauseThen,
	}
}
