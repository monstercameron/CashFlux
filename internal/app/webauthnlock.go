// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — WebAuthn PRF passkey registration and unlock (C282).
//
// Design: the PRF-derived key wraps the session passcode, not the dataset
// directly. On unlock, the recovered passcode is handed to onAppUnlocked()
// — the exact same call the passcode-input gate makes — so the dataset
// decryption path (datasetcrypto.go) is never touched.
//
// Three localStorage keys hold the passkey state:
//   - cashflux:webauthn-credid  (base64 credential ID)
//   - cashflux:webauthn-salt    (base64 32-byte PRF salt, constant per credential)
//   - cashflux:webauthn-vault   (JSON-encoded prfVault: IV + AES-GCM ciphertext of passcode)
//
// All three are preserved across financial data wipes (persist.go keptOnWipeKeys).
// clearPasskey() removes all three and is called whenever the passcode changes or
// the lock is disabled, so a stale vault can never permanently block access.
package app

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/webauthn"
)

// localStorage key names for the WebAuthn PRF passkey state.
const (
	webauthnCredKey  = "cashflux:webauthn-credid"
	webauthnSaltKey  = "cashflux:webauthn-salt"
	webauthnVaultKey = "cashflux:webauthn-vault"
	webauthnModalID  = "cf-webauthn-modal"
)

// webauthnRPID returns the relying-party ID for WebAuthn ceremonies. Uses the
// current page hostname, falling back to "localhost" for local dev.
func webauthnRPID() string {
	hostname := js.Global().Get("location").Get("hostname").String()
	if hostname == "" {
		return "localhost"
	}
	return hostname
}

// hasPasskey reports whether all three passkey state keys are present in
// localStorage. True iff a credential was successfully registered.
func hasPasskey() bool {
	return lsGet(webauthnCredKey) != "" &&
		lsGet(webauthnSaltKey) != "" &&
		lsGet(webauthnVaultKey) != ""
}

// clearPasskey removes the credential ID, PRF salt, and vault from localStorage,
// effectively deregistering the passkey. Must be called on passcode change and
// on lock disable so a stale vault can never permanently block access.
func clearPasskey() {
	lsRemove(webauthnCredKey)
	lsRemove(webauthnSaltKey)
	lsRemove(webauthnVaultKey)
}

// registerPasskey runs the full registration ceremony:
//  1. Generates a stable 32-byte PRF salt and a fresh user ID.
//  2. Calls webauthn.Register() to create the credential.
//  3. Calls webauthn.GetPRF() with the new credential and salt to obtain the
//     32-byte PRF output.
//  4. Calls encryptVaultWithPRF() to wrap the active session passcode.
//  5. Persists credential ID (base64), salt (base64), and vault (JSON) to
//     localStorage.
//
// A non-nil error means any step failed; the passcode gate is unaffected.
// registerPasskey must only be called when the lock is enabled (activePasscode
// non-empty), i.e. from the passkey manager opened from Settings → Security.
func registerPasskey(onDone func(err error)) {
	if activePasscode == "" {
		onDone(errors.New("webauthn register: no active passcode — enable app lock first"))
		return
	}

	// Generate 32-byte PRF salt (constant per credential; stored in localStorage).
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		onDone(fmt.Errorf("webauthn register: salt generation: %w", err))
		return
	}

	// Stable per-device user ID (opaque to the RP; not a username).
	userIDBuf := make([]byte, 16)
	if _, err := rand.Read(userIDBuf); err != nil {
		onDone(fmt.Errorf("webauthn register: userID generation: %w", err))
		return
	}
	userID := base64.URLEncoding.EncodeToString(userIDBuf)

	rpID := webauthnRPID()
	saltB64 := base64.StdEncoding.EncodeToString(salt)

	webauthn.Register(rpID, userID, "CashFlux User", func(credentialID []byte, err error) {
		if err != nil {
			onDone(fmt.Errorf("webauthn register: ceremony: %w", err))
			return
		}

		// Immediately exercise GetPRF to obtain the key material for wrapping.
		webauthn.GetPRF(rpID, credentialID, salt, func(prf32 []byte, err error) {
			if err != nil {
				onDone(fmt.Errorf("webauthn register: GetPRF: %w", err))
				return
			}

			// Capture the passcode at the time of registration (local var so
			// the closure doesn't rely on activePasscode still matching later).
			passcodeSnapshot := activePasscode

			encryptVaultWithPRF(passcodeSnapshot, prf32, func(vaultJSON []byte, err error) {
				if err != nil {
					onDone(fmt.Errorf("webauthn register: vault encrypt: %w", err))
					return
				}
				// Persist all three keys atomically (best-effort; localStorage
				// is synchronous, so all three either succeed or the user will
				// see hasPasskey()==false and can retry).
				lsSet(webauthnCredKey, base64.StdEncoding.EncodeToString(credentialID))
				lsSet(webauthnSaltKey, saltB64)
				lsSet(webauthnVaultKey, string(vaultJSON))
				onDone(nil)
			})
		})
	})
}

