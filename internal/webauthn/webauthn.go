// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package webauthn provides syscall/js wrappers for the WebAuthn API with the
// PRF extension, enabling local passkey-based unlock in CashFlux (C282).
//
// Design notes:
//   - All operations are async (Promise-based via js.FuncOf callbacks). No
//     goroutine channels are used — all callbacks execute on the JS event loop,
//     matching the pattern used throughout internal/app/datasetcrypto.go.
//   - The WASM app self-generates all challenges and PRF salts; no server
//     round-trip is needed, keeping the design fully local-first.
//   - Handle failures gracefully: every exported function reports errors via its
//     onDone callback rather than panicking. Callers MUST keep the passcode path
//     open as a fallback — never gate the whole unlock flow on passkey success.
//   - The PRF extension is used to derive a stable 32-byte secret from the
//     authenticator. This secret is used to encrypt the session passcode, which
//     is then used to unlock the dataset via the existing passcode path.
package webauthn

import (
	"crypto/rand"
	"errors"
	"fmt"
	"syscall/js"
)

// Available calls onResult with true when the browser exposes a
// user-verifying platform authenticator (Touch ID, Face ID, Windows Hello) AND
// the PRF extension is likely supported; false otherwise. The check is fully
// async; onResult is called on the JS event loop. Call Available before
// showing any passkey setup UI so it is hidden on unsupported devices.
func Available(onResult func(bool)) {
	pkc := js.Global().Get("PublicKeyCredential")
	if pkc.IsNull() || pkc.IsUndefined() {
		onResult(false)
		return
	}
	avail := pkc.Get("isUserVerifyingPlatformAuthenticatorAvailable")
	if avail.IsNull() || avail.IsUndefined() || avail.Type() != js.TypeFunction {
		onResult(false)
		return
	}
	promise := pkc.Call("isUserVerifyingPlatformAuthenticatorAvailable")
	if promise.IsNull() || promise.IsUndefined() {
		onResult(false)
		return
	}
	var res, rej js.Func
	res = js.FuncOf(func(_ js.Value, args []js.Value) any {
		res.Release()
		rej.Release()
		if len(args) > 0 && args[0].Bool() {
			checkPRFCapability(pkc, onResult)
		} else {
			onResult(false)
		}
		return nil
	})
	rej = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		res.Release()
		rej.Release()
		onResult(false)
		return nil
	})
	promise.Call("then", res).Call("catch", rej)
}

// checkPRFCapability verifies PRF capability via getClientCapabilities (Chrome
// 132+). Falls back to optimistic true when the API is unavailable — PRF
// failures at usage time produce a recoverable error rather than a UI gap.
func checkPRFCapability(pkc js.Value, onResult func(bool)) {
	gcCap := pkc.Get("getClientCapabilities")
	if gcCap.IsNull() || gcCap.IsUndefined() || gcCap.Type() != js.TypeFunction {
		// API not available; optimistically pass — some browsers support PRF
		// without advertising it via getClientCapabilities.
		onResult(true)
		return
	}
	promise := pkc.Call("getClientCapabilities")
	if promise.IsNull() || promise.IsUndefined() {
		onResult(true)
		return
	}
	var res, rej js.Func
	res = js.FuncOf(func(_ js.Value, args []js.Value) any {
		res.Release()
		rej.Release()
		if len(args) > 0 {
			prf := args[0].Get("prf")
			if !prf.IsNull() && !prf.IsUndefined() {
				onResult(prf.Bool())
				return nil
			}
		}
		onResult(true) // key absent → optimistic
		return nil
	})
	rej = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		res.Release()
		rej.Release()
		onResult(true) // error → optimistic
		return nil
	})
	promise.Call("then", res).Call("catch", rej)
}

// Register calls navigator.credentials.create() with PRF enabled and reports
// the new credential's raw ID via onDone. rpID should be the app's origin
// hostname (e.g. "localhost" or "cashflux.app"). userID is a stable per-user
// identifier; userName is the display name shown in the authenticator UI.
//
// A non-nil error means the ceremony failed or was cancelled — the caller must
// keep the passcode unlock path fully available as a fallback; never block on
// registration success.
//
// Note: PRF output during create() is NOT extracted here because not all
// authenticators return it during registration. A separate GetPRF call is
// required after Register to obtain the PRF output for key wrapping.
func Register(rpID, userID, userName string, onDone func(credentialID []byte, err error)) {
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		onDone(nil, fmt.Errorf("webauthn register: challenge: %w", err))
		return
	}
	creds := js.Global().Get("navigator").Get("credentials")
	if creds.IsNull() || creds.IsUndefined() {
		onDone(nil, errors.New("webauthn register: navigator.credentials unavailable"))
		return
	}
	challengeArr := goToUint8Array(challenge)
	userIDArr := goToUint8Array([]byte(userID))

	opts := map[string]any{
		"publicKey": map[string]any{
			"challenge": challengeArr,
			"rp": map[string]any{
				"id":   rpID,
				"name": "CashFlux",
			},
			"user": map[string]any{
				"id":          userIDArr,
				"name":        userName,
				"displayName": userName,
			},
			"pubKeyCredParams": []any{
				map[string]any{"type": "public-key", "alg": -7},  // ES256
				map[string]any{"type": "public-key", "alg": -257}, // RS256
			},
			"authenticatorSelection": map[string]any{
				"userVerification": "required",
				"residentKey":      "preferred",
			},
			"extensions": map[string]any{
				// Request PRF support; the authenticator may return a PRF output
				// immediately here, but we always make a dedicated GetPRF call
				// afterward for consistency and broader compatibility.
				"prf": map[string]any{},
			},
		},
	}
	promise := creds.Call("create", opts)
	if promise.IsNull() || promise.IsUndefined() {
		onDone(nil, errors.New("webauthn register: create() returned nil"))
		return
	}
	var res, rej js.Func
	res = js.FuncOf(func(_ js.Value, args []js.Value) any {
		res.Release()
		rej.Release()
		if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
			onDone(nil, errors.New("webauthn register: create() returned no credential"))
			return nil
		}
		rawID := args[0].Get("rawId")
		if rawID.IsNull() || rawID.IsUndefined() {
			onDone(nil, errors.New("webauthn register: credential missing rawId"))
			return nil
		}
		idArr := js.Global().Get("Uint8Array").New(rawID)
		onDone(uint8ArrayToGo(idArr), nil)
		return nil
	})
	rej = js.FuncOf(func(_ js.Value, args []js.Value) any {
		res.Release()
		rej.Release()
		msg := "webauthn register: create() cancelled or failed"
		if len(args) > 0 {
			msg = fmt.Sprintf("webauthn register: %s", args[0].String())
		}
		onDone(nil, errors.New(msg))
		return nil
	})
	promise.Call("then", res).Call("catch", rej)
}

