// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — WebAuthn PRF vault encryption/decryption (C282).
//
// This file implements the AES-GCM-256 wrap/unwrap of the session passcode
// using the WebAuthn PRF output as key material. The vault approach is chosen
// deliberately to avoid modifying datasetcrypto.go at all:
//
//   - The PRF output (32 bytes from the authenticator) is imported directly as
//     a raw AES-256-GCM CryptoKey via crypto.subtle.importKey("raw", ...).
//   - The vault AES-GCM-encrypts the UTF-8 passcode string (not the dataset).
//   - On passkey unlock, decryptVaultWithPRF recovers the passcode string, which
//     is then passed to onAppUnlocked() — exactly as the passcode-input gate does.
//   - The PBKDF2 → dataset-key derivation in datasetcrypto.go is UNTOUCHED.
//
// Lockout-safety: a decryption failure (wrong credential, corrupted vault,
// stale passcode) returns a non-nil error. The caller (webauthnlock.go) keeps
// the passcode input gate visible and functional. No data is ever wiped.
package app

import (
	"errors"
	"fmt"
	"syscall/js"
)

// encryptVaultWithPRF AES-GCM-encrypts the UTF-8 passcode using prf32 as the
// raw AES-256 key material. Returns the vault JSON bytes (prfVault format)
// via onDone. prf32 must be exactly 32 bytes.
//
// The vault is subsequently stored in localStorage and is only useful with the
// same WebAuthn credential and salt that produced prf32. It is safe to store in
// the clear because it is meaningless without the authenticator.
func encryptVaultWithPRF(passcode string, prf32 []byte, onDone func(vault []byte, err error)) {
	if len(prf32) != 32 {
		onDone(nil, fmt.Errorf("prf vault: prf32 must be 32 bytes, got %d", len(prf32)))
		return
	}
	sub := subtle()
	if sub.IsNull() || sub.IsUndefined() {
		onDone(nil, errors.New("prf vault: crypto.subtle unavailable"))
		return
	}

	// Generate a fresh 12-byte IV (AES-GCM standard nonce size).
	iv := make([]byte, 12)
	if err := cryptoGetRandomValues(iv); err != nil {
		onDone(nil, fmt.Errorf("prf vault: iv generation: %w", err))
		return
	}

	// Import the raw PRF output as an AES-256-GCM CryptoKey.
	prf32Arr := js.Global().Get("Uint8Array").New(32)
	js.CopyBytesToJS(prf32Arr, prf32)

	var importRes, importRej js.Func
	importRes = js.FuncOf(func(_ js.Value, args []js.Value) any {
		importRes.Release()
		importRej.Release()
		if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
			onDone(nil, errors.New("prf vault: importKey returned no key"))
			return nil
		}
		aesKey := args[0]

		// Encrypt the passcode (UTF-8 bytes) under the imported AES key.
		passcodeBytes := []byte(passcode)
		plainArr := js.Global().Get("Uint8Array").New(len(passcodeBytes))
		js.CopyBytesToJS(plainArr, passcodeBytes)

		ivArr := js.Global().Get("Uint8Array").New(12)
		js.CopyBytesToJS(ivArr, iv)

		encParams := map[string]any{"name": "AES-GCM", "iv": ivArr}
		encPromise := sub.Call("encrypt", encParams, aesKey, plainArr)

		var encRes, encRej js.Func
		encRes = js.FuncOf(func(_ js.Value, eArgs []js.Value) any {
			encRes.Release()
			encRej.Release()
			if len(eArgs) == 0 || eArgs[0].IsNull() || eArgs[0].IsUndefined() {
				onDone(nil, errors.New("prf vault: encrypt promise resolved with no value"))
				return nil
			}
			// eArgs[0] is an ArrayBuffer; wrap as Uint8Array to copy bytes.
			cipherBuf := js.Global().Get("Uint8Array").New(eArgs[0])
			cipherBytes := make([]byte, cipherBuf.Get("length").Int())
			js.CopyBytesToGo(cipherBytes, cipherBuf)

			vaultJSON, err := marshalVault(iv, cipherBytes)
			if err != nil {
				onDone(nil, fmt.Errorf("prf vault: marshal: %w", err))
				return nil
			}
			onDone(vaultJSON, nil)
			return nil
		})
		encRej = js.FuncOf(func(_ js.Value, eArgs []js.Value) any {
			encRes.Release()
			encRej.Release()
			msg := "prf vault: encrypt rejected"
			if len(eArgs) > 0 {
				msg = fmt.Sprintf("%s: %s", msg, eArgs[0].String())
			}
			onDone(nil, errors.New(msg))
			return nil
		})
		encPromise.Call("then", encRes).Call("catch", encRej)
		return nil
	})
	importRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
		importRes.Release()
		importRej.Release()
		msg := "prf vault: importKey rejected"
		if len(rArgs) > 0 {
			msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
		}
		onDone(nil, errors.New(msg))
		return nil
	})

	importPromise := sub.Call("importKey",
		"raw",
		prf32Arr,
		map[string]any{"name": "AES-GCM", "length": 256},
		false, // not extractable
		[]any{"encrypt", "decrypt"},
	)
	importPromise.Call("then", importRes).Call("catch", importRej)
}