// unlockWithPasskey runs the authentication ceremony to recover the passcode
// from the PRF vault, then calls onAppUnlocked and dismisses the lock gate.
// A non-nil error means any step failed — the caller must keep the passcode
// gate visible and functional. Never panics; never wipes data.
func unlockWithPasskey(doc, gate js.Value, onDone func(err error)) {
	credB64 := lsGet(webauthnCredKey)
	saltB64 := lsGet(webauthnSaltKey)
	vaultRaw := lsGet(webauthnVaultKey)

	if credB64 == "" || saltB64 == "" || vaultRaw == "" {
		onDone(errors.New("webauthn unlock: passkey not registered"))
		return
	}

	credentialID, err := base64.StdEncoding.DecodeString(credB64)
	if err != nil {
		onDone(fmt.Errorf("webauthn unlock: bad credentialID base64: %w", err))
		return
	}
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		onDone(fmt.Errorf("webauthn unlock: bad salt base64: %w", err))
		return
	}

	rpID := webauthnRPID()

	webauthn.GetPRF(rpID, credentialID, salt, func(prf32 []byte, err error) {
		if err != nil {
			onDone(fmt.Errorf("webauthn unlock: GetPRF: %w", err))
			return
		}

		decryptVaultWithPRF([]byte(vaultRaw), prf32, func(passcode string, err error) {
			if err != nil {
				onDone(fmt.Errorf("webauthn unlock: vault decrypt: %w", err))
				return
			}

			// Verify the recovered passcode matches the stored hash. If the
			// passcode changed since the vault was sealed, the vault is stale —
			// clear it and fall back to the passcode input gate (no lockout).
			if !loadAppLock().Verify(passcode) {
				clearPasskey()
				onDone(errors.New("webauthn unlock: vault passcode mismatch — passkey cleared; use passcode to unlock"))
				return
			}

			// Passcode verified: unlock the app exactly as the passcode gate does.
			onAppUnlocked(passcode)
			unlockGate(doc, gate)
			onDone(nil)
		})
	})
}

// showPasskeyManager checks WebAuthn/PRF availability asynchronously, then
// builds or re-shows the passkey management modal. Called from the Settings
// Security section "Manage passkey" button. If the modal already exists it is
// re-shown with refreshed state.
func showPasskeyManager() {
	webauthn.Available(func(available bool) {
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return
		}
		// Re-use existing modal if already built.
		if m := doc.Call("getElementById", webauthnModalID); !m.IsNull() && !m.IsUndefined() {
			m.Get("style").Set("display", "grid")
			refreshPasskeyMgrState(doc, available)
			return
		}
		buildPasskeyManager(doc, available)
	})
}

// buildPasskeyManager creates the passkey management modal and appends it to
// <body>. available indicates whether the browser supports WebAuthn+PRF.
func buildPasskeyManager(doc js.Value, available bool) {
	ov := doc.Call("createElement", "div")
	ov.Set("id", webauthnModalID)
	ov.Get("style").Set("cssText", "position:fixed;inset:0;z-index:1001;display:grid;place-items:center;background:rgba(0,0,0,0.6);")
	ov.Call("setAttribute", "role", "dialog")
	ov.Call("setAttribute", "aria-modal", "true")
	ov.Call("setAttribute", "aria-label", uistate.T("webauthn.setupTitle"))

	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "display:flex;flex-direction:column;gap:0.7rem;width:min(90vw,340px);padding:1.2rem;background:var(--bg-elev,#1a1a1d);color:var(--text,#f4f4f5);border:1px solid var(--border,#2a2a2c);border-radius:10px;box-shadow:0 12px 40px rgba(0,0,0,0.5);")
	card.Set("innerHTML",
		`<div style="font-size:1.05rem;font-weight:600;">`+escT("webauthn.setupTitle")+`</div>`+
			`<div style="font-size:0.85rem;opacity:0.75;">`+escT("webauthn.setupDesc")+`</div>`+
			`<div id="cf-wa-status" aria-live="polite" role="status" style="font-size:0.82rem;min-height:1.2em;"></div>`+
			`<div id="cf-wa-actions" style="display:flex;gap:0.5rem;flex-direction:column;"></div>`+
			`<div style="display:flex;justify-content:flex-end;margin-top:0.3rem;">`+
			`<button id="cf-wa-close" type="button" style="padding:0.5rem 0.9rem;border-radius:8px;border:1px solid var(--border,#2a2a2c);background:transparent;color:inherit;cursor:pointer;">`+
			escT("action.cancel")+`</button></div>`)

	ov.Call("appendChild", card)
	doc.Get("body").Call("appendChild", ov)

	closeCb := js.FuncOf(func(js.Value, []js.Value) any {
		ov.Get("style").Set("display", "none")
		return nil
	})
	doc.Call("getElementById", "cf-wa-close").Call("addEventListener", "click", closeCb)

	refreshPasskeyMgrState(doc, available)
}

