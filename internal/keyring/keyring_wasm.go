// SPDX-License-Identifier: MIT

//go:build js && wasm

package keyring

import (
	"errors"
	"fmt"
	"syscall/js"
)

// This file is the browser accelerator for passcode key derivation.
// PBKDF2-SHA-256 at PasscodeIterations (600,000) takes seconds in pure-Go
// wasm but runs on optimized native code through WebCrypto's
// crypto.subtle.deriveBits. WebCrypto is asynchronous, so — like the app's
// existing dataset crypto — the derivation reports through a callback
// rather than blocking: a blocked Go goroutine waiting on a Promise can
// deadlock the page if it was started from a JS callback.
//
// Browser flow for one unlock:
//
//	keyring.WebCryptoPBKDF2(passcode, entry.Salt, entry.Iterations, 32,
//	    func(kek []byte, err error) {
//	        // hand kek to the synchronous unwrap via FixedKEKDeriver
//	        wk, err := kr.Unwrap(keyring.WrapPasscode, "", keyring.UnwrapSecret{
//	            Passcode: passcode,
//	            Deriver:  keyring.FixedKEKDeriver{KEK: kek},
//	        })
//	    })
//
// The returned KEK bytes are the caller's to zero after use. Derivation
// must run once per unlock, never per save: the entry's stored salt makes
// that possible.

// WebCryptoPBKDF2 derives keyLen bytes from password and salt using
// PBKDF2-SHA-256 through crypto.subtle and calls onDone exactly once with
// either the derived bytes or an error. onDone must not be nil.
func WebCryptoPBKDF2(password string, salt []byte, iterations, keyLen int, onDone func(key []byte, err error)) {
	if onDone == nil {
		return
	}
	if iterations <= 0 || keyLen <= 0 {
		onDone(nil, fmt.Errorf("%w: pbkdf2 parameters must be positive", ErrInvalidSecret))
		return
	}
	cryptoObj := js.Global().Get("crypto")
	if cryptoObj.IsNull() || cryptoObj.IsUndefined() {
		onDone(nil, errors.New("keyring: Web Crypto API unavailable"))
		return
	}
	subtle := cryptoObj.Get("subtle")
	if subtle.IsNull() || subtle.IsUndefined() {
		onDone(nil, errors.New("keyring: crypto.subtle unavailable (needs a secure context)"))
		return
	}

	passArr := uint8ArrayFromBytes([]byte(password))
	saltArr := uint8ArrayFromBytes(salt)

	var importOK, importFail, deriveOK, deriveFail js.Func
	release := func() {
		importOK.Release()
		importFail.Release()
		deriveOK.Release()
		deriveFail.Release()
	}
	fail := func(stage string, args []js.Value) {
		msg := "keyring: WebCrypto PBKDF2 " + stage + " rejected"
		if len(args) > 0 {
			msg = fmt.Sprintf("%s: %s", msg, args[0].String())
		}
		release()
		onDone(nil, errors.New(msg))
	}

	deriveOK = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			fail("deriveBits", args)
			return nil
		}
		buf := js.Global().Get("Uint8Array").New(args[0])
		key := make([]byte, buf.Get("length").Int())
		js.CopyBytesToGo(key, buf)
		release()
		onDone(key, nil)
		return nil
	})
	deriveFail = js.FuncOf(func(_ js.Value, args []js.Value) any {
		fail("deriveBits", args)
		return nil
	})

	importOK = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			fail("importKey", args)
			return nil
		}
		params := map[string]any{
			"name":       "PBKDF2",
			"hash":       "SHA-256",
			"salt":       saltArr,
			"iterations": iterations,
		}
		promise := subtle.Call("deriveBits", params, args[0], keyLen*8)
		promise.Call("then", deriveOK).Call("catch", deriveFail)
		return nil
	})
	importFail = js.FuncOf(func(_ js.Value, args []js.Value) any {
		fail("importKey", args)
		return nil
	})

	importPromise := subtle.Call("importKey",
		"raw",
		passArr,
		map[string]any{"name": "PBKDF2"},
		false,
		[]any{"deriveBits"},
	)
	importPromise.Call("then", importOK).Call("catch", importFail)
}

// uint8ArrayFromBytes copies b into a fresh Uint8Array.
func uint8ArrayFromBytes(b []byte) js.Value {
	arr := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(arr, b)
	return arr
}