// decryptVaultWithPRF recovers the passcode string from vaultJSON using prf32
// as the AES-256 key material. Returns the plaintext passcode via onDone.
//
// A non-nil error means the vault could not be decrypted — the credential is
// wrong, the vault is corrupted, or the PRF was derived with a different salt.
// IMPORTANT: callers must treat this as a recoverable failure; NEVER wipe data.
// The caller (webauthnlock.go) must fall back to the passcode input gate.
func decryptVaultWithPRF(vaultJSON []byte, prf32 []byte, onDone func(passcode string, err error)) {
	if len(prf32) != 32 {
		onDone("", fmt.Errorf("prf vault: prf32 must be 32 bytes, got %d", len(prf32)))
		return
	}

	iv, cipherBytes, err := parseVault(vaultJSON)
	if err != nil {
		onDone("", fmt.Errorf("prf vault: parse: %w", err))
		return
	}

	sub := subtle()
	if sub.IsNull() || sub.IsUndefined() {
		onDone("", errors.New("prf vault: crypto.subtle unavailable"))
		return
	}

	// Import the raw PRF output as an AES-256-GCM CryptoKey.
	prf32Arr := js.Global().Get("Uint8Array").New(32)
	js.CopyBytesToJS(prf32Arr, prf32)

	var importRes, importRej js.Func
	importRes = js.FuncOf(func(_ js.Value, args []js.Value) any {
		importRes.Release()
		importRej.Release()
		if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
			onDone("", errors.New("prf vault: importKey returned no key"))
			return nil
		}
		aesKey := args[0]

		ivArr := js.Global().Get("Uint8Array").New(len(iv))
		js.CopyBytesToJS(ivArr, iv)

		cipherArr := js.Global().Get("Uint8Array").New(len(cipherBytes))
		js.CopyBytesToJS(cipherArr, cipherBytes)

		decParams := map[string]any{"name": "AES-GCM", "iv": ivArr}
		decPromise := sub.Call("decrypt", decParams, aesKey, cipherArr)

		var decRes, decRej js.Func
		decRes = js.FuncOf(func(_ js.Value, dArgs []js.Value) any {
			decRes.Release()
			decRej.Release()
			if len(dArgs) == 0 || dArgs[0].IsNull() || dArgs[0].IsUndefined() {
				onDone("", errors.New("prf vault: decrypt promise resolved with no value"))
				return nil
			}
			plainBuf := js.Global().Get("Uint8Array").New(dArgs[0])
			plain := make([]byte, plainBuf.Get("length").Int())
			js.CopyBytesToGo(plain, plainBuf)
			onDone(string(plain), nil)
			return nil
		})
		decRej = js.FuncOf(func(_ js.Value, dArgs []js.Value) any {
			decRes.Release()
			decRej.Release()
			msg := "prf vault: decrypt rejected — wrong credential or corrupted vault"
			if len(dArgs) > 0 {
				msg = fmt.Sprintf("%s: %s", msg, dArgs[0].String())
			}
			onDone("", errors.New(msg))
			return nil
		})
		decPromise.Call("then", decRes).Call("catch", decRej)
		return nil
	})
	importRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
		importRes.Release()
		importRej.Release()
		msg := "prf vault: importKey rejected"
		if len(rArgs) > 0 {
			msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
		}
		onDone("", errors.New(msg))
		return nil
	})

	importPromise := sub.Call("importKey",
		"raw",
		prf32Arr,
		map[string]any{"name": "AES-GCM", "length": 256},
		false,
		[]any{"encrypt", "decrypt"},
	)
	importPromise.Call("then", importRes).Call("catch", importRej)
}