// refreshPasskeyMgrState updates the passkey manager modal action buttons based
// on the current hasPasskey() state and availability. Safe to call repeatedly.
func refreshPasskeyMgrState(doc js.Value, available bool) {
	// Clear the status message.
	if st := doc.Call("getElementById", "cf-wa-status"); !st.IsNull() && !st.IsUndefined() {
		st.Set("textContent", "")
		st.Get("style").Set("color", "")
	}
	registered := hasPasskey()
	wirePasskeyMgrButtons(doc, registered, available)
}

// setPasskeyMgrStatus sets the text and color of the status message element
// inside the passkey manager modal. isErr=true uses the danger color.
func setPasskeyMgrStatus(doc js.Value, msg string, isErr bool) {
	el := doc.Call("getElementById", "cf-wa-status")
	if el.IsNull() || el.IsUndefined() {
		return
	}
	el.Set("textContent", msg)
	if isErr {
		el.Get("style").Set("color", "var(--danger,#d8716f)")
	} else {
		el.Get("style").Set("color", "var(--ok-text,#52c278)")
	}
}

// wirePasskeyMgrButtons replaces the action-button area of the passkey manager
// with the appropriate button(s) for the current registered/available state.
func wirePasskeyMgrButtons(doc js.Value, registered, available bool) {
	actionsEl := doc.Call("getElementById", "cf-wa-actions")
	if actionsEl.IsNull() || actionsEl.IsUndefined() {
		return
	}
	// Clear existing buttons.
	actionsEl.Set("innerHTML", "")

	if !available && !registered {
		// Device/browser doesn't support WebAuthn+PRF and no credential stored.
		notAvailEl := doc.Call("createElement", "div")
		notAvailEl.Get("style").Set("cssText", "font-size:0.85rem;opacity:0.7;font-style:italic;")
		notAvailEl.Set("textContent", uistate.T("webauthn.notAvailable"))
		actionsEl.Call("appendChild", notAvailEl)
		return
	}

	if registered {
		// Show "Remove passkey" danger button.
		removeBtn := doc.Call("createElement", "button")
		removeBtn.Set("type", "button")
		removeBtn.Set("textContent", uistate.T("webauthn.removeBtn"))
		removeBtn.Get("style").Set("cssText", "padding:0.6rem 0.8rem;min-height:44px;border-radius:8px;border:1px solid var(--danger,#d8716f);background:transparent;color:var(--danger,#d8716f);cursor:pointer;font-size:0.9rem;font-weight:600;")
		actionsEl.Call("appendChild", removeBtn)
		removeCb := js.FuncOf(func(js.Value, []js.Value) any {
			if !js.Global().Call("confirm", uistate.T("webauthn.removeConfirm")).Bool() {
				return nil
			}
			clearPasskey()
			setPasskeyMgrStatus(doc, uistate.T("webauthn.removedOK"), false)
			wirePasskeyMgrButtons(doc, false, available)
			return nil
		})
		removeBtn.Call("addEventListener", "click", removeCb)
		return
	}

	// Show "Register passkey" button when available but not yet registered.
	registerBtn := doc.Call("createElement", "button")
	registerBtn.Set("type", "button")
	registerBtn.Set("textContent", uistate.T("webauthn.registerBtn"))
	registerBtn.Get("style").Set("cssText", "padding:0.6rem 0.8rem;min-height:44px;border-radius:8px;border:0;background:var(--accent,#2e8b57);color:#052e13;font-weight:600;cursor:pointer;font-size:0.9rem;")
	actionsEl.Call("appendChild", registerBtn)

	registerCb := js.FuncOf(func(js.Value, []js.Value) any {
		registerBtn.Set("disabled", true)
		registerPasskey(func(err error) {
			registerBtn.Set("disabled", false)
			if err != nil {
				setPasskeyMgrStatus(doc, uistate.T("webauthn.setupFail"), true)
				return
			}
			setPasskeyMgrStatus(doc, uistate.T("webauthn.setupOK"), false)
			wirePasskeyMgrButtons(doc, true, available)
		})
		return nil
	})
	registerBtn.Call("addEventListener", "click", registerCb)
}
