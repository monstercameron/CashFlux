// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — encrypted institution-credential vault.
//
// ┌───────────────────────────────────────────────────────────────────────────┐
// │ ⚠️  SECURITY: NOT YET INDEPENDENTLY SECURITY-REVIEWED — FIRST PASS.          │
// │                                                                             │
// │ TODO(security-review): a full review is REQUIRED before this is trusted for │
// │ real bank logins. This is deliberately conservative but has known gaps the  │
// │ review must close:                                                          │
// │   • XSS on this origin could read the derived key / plaintext while the app │
// │     is unlocked (same exposure as the in-memory dataset). Consider a worker │
// │     + stricter CSP + short-lived reveal.                                    │
// │   • The app passcode gates everything; a weak passcode ⇒ weak encryption.   │
// │     Consider enforcing passcode strength / WebAuthn-PRF-derived keys.       │
// │   • No hardware-backed key / secure enclave.                                │
// │   • Changing the passcode orphans the vault (it becomes undecryptable) —    │
// │     needs a re-encrypt-on-passcode-change hook.                             │
// │   • Retrieval is copy-to-clipboard behind a passcode re-auth and the        │
// │     password is never put in the DOM / shown — but the clipboard still      │
// │     holds it afterwards (readable by other apps / clipboard managers);      │
// │     consider an auto-clear-after-N-seconds.                                  │
// └───────────────────────────────────────────────────────────────────────────┘
//
// Design (the safe parts, so the review starts from a sane baseline):
//   - Stored in a DEDICATED browserstore key, NEVER inside the dataset blob — so
//     credentials are never exported (ExportJSON/Redacted), never synced to the
//     backend, and never appear in a CSV/JSON backup. Local to this device only.
//   - Encrypted at rest with the same AES-GCM-256 + PBKDF2 (600k iters) stack the
//     dataset uses, keyed by the session passcode (encryptDataset/decryptDataset).
//     The ciphertext is meaningless without the passcode.
//   - Only available while the app is unlocked with a passcode set
//     (datasetEncryptionActive). No passcode ⇒ no credential storage.
//   - Never logged.
package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

// credVaultStoreKey is a dedicated, LOCAL-ONLY browserstore key. It is intentionally
// NOT part of the dataset blob (cashflux:dataset) — so the vault is never exported,
// never synced, and never included in a backup.
const credVaultStoreKey = "cashflux:credvault"

// errVaultLocked is returned when the vault is accessed without an active passcode.
var errVaultLocked = errors.New("credential vault requires an app passcode + unlock")

// Credential is one account's institution login. Every field is optional. It is only
// ever held in memory decrypted while the modal is open, and encrypted at rest.
type Credential struct {
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	LoginURL  string `json:"loginUrl,omitempty"`
	Notes     string `json:"notes,omitempty"` // e.g. security-question answers, phone PIN
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// IsEmpty reports whether the credential carries no data (so it can be dropped).
func (c Credential) IsEmpty() bool {
	return c.Username == "" && c.Password == "" && c.LoginURL == "" && c.Notes == ""
}

// credVault maps an account id → its credential.
type credVault map[string]Credential

// credVaultAvailable reports whether the vault can be used right now (a passcode is
// set and the session is unlocked). The UI gates on this.
func credVaultAvailable() bool { return datasetEncryptionActive() }

// loadCredVault decrypts the whole vault. onDone receives an empty (non-nil) map when
// no vault exists yet, or an error when locked / corrupt. Async (Web Crypto).
func loadCredVault(onDone func(v credVault, err error)) {
	if !credVaultAvailable() {
		onDone(nil, errVaultLocked)
		return
	}
	raw := browserstore.GetString(credVaultStoreKey)
	if raw == "" {
		onDone(credVault{}, nil)
		return
	}
	envBytes, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		onDone(nil, err)
		return
	}
	env, ok := cryptobox.Parse(envBytes)
	if !ok {
		onDone(nil, errors.New("credential vault: unreadable envelope"))
		return
	}
	decryptDataset(env, activePasscode, func(plain []byte, err error) {
		if err != nil {
			onDone(nil, err)
			return
		}
		v := credVault{}
		if len(plain) > 0 {
			if e := json.Unmarshal(plain, &v); e != nil {
				onDone(nil, e)
				return
			}
		}
		if v == nil {
			v = credVault{}
		}
		onDone(v, nil)
	})
}

// saveCredVault encrypts + persists the vault. Async (Web Crypto).
func saveCredVault(v credVault, onDone func(err error)) {
	if !credVaultAvailable() {
		onDone(errVaultLocked)
		return
	}
	// Drop empty entries so removing a credential shrinks the vault.
	for id, c := range v {
		if c.IsEmpty() {
			delete(v, id)
		}
	}
	plain, err := json.Marshal(v)
	if err != nil {
		onDone(err)
		return
	}
	encryptDataset(plain, activePasscode, func(env []byte, err error) {
		if err != nil {
			onDone(err)
			return
		}
		browserstore.SetThen(credVaultStoreKey, base64.StdEncoding.EncodeToString(env), func() { onDone(nil) })
	})
}
