// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — wasm-side dataset encryption helpers (C45).
//
// This file wires crypto.subtle (Web Cryptography API) to the CashFlux
// envelope format defined in internal/cryptobox. All operations are
// asynchronous: they drive Promise chains via js.FuncOf callbacks and report
// results through a Go callback so callers stay non-blocking.
//
// Security contract:
//   - The derived AES key never leaves the JS runtime; only the envelope
//     (salt + IV + ciphertext) is persisted.
//   - No passcode ⟹ no encryption: callers must check that a passcode is set
//     before calling encryptDataset; with no passcode, data stays plaintext.
//   - Decryption failure is non-fatal: the caller receives an error and keeps
//     the untouched ciphertext rather than wiping it.
package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

// subtle is a convenience accessor for crypto.subtle.
func subtle() js.Value { return js.Global().Get("crypto").Get("subtle") }

// cryptoGetRandomValues fills dst with cryptographically secure random bytes
// using crypto.getRandomValues. Returns an error when the Web Crypto API is
// unavailable (e.g. non-secure context).
func cryptoGetRandomValues(dst []byte) error {
	crypto := js.Global().Get("crypto")
	if crypto.IsNull() || crypto.IsUndefined() {
		return errors.New("cryptobox: crypto API unavailable")
	}
	arr := js.Global().Get("Uint8Array").New(len(dst))
	crypto.Call("getRandomValues", arr)
	js.CopyBytesToGo(dst, arr)
	return nil
}

// deriveKey derives an AES-GCM-256 CryptoKey from passcode and saltB64
// (a base64-encoded 16-byte salt) using PBKDF2-SHA-256 with
// cryptobox.PBKDF2Iterations iterations. The key is passed to onDone; both
// arguments are mutually exclusive (key is zero on error, err is nil on
// success).
//
// All js.FuncOf callbacks are Released before onDone is called.
func deriveKey(passcode string, saltB64 string, onDone func(key js.Value, err error)) {
	saltBytes, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		onDone(js.Undefined(), fmt.Errorf("cryptobox: bad salt base64: %w", err))
		return
	}

	sub := subtle()
	crypto := js.Global().Get("crypto")
	if sub.IsNull() || sub.IsUndefined() || crypto.IsNull() || crypto.IsUndefined() {
		onDone(js.Undefined(), errors.New("cryptobox: Web Crypto API unavailable"))
		return
	}

	// Step 1: importKey the raw passcode bytes as a PBKDF2 base key.
	passBytes := []byte(passcode)
	passArr := js.Global().Get("Uint8Array").New(len(passBytes))
	js.CopyBytesToJS(passArr, passBytes)

	saltArr := js.Global().Get("Uint8Array").New(len(saltBytes))
	js.CopyBytesToJS(saltArr, saltBytes)

	var importDone, deriveDone js.Func

	importDone = js.FuncOf(func(_ js.Value, args []js.Value) any {
		importDone.Release()
		if len(args) == 0 {
			onDone(js.Undefined(), errors.New("cryptobox: importKey promise rejected"))
			return nil
		}
		baseKey := args[0]

		// Step 2: deriveKey PBKDF2 → AES-GCM 256.
		pbkdf2Params := map[string]any{
			"name":       "PBKDF2",
			"hash":       "SHA-256",
			"salt":       saltArr,
			"iterations": cryptobox.PBKDF2Iterations,
		}
		aesParams := map[string]any{"name": "AES-GCM", "length": 256}

		derivePromise := sub.Call("deriveKey",
			pbkdf2Params,
			baseKey,
			aesParams,
			false, // extractable = false — key never leaves JS
			[]any{"encrypt", "decrypt"},
		)

		deriveDone = js.FuncOf(func(_ js.Value, dArgs []js.Value) any {
			deriveDone.Release()
			if len(dArgs) == 0 {
				onDone(js.Undefined(), errors.New("cryptobox: deriveKey promise rejected"))
				return nil
			}
			onDone(dArgs[0], nil)
			return nil
		})
		var deriveRej js.Func
		deriveRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
			deriveRej.Release()
			msg := "cryptobox: deriveKey rejected"
			if len(rArgs) > 0 {
				msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
			}
			onDone(js.Undefined(), errors.New(msg))
			return nil
		})
		derivePromise.Call("then", deriveDone).Call("catch", deriveRej)
		return nil
	})

	var importRej js.Func
	importRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
		importRej.Release()
		msg := "cryptobox: importKey rejected"
		if len(rArgs) > 0 {
			msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
		}
		onDone(js.Undefined(), errors.New(msg))
		return nil
	})

	importPromise := sub.Call("importKey",
		"raw",
		passArr,
		map[string]any{"name": "PBKDF2"},
		false,
		[]any{"deriveKey"},
	)
	importPromise.Call("then", importDone).Call("catch", importRej)
}

