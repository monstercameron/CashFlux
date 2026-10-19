// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicred"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

const aiCredentialStoreKey = "cashflux:cred:openai"

var aiCredentialSaveTimer *time.Timer
var aiCredentialWriteRevision uint64

func queueAICredentialSave(key string, remember bool, done func(error)) {
	if aiCredentialSaveTimer != nil {
		aiCredentialSaveTimer.Stop()
	}
	aiCredentialSaveTimer = time.AfterFunc(350*time.Millisecond, func() {
		saveAICredential(key, remember, func(err error) {
			if err != nil && appstate.Default != nil {
				appstate.Default.Log().Error("save OpenAI device credential failed", "err", err)
			}
			if done != nil {
				done(err)
			}
		})
	})
}

func cancelQueuedAICredentialSave() {
	if aiCredentialSaveTimer != nil {
		aiCredentialSaveTimer.Stop()
		aiCredentialSaveTimer = nil
	}
}

// saveAICredential stores the BYO key outside all dataset-scoped state. A
// remembered key is written only after strict IndexedDB durability succeeds.
func saveAICredential(key string, remember bool, done func(error)) {
	aiCredentialWriteRevision++
	revision := aiCredentialWriteRevision
	key = strings.TrimSpace(key)
	if !aicred.ShouldPersist(remember, key) {
		browserstore.RemoveDurable(aiCredentialStoreKey, done)
		return
	}
	store := func(plainKey, envelope string) {
		if revision != aiCredentialWriteRevision {
			return
		}
		record, err := aicred.New(plainKey, envelope, true)
		if err != nil {
			if done != nil {
				done(err)
			}
			return
		}
		data, err := aicred.Marshal(record)
		if err != nil {
			if done != nil {
				done(err)
			}
			return
		}
		browserstore.SetDurable(aiCredentialStoreKey, string(data), done)
	}
	if datasetEncryptionActive() {
		encryptDataset([]byte(key), activePasscode, func(envelope []byte, err error) {
			if revision != aiCredentialWriteRevision {
				return
			}
			if err != nil {
				if done != nil {
					done(err)
				}
				return
			}
			if _, ok := cryptobox.Parse(envelope); !ok {
				if done != nil {
					done(errSyncEncryptEmpty)
				}
				return
			}
			store("", string(envelope))
		})
		return
	}
	store(key, "")
}

// hydrateAIKey loads the device credential after dataset hydration and migrates
// credentials written by older releases. A locked encrypted credential waits
// for onAppUnlocked, which calls this function again after setting passcode.
func hydrateDeviceAIKey() {
	app := appstate.Default
	if app == nil {
		return
	}
	data := strings.TrimSpace(browserstore.GetString(aiCredentialStoreKey))
	if data != "" {
		record, err := aicred.Parse([]byte(data))
		if err != nil {
			app.Log().Error("stored OpenAI credential is unreadable", "err", err)
			return
		}
		setRememberAIKey(record.Remember)
		if record.Envelope != "" {
			if activePasscode == "" {
				return
			}
			parsed, ok := cryptobox.Parse([]byte(record.Envelope))
			if !ok {
				app.Log().Error("stored OpenAI credential envelope is unreadable")
				return
			}
			decryptDataset(parsed, activePasscode, func(plain []byte, err error) {
				if err != nil {
					app.Log().Error("stored OpenAI credential could not be decrypted", "err", err)
					return
				}
				applyDeviceAIKey(string(plain))
			})
			return
		}
		applyDeviceAIKey(record.Key)
		return
	}
	// Dataset keys predate device credentials. Move the value out before the
	// next dataset write; the durable credential remains the recovery copy.
	settings := app.Settings()
	legacy := uistate.LoadAIKey()
	key := aicred.MigrationKey(settings.OpenAIKey, legacy, false)
	if key == "" {
		return
	}
	remember := strings.TrimSpace(settings.OpenAIKey) != "" || strings.TrimSpace(legacy) != ""
	saveAICredential(key, remember, func(err error) {
		if err != nil {
			app.Log().Error("migrate OpenAI credential failed", "err", err)
			return
		}
		applyDeviceAIKey(key)
		uistate.ClearAIKey()
	})
}

func applyDeviceAIKey(key string) {
	app := appstate.Default
	if app == nil {
		return
	}
	settings := app.Settings()
	settings.OpenAIKey = strings.TrimSpace(key)
	if err := app.PutSettings(settings); err != nil {
		app.Log().Error("load OpenAI credential into memory failed", "err", err)
	}
}

func setRememberAIKey(remember bool) {
	pr := uistate.LoadPrefs().Normalize()
	if pr.RememberAIKey == remember {
		return
	}
	pr.RememberAIKey = remember
	uistate.PersistPrefs(pr)
	// Ensure the next prefs atom created by Settings reads the device-global value.
	uistate.SetPrefs(pr)
}

// persistRememberPreference keeps the UI's legacy preference field aligned with
// the device-global credential record while preserving every other preference.
func persistRememberPreference(remember bool) {
	pr := uistate.LoadPrefs().Normalize()
	pr.RememberAIKey = remember
	uistate.PersistPrefs(pr)
	uistate.SetPrefs(pr)
}
