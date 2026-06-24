// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — zero-knowledge artifact blob encryption (EC2).
//
// This file provides synchronous wrappers around the async Web Crypto API for
// encrypting and decrypting artifact blobs before they are uploaded to (or after
// they are downloaded from) the backend blob store.
//
// # Design
//
// Dataset encryption (C45, datasetcrypto.go) is non-deterministic: it generates
// a fresh random IV per call so that identical plaintexts produce different
// ciphertexts. Artifact blobs are content-addressed: the server stores blobs
// keyed by sha256(payload), so identical encrypted payloads from different clients
// must land on the same hash. This is achieved with a deterministic IV:
//
//	IV = sha256(plaintext)[:12]
//
// Security note on IV reuse: AES-GCM requires unique (key, IV) pairs per
// encryption. Here the IV is derived from the plaintext, so two *different*
// plaintexts get different IVs (sha256 collision probability is negligible for
// any realistic artifact corpus). *Identical* plaintexts intentionally produce
// identical ciphertexts — that is the dedup mechanism. This is semantically
// equivalent to a deterministic authenticated cipher and is safe for this
// use-case (content-addressed storage, no chosen-plaintext concern from the
// server, key is never reused across users because it is derived from the
// passcode + a per-install salt).
//
// # Stable salt and session key cache
//
// A stable 16-byte salt is read from localStorage key "cf.artifactSalt"; if
// absent it is generated, stored, and reused. The salt is not secret — its
// purpose is to keep the derived key stable across sessions for a given install
// so that repeated uploads of the same artifact produce the same ciphertext
// (enabling dedup and stable content-address hashes).
//
// PBKDF2 is expensive (600 000 iterations). The derived AES key is cached in
// memory keyed by saltB64, so we pay that cost at most once per unique salt
// per session — at most once for uploads (stable salt) and at most once per
// unique foreign salt encountered during downloads.
//
// # Sync bridge
//
// encryptArtifactSync and decryptArtifactSync present a synchronous interface
// to callers that run inside goroutines (uploadBackendArtifactBlob,
// downloadBackendArtifactBlob). They block on a buffered channel of size 1
// while waiting for the async crypto.subtle Promise to resolve. This is safe
// because the Go/WASM runtime scheduler parks the goroutine and services the
// JS event loop — exactly the same pattern used by sync HTTP calls in wasm.
package app

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

// artifactSaltKey is the localStorage key that holds the stable per-install
// artifact salt (base64-encoded 16 bytes).
const artifactSaltKey = "cf.artifactSalt"

// artifactKeyCache caches derived js.Value CryptoKeys keyed by saltB64 so
// that PBKDF2 key derivation (600 000 iterations) is paid at most once per
// unique salt per session.
var artifactKeyCache = map[string]js.Value{}

// artifactSalt returns (and lazily creates) the stable per-install artifact
// salt. The salt is stored as standard base64 in localStorage so it survives
// page reloads.
func artifactSalt() (string, error) {
	ls := js.Global().Get("localStorage")
	if v := ls.Call("getItem", artifactSaltKey); !v.IsNull() && !v.IsUndefined() {
		if s := v.String(); s != "" {
			return s, nil
		}
	}
	raw := make([]byte, 16)
	if err := cryptoGetRandomValues(raw); err != nil {
		return "", fmt.Errorf("artifact salt: generate random bytes: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	ls.Call("setItem", artifactSaltKey, b64)
	return b64, nil
}

// cachedArtifactKey returns the cached CryptoKey for saltB64, or derives and
// caches it if not present. It blocks the calling goroutine until the async
// PBKDF2 derivation completes.
func cachedArtifactKey(saltB64 string) (js.Value, error) {
	if k, ok := artifactKeyCache[saltB64]; ok {
		return k, nil
	}
	type result struct {
		key js.Value
		err error
	}
	ch := make(chan result, 1)
	deriveKey(activePasscode, saltB64, func(key js.Value, err error) {
		ch <- result{key: key, err: err}
	})
	res := <-ch
	if res.err != nil {
		return js.Undefined(), fmt.Errorf("artifact key derivation: %w", res.err)
	}
	artifactKeyCache[saltB64] = res.key
	slog.Debug("artifact key derived and cached", "saltB64prefix", saltB64[:8])
	return res.key, nil
}

// encryptArtifactSync encrypts plain using a key derived from activePasscode and
// the stable per-install artifact salt, then returns the serialised
// cryptobox.Envelope bytes.
//
// The AES-GCM IV is sha256(plain)[:12], making ciphertext deterministic for
// identical plaintext+key — a deliberate choice that preserves content-addressing
// and per-user dedup on the backend blob store. See package-level doc comment for
// the IV-reuse safety argument.
//
// This function blocks the calling goroutine while async crypto.subtle Promises
// resolve; it is safe to call from a goroutine (the WASM scheduler yields to the
// JS event loop while the goroutine is parked on the channel).
func encryptArtifactSync(plain []byte) ([]byte, error) {
	saltB64, err := artifactSalt()
	if err != nil {
		return nil, fmt.Errorf("encryptArtifactSync: %w", err)
	}

	key, err := cachedArtifactKey(saltB64)
	if err != nil {
		return nil, fmt.Errorf("encryptArtifactSync: %w", err)
	}

	// Deterministic IV: sha256(plaintext)[:12].
	// Two different plaintexts → different sha256 digests → different IVs.
	// Identical plaintext → same IV → same ciphertext (content-address stable).
	digest := sha256.Sum256(plain)
	iv := digest[:12]
	ivB64 := base64.StdEncoding.EncodeToString(iv)

	ivArr := js.Global().Get("Uint8Array").New(len(iv))
	js.CopyBytesToJS(ivArr, iv)

	plainArr := js.Global().Get("Uint8Array").New(len(plain))
	js.CopyBytesToJS(plainArr, plain)

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)

	var encDone, encRej js.Func
	encDone = js.FuncOf(func(_ js.Value, args []js.Value) any {
		encDone.Release()
		if len(args) == 0 {
			ch <- result{err: fmt.Errorf("encryptArtifactSync: encrypt promise rejected (no args)")}
			return nil
		}
		buf := js.Global().Get("Uint8Array").New(args[0])
		out := make([]byte, buf.Get("length").Int())
		js.CopyBytesToGo(out, buf)
		ch <- result{data: out}
		return nil
	})
	encRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
		encRej.Release()
		msg := "encryptArtifactSync: encrypt rejected"
		if len(rArgs) > 0 {
			msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
		}
		ch <- result{err: fmt.Errorf("%s", msg)}
		return nil
	})

	encParams := map[string]any{"name": "AES-GCM", "iv": ivArr}
	subtle().Call("encrypt", encParams, key, plainArr).
		Call("then", encDone).
		Call("catch", encRej)

	res := <-ch
	if res.err != nil {
		return nil, res.err
	}

	env := cryptobox.Envelope{
		V:      cryptobox.CurrentVersion,
		Alg:    cryptobox.AlgAESGCM,
		Salt:   saltB64,
		IV:     ivB64,
		Cipher: base64.StdEncoding.EncodeToString(res.data),
	}
	marshalled := cryptobox.Marshal(env)
	if marshalled == nil {
		return nil, fmt.Errorf("encryptArtifactSync: marshal envelope failed")
	}
	return marshalled, nil
}

