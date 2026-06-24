// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/browserstore"

// aiKeyStore is the browser-store entry (IndexedDB) holding the OpenAI key when the
// user has opted into remembering it on this device (prefs.RememberAIKey). It is
// kept OUT of the autosaved dataset (which always redacts the key), so the secret
// never rides along in an export/sync — but it is still in IndexedDB, not
// localStorage, so the app depends on no localStorage.
const aiKeyStore = "cashflux:openai-key"

// PersistAIKey stores the OpenAI key (only call this when the user has opted in via
// the "remember on this device" toggle).
func PersistAIKey(key string) { browserstore.Set(aiKeyStore, key) }

// ClearAIKey removes any persisted OpenAI key (remember-key toggle off).
func ClearAIKey() { browserstore.Remove(aiKeyStore) }

// LoadAIKey reads the persisted OpenAI key, or "" if none is stored.
func LoadAIKey() string { return browserstore.GetString(aiKeyStore) }
