// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/browserstore"

// aiKeyStore is the LEGACY browser-store entry (IndexedDB) that used to hold the
// OpenAI key separately from the dataset. The key now lives in Settings.OpenAIKey
// inside the SQLite dataset — the single source of truth — so nothing writes here
// anymore. LoadAIKey/ClearAIKey remain solely for the one-time boot/unlock migration
// (see app.migrateStandaloneAIKey) that folds any pre-existing standalone key into
// the dataset and then deletes this entry.
const aiKeyStore = "cashflux:openai-key"

// ClearAIKey removes the legacy standalone OpenAI-key entry. Called by the migration
// once the key has been adopted into the dataset.
func ClearAIKey() { browserstore.Remove(aiKeyStore) }

// LoadAIKey reads the legacy standalone OpenAI key, or "" if none is stored. Used
// only by the migration to detect a pre-existing standalone key to adopt.
func LoadAIKey() string { return browserstore.GetString(aiKeyStore) }