// encryptDataset encrypts plaintext under a key derived from passcode, then
// marshals the result into a cryptobox.Envelope and calls onDone with the
// serialised envelope bytes. On any failure, onDone receives a non-nil error
// and nil bytes — the caller must keep the original plaintext in that case.
//
// A fresh random 16-byte salt and 12-byte IV are generated for every call so
// that ciphertexts are non-deterministic even for identical inputs.
func encryptDataset(plaintext []byte, passcode string, onDone func(env []byte, err error)) {
	// Generate salt (16 bytes) and IV (12 bytes).
	salt := make([]byte, 16)
	iv := make([]byte, 12)
	if err := cryptoGetRandomValues(salt); err != nil {
		onDone(nil, fmt.Errorf("cryptobox: salt generation failed: %w", err))
		return
	}
	if err := cryptoGetRandomValues(iv); err != nil {
		onDone(nil, fmt.Errorf("cryptobox: iv generation failed: %w", err))
		return
	}

	saltB64 := base64.StdEncoding.EncodeToString(salt)
	ivB64 := base64.StdEncoding.EncodeToString(iv)

	deriveKey(passcode, saltB64, func(key js.Value, err error) {
		if err != nil {
			onDone(nil, err)
			return
		}

		// Build the AES-GCM encrypt parameters.
		ivArr := js.Global().Get("Uint8Array").New(len(iv))
		js.CopyBytesToJS(ivArr, iv)

		plainArr := js.Global().Get("Uint8Array").New(len(plaintext))
		js.CopyBytesToJS(plainArr, plaintext)

		encParams := map[string]any{"name": "AES-GCM", "iv": ivArr}
		encPromise := subtle().Call("encrypt", encParams, key, plainArr)

		var encDone js.Func
		encDone = js.FuncOf(func(_ js.Value, args []js.Value) any {
			encDone.Release()
			if len(args) == 0 {
				onDone(nil, errors.New("cryptobox: encrypt promise rejected"))
				return nil
			}
			// args[0] is an ArrayBuffer; wrap it as Uint8Array to read bytes.
			cipherBuf := js.Global().Get("Uint8Array").New(args[0])
			cipherBytes := make([]byte, cipherBuf.Get("length").Int())
			js.CopyBytesToGo(cipherBytes, cipherBuf)

			env := cryptobox.Envelope{
				V:      cryptobox.CurrentVersion,
				Alg:    cryptobox.AlgAESGCM,
				Salt:   saltB64,
				IV:     ivB64,
				Cipher: base64.StdEncoding.EncodeToString(cipherBytes),
			}
			onDone(cryptobox.Marshal(env), nil)
			return nil
		})
		var encRej js.Func
		encRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
			encRej.Release()
			msg := "cryptobox: encrypt rejected"
			if len(rArgs) > 0 {
				msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
			}
			onDone(nil, errors.New(msg))
			return nil
		})
		encPromise.Call("then", encDone).Call("catch", encRej)
	})
}

// decryptDataset decrypts an Envelope using a key derived from passcode and
// calls onDone with the recovered plaintext bytes. On any failure (wrong
// passcode, corrupted ciphertext, API error) onDone receives a non-nil error
// and nil bytes; the caller must keep the ciphertext untouched.
func decryptDataset(env cryptobox.Envelope, passcode string, onDone func(plaintext []byte, err error)) {
	deriveKey(passcode, env.Salt, func(key js.Value, err error) {
		if err != nil {
			onDone(nil, err)
			return
		}

		ivBytes, err := base64.StdEncoding.DecodeString(env.IV)
		if err != nil {
			onDone(nil, fmt.Errorf("cryptobox: bad iv base64: %w", err))
			return
		}
		cipherBytes, err := base64.StdEncoding.DecodeString(env.Cipher)
		if err != nil {
			onDone(nil, fmt.Errorf("cryptobox: bad cipher base64: %w", err))
			return
		}

		ivArr := js.Global().Get("Uint8Array").New(len(ivBytes))
		js.CopyBytesToJS(ivArr, ivBytes)

		cipherArr := js.Global().Get("Uint8Array").New(len(cipherBytes))
		js.CopyBytesToJS(cipherArr, cipherBytes)

		decParams := map[string]any{"name": "AES-GCM", "iv": ivArr}
		decPromise := subtle().Call("decrypt", decParams, key, cipherArr)

		var decDone js.Func
		decDone = js.FuncOf(func(_ js.Value, args []js.Value) any {
			decDone.Release()
			if len(args) == 0 {
				onDone(nil, errors.New("cryptobox: decrypt promise rejected"))
				return nil
			}
			plainBuf := js.Global().Get("Uint8Array").New(args[0])
			plain := make([]byte, plainBuf.Get("length").Int())
			js.CopyBytesToGo(plain, plainBuf)
			onDone(plain, nil)
			return nil
		})
		var decRej js.Func
		decRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
			decRej.Release()
			msg := "cryptobox: decrypt rejected (wrong passcode or corrupted data)"
			if len(rArgs) > 0 {
				msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
			}
			onDone(nil, errors.New(msg))
			return nil
		})
		decPromise.Call("then", decDone).Call("catch", decRej)
	})
}