// decryptArtifactSync parses envBytes as a cryptobox.Envelope and decrypts its
// payload using a key derived from activePasscode and the salt embedded in the
// envelope. The per-envelope salt lets us correctly decrypt blobs that were
// encrypted with a different install's artifact salt (e.g. multi-device sync).
//
// Like encryptArtifactSync, this blocks the calling goroutine safely.
func decryptArtifactSync(envBytes []byte) ([]byte, error) {
	env, ok := cryptobox.Parse(envBytes)
	if !ok {
		return nil, fmt.Errorf("decryptArtifactSync: invalid envelope")
	}

	key, err := cachedArtifactKey(env.Salt)
	if err != nil {
		return nil, fmt.Errorf("decryptArtifactSync: %w", err)
	}

	ivBytes, err := base64.StdEncoding.DecodeString(env.IV)
	if err != nil {
		return nil, fmt.Errorf("decryptArtifactSync: bad IV base64: %w", err)
	}
	cipherBytes, err := base64.StdEncoding.DecodeString(env.Cipher)
	if err != nil {
		return nil, fmt.Errorf("decryptArtifactSync: bad cipher base64: %w", err)
	}

	ivArr := js.Global().Get("Uint8Array").New(len(ivBytes))
	js.CopyBytesToJS(ivArr, ivBytes)

	cipherArr := js.Global().Get("Uint8Array").New(len(cipherBytes))
	js.CopyBytesToJS(cipherArr, cipherBytes)

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)

	var decDone, decRej js.Func
	decDone = js.FuncOf(func(_ js.Value, args []js.Value) any {
		decDone.Release()
		if len(args) == 0 {
			ch <- result{err: fmt.Errorf("decryptArtifactSync: decrypt promise rejected (no args)")}
			return nil
		}
		buf := js.Global().Get("Uint8Array").New(args[0])
		out := make([]byte, buf.Get("length").Int())
		js.CopyBytesToGo(out, buf)
		ch <- result{data: out}
		return nil
	})
	decRej = js.FuncOf(func(_ js.Value, rArgs []js.Value) any {
		decRej.Release()
		msg := "decryptArtifactSync: decrypt rejected (wrong passcode or corrupted data)"
		if len(rArgs) > 0 {
			msg = fmt.Sprintf("%s: %s", msg, rArgs[0].String())
		}
		ch <- result{err: fmt.Errorf("%s", msg)}
		return nil
	})

	decParams := map[string]any{"name": "AES-GCM", "iv": ivArr}
	subtle().Call("decrypt", decParams, key, cipherArr).
		Call("then", decDone).
		Call("catch", decRej)

	res := <-ch
	if res.err != nil {
		return nil, res.err
	}
	return res.data, nil
}

// clearArtifactKeyCache evicts all cached artifact CryptoKeys. Call this when
// the session passcode changes so stale keys are not reused.
func clearArtifactKeyCache() {
	artifactKeyCache = map[string]js.Value{}
}
