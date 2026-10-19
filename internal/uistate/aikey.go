// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/browserstore"

// aiKeyStore is the legacy browser-store entry used before device credentials.
// LoadAIKey/ClearAIKey remain for one-time migration to cashflux:cred:openai.
const aiKeyStore = "cashflux:openai-key"

// ClearAIKey removes the legacy standalone OpenAI-key entry after migration.
func ClearAIKey() { browserstore.Remove(aiKeyStore) }

// LoadAIKey reads the legacy standalone OpenAI key, or "" if none is stored.
func LoadAIKey() string { return browserstore.GetString(aiKeyStore) }
