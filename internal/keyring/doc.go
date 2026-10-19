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
// Authenticated keyring format (v2).
//
// Marshal requires the current WDK and MACs the entire canonical metadata under
// SubkeyMetadata, including the current version, retained versions, key IDs,
// key-check values and every wrap byte. Parse requires that WDK and a caller's
// durably persisted highest-seen version for this scope/workspace. Unlock is the
// bootstrap path when only a wrap secret is available: candidate metadata and
// keys remain internal until the MAC and rollback floor are checked. Never use
// encoding/json directly to load an untrusted Keyring. The old unsigned v1
// format is rejected; this foundation is not yet integrated into the app.
//
// Pairing protocol (v2).
//
// B (new device) sends H(lp(domain), lp(pubB), lp(nonceB)) first. A (old device)
// freezes that commitment, then sends pubA and nonceA. B freezes A's challenge,
// then reveals pubB and nonceB. A verifies the commitment. Both hash a canonical
// length-prefixed transcript binding the scope/workspace info, commitment,
// pubA, pubB, nonceA and nonceB and display its eight-digit SAS. Transfer APIs
// require explicit human confirmation of the OTHER device's code. A mismatch
// aborts; a retry requires fresh keys and nonces. An active MITM's per-attempt
// success probability is approximately 1 in 100 million, not zero. The caller
// must limit retries and never automatically compare codes sent by the relay.
//
// A seals the current WDK's ID, version and material with HPKE base mode, then
// MACs the encapsulated key, ciphertext, keyring digest and transcript with a
// dedicated key derived from the ephemeral ECDH secret and transcript hash.
// The transcript alone is public and is NOT a MAC key. B verifies this sender
// MAC before opening HPKE, then checks the received WDK against the authenticated
// keyring identity, key-check value and highest-seen version. Each confirmed
// session permits one transfer attempt and is discarded on success or failure.
// The exported low-level SealForDevice/OpenOnDevice helpers remain HPKE base
// primitives; they do not authenticate a sender and must not transfer WDKs.
//
// This package is pure Go (plus an optional WebCrypto PBKDF2 accelerator on
// js/wasm) and holds no global state. Secrets are copied defensively; callers
// should Destroy workspace keys and zero KEKs when locking.
package keyring
