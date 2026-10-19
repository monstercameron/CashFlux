// SPDX-License-Identifier: MIT

// Package keyring implements the CashFlux v2 workspace encryption keys.
//
// Every workspace has one random 256-bit Workspace Data Key (WDK) per key
// version. Data is encrypted under HKDF subkeys derived from the WDK, never
// under a passcode directly. The WDK itself is stored only wrapped (encrypted)
// by one or more independent wrap entries: a passcode (PBKDF2-SHA-256), a
// one-time-shown recovery code, a passkey (WebAuthn PRF output), or a device
// key held by the platform. Adding or removing a way to unlock therefore never
// re-encrypts the data, and changing a passcode only rewraps the WDK.
//
// Threat model.
//
// The sync server is untrusted for confidentiality. It stores only the
// keyring blob (wrapped keys, salts, credential IDs — none of which can
// decrypt anything on their own) and ciphertext sealed with Seal. From those
// it can learn metadata it already needs to route blobs (workspace, sizes,
// timing, key versions, how many unlock methods exist, passkey credential
// IDs), but it cannot learn the WDK, any subkey, or any plaintext, and it
// cannot forge ciphertext: every sealed blob and every wrapped key is
// authenticated, and the authenticated data names the scope (server and
// account), workspace, change, and key version it belongs to, so a blob moved
// to the wrong account or workspace fails to open instead of merging.
//
// What each secret protects against:
//
//   - Passcode wrap: an attacker who obtains the keyring blob (from the
//     server or a stolen backup) must guess the passcode through
//     PBKDF2-SHA-256 with 600,000 iterations. A weak passcode is still
//     guessable; the passcode does not protect against malware running in
//     an already-unlocked client.
//   - Recovery code: 128 bits of entropy. Anyone who holds the code and the
//     keyring blob can unlock the workspace, so it must be stored like a
//     password. Its checksum character only catches typos; it adds no entropy.
//   - Passkey wrap: unwrapping needs the authenticator's PRF output, which
//     never leaves the authenticator except as that output. The stored
//     credential ID and PRF salt are not secret.
//   - Device wrap: the device key never enters this package (in the browser
//     it is a non-extractable WebCrypto key). It protects data at rest on
//     that device only.
//
// "No passcode" means there is no passcode wrap entry. The workspace is then
// only as protected as its remaining wraps: typically the device wrap alone,
// which stops a server or backup thief (they have no device key) but not
// someone with unlocked access to the device itself. A keyring with no wrap
// entries at all cannot be unlocked by anyone — the WDK is unrecoverable —
// so callers must add at least one wrap before discarding the in-memory key,
// and must keep old key versions in the keyring for as long as ciphertext
// sealed under them exists.
//
// This package is pure Go (plus an optional WebCrypto PBKDF2 accelerator on
// js/wasm) and holds no global state. Secrets are copied defensively; callers
// should Destroy workspace keys and zero KEKs when locking.
package keyring