// GetPRF calls navigator.credentials.get() with PRF eval on the given salt and
// reports the 32-byte PRF output via onDone. credentialID is the raw credential
// ID returned by Register. salt is a 32-byte application-chosen PRF input that
// must be identical on every call for the same credential (it is stored in
// localStorage alongside the credential ID).
//
// A non-nil error means the ceremony failed — the caller must fall back to the
// passcode path; GetPRF errors are always recoverable.
func GetPRF(rpID string, credentialID []byte, salt []byte, onDone func(prf32 []byte, err error)) {
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		onDone(nil, fmt.Errorf("webauthn get-prf: challenge: %w", err))
		return
	}
	creds := js.Global().Get("navigator").Get("credentials")
	if creds.IsNull() || creds.IsUndefined() {
		onDone(nil, errors.New("webauthn get-prf: navigator.credentials unavailable"))
		return
	}
	challengeArr := goToUint8Array(challenge)
	credIDArr := goToUint8Array(credentialID)
	saltArr := goToUint8Array(salt)

	opts := map[string]any{
		"publicKey": map[string]any{
			"challenge": challengeArr,
			"rpId":      rpID,
			"allowCredentials": []any{
				map[string]any{
					"type": "public-key",
					"id":   credIDArr,
				},
			},
			"userVerification": "required",
			"extensions": map[string]any{
				"prf": map[string]any{
					"eval": map[string]any{
						"first": saltArr,
					},
				},
			},
		},
	}
	promise := creds.Call("get", opts)
	if promise.IsNull() || promise.IsUndefined() {
		onDone(nil, errors.New("webauthn get-prf: get() returned nil"))
		return
	}
	var res, rej js.Func
	res = js.FuncOf(func(_ js.Value, args []js.Value) any {
		res.Release()
		rej.Release()
		if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
			onDone(nil, errors.New("webauthn get-prf: get() returned no credential"))
			return nil
		}
		cred := args[0]
		ext := cred.Call("getClientExtensionResults")
		prf := ext.Get("prf")
		if prf.IsNull() || prf.IsUndefined() {
			onDone(nil, errors.New("webauthn get-prf: PRF extension absent from authenticator response"))
			return nil
		}
		results := prf.Get("results")
		if results.IsNull() || results.IsUndefined() {
			onDone(nil, errors.New("webauthn get-prf: PRF results missing"))
			return nil
		}
		first := results.Get("first")
		if first.IsNull() || first.IsUndefined() {
			onDone(nil, errors.New("webauthn get-prf: PRF first output missing — authenticator may not support PRF"))
			return nil
		}
		prfBytes := uint8ArrayToGo(js.Global().Get("Uint8Array").New(first))
		if len(prfBytes) != 32 {
			onDone(nil, fmt.Errorf("webauthn get-prf: unexpected PRF output length %d (want 32)", len(prfBytes)))
			return nil
		}
		onDone(prfBytes, nil)
		return nil
	})
	rej = js.FuncOf(func(_ js.Value, args []js.Value) any {
		res.Release()
		rej.Release()
		msg := "webauthn get-prf: authentication cancelled or failed"
		if len(args) > 0 {
			msg = fmt.Sprintf("webauthn get-prf: %s", args[0].String())
		}
		onDone(nil, errors.New(msg))
		return nil
	})
	promise.Call("then", res).Call("catch", rej)
}

// goToUint8Array copies b into a new JS Uint8Array.
func goToUint8Array(b []byte) js.Value {
	arr := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(arr, b)
	return arr
}

// uint8ArrayToGo copies a JS Uint8Array into a new Go byte slice.
func uint8ArrayToGo(arr js.Value) []byte {
	n := arr.Get("length").Int()
	b := make([]byte, n)
	js.CopyBytesToGo(b, arr)
	return b
}
