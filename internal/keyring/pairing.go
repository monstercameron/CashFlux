// SPDX-License-Identifier: MIT

package keyring

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/hpke"
	"encoding/binary"
	"fmt"
)

// PairingPublicKeySize is the size in bytes of an X25519 pairing public key.
const PairingPublicKeySize = 32

// ParsePairingPublicKey validates raw public key bytes received from a new
// device (for example scanned from a QR code).
func ParsePairingPublicKey(raw []byte) (*ecdh.PublicKey, error) {
	pub, err := ecdh.X25519().NewPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: pairing public key: %v", ErrInvalidKey, err)
	}
	return pub, nil
}

// ParsePairingPrivateKey restores a raw key for low-level HPKE use. A SAS
// exchange cannot be resumed with just a private key: restart it with fresh
// state through NewPairingRequest after an interruption.
func ParsePairingPrivateKey(raw []byte) (*ecdh.PrivateKey, error) {
	priv, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: pairing private key: %v", ErrInvalidKey, err)
	}
	return priv, nil
}

// SealForDevice seals bytes to a recipient's public key with HPKE base mode.
// info binds the transfer to its context (use
// PairingInfo); it must match on the receiving side. It returns the HPKE
// encapsulated key and the ciphertext separately, ready to relay.
// This low-level HPKE base-mode primitive does NOT authenticate a sender.
// Workspace transfers must use SealWorkspaceKeyForDevice with a confirmed SAS.
func SealForDevice(recipientPub, material, info []byte) (enc, ciphertext []byte, err error) {
	return sealForDeviceAAD(recipientPub, material, info, nil)
}

// sealForDeviceAAD is SealForDevice with explicit HPKE additional data,
// split out so the RFC 9180 test vectors (which use per-message AAD) can be
// verified against the same suite wiring.
func sealForDeviceAAD(recipientPub, material, info, aad []byte) (enc, ciphertext []byte, err error) {
	pub, err := ParsePairingPublicKey(recipientPub)
	if err != nil {
		return nil, nil, err
	}
	hpkePub, err := hpke.NewDHKEMPublicKey(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("keyring: hpke public key: %w", err)
	}
	enc, sender, err := hpke.NewSender(hpkePub, hpke.HKDFSHA256(), hpke.AES256GCM(), info)
	if err != nil {
		return nil, nil, fmt.Errorf("keyring: hpke sender: %w", err)
	}
	ciphertext, err = sender.Seal(aad, material)
	if err != nil {
		return nil, nil, fmt.Errorf("keyring: hpke seal: %w", err)
	}
	return enc, ciphertext, nil
}

// OpenOnDevice opens material sealed by SealForDevice, on the new device.
// info must match the sender's. A mismatch of key, info, or ciphertext
// returns an *AuthenticationError. This primitive does not authenticate the
// sender; use OpenWorkspaceKeyOnDevice for a workspace key transfer.
func OpenOnDevice(priv *ecdh.PrivateKey, enc, ciphertext, info []byte) ([]byte, error) {
	return openOnDeviceAAD(priv, enc, ciphertext, info, nil)
}

// openOnDeviceAAD is OpenOnDevice with explicit HPKE additional data.
func openOnDeviceAAD(priv *ecdh.PrivateKey, enc, ciphertext, info, aad []byte) ([]byte, error) {
	if priv == nil {
		return nil, fmt.Errorf("%w: pairing private key is nil", ErrInvalidKey)
	}
	hpkePriv, err := hpke.NewDHKEMPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("keyring: hpke private key: %w", err)
	}
	recipient, err := hpke.NewRecipient(enc, hpkePriv, hpke.HKDFSHA256(), hpke.AES256GCM(), info)
	if err != nil {
		return nil, &AuthenticationError{Operation: "open pairing", Err: err}
	}
	material, err := recipient.Open(aad, ciphertext)
	if err != nil {
		return nil, &AuthenticationError{Operation: "open pairing", Err: err}
	}
	return material, nil
}

// pairingInfoDomain labels pairing HPKE info strings.
const pairingInfoDomain = "cfv2/pairing/v2"

// PairingInfo builds the HPKE info for a pairing transfer, binding it to the
// scope and workspace the key belongs to. Both devices must build it from
// the same values; a key sealed for one workspace cannot be opened as
// another's.
func PairingInfo(scope Scope, workspaceID string) []byte {
	return joinLengthPrefixed(
		[]byte(pairingInfoDomain),
		[]byte(scope.ServerID),
		[]byte(scope.AccountID),
		[]byte(workspaceID),
	)
}

// SealWorkspaceKeyForDevice seals a whole workspace key (identity plus
// material) using a confirmed old-device session. It MACs the HPKE output and
// keyring digest with the transcript-bound ECDH secret, authenticating the
// sender as the peer whose SAS the user compared. keyringBlob must be Marshal's
// authenticated current keyring. A confirmed session is consumed on any attempt.
func SealWorkspaceKeyForDevice(session *PairingSession, wk *WorkspaceKey, keyringBlob []byte, highestSeen uint32) (enc, ciphertext, tag []byte, err error) {
	if err := session.requireConfirmed(true); err != nil {
		return nil, nil, nil, err
	}
	defer session.Destroy()
	if wk.Destroyed() {
		return nil, nil, nil, fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	if _, err := Parse(keyringBlob, wk, highestSeen); err != nil {
		return nil, nil, nil, err
	}
	var versionBytes [4]byte
	binary.BigEndian.PutUint32(versionBytes[:], wk.version)
	payload := joinLengthPrefixed(wk.keyID[:], versionBytes[:], wk.material)
	defer clearBytes(payload)
	enc, ciphertext, err = SealForDevice(session.recipient, payload, session.info)
	if err != nil {
		return nil, nil, nil, err
	}
	return enc, ciphertext, session.transferMAC(enc, ciphertext, keyringBlob), nil
}

// OpenWorkspaceKeyOnDevice reverses SealWorkspaceKeyForDevice, returning the
// transferred workspace key only after authenticating the sender, received key
// identity/material against the keyring, full metadata and rollback floor.
// A relayed keyring is untrusted until this succeeds. Persist the returned key's
// version floor per scope/workspace before acknowledging pairing. The confirmed
// new-device session is consumed on success or failure, preventing replay.
func OpenWorkspaceKeyOnDevice(session *PairingSession, enc, ciphertext, tag, keyringBlob []byte, highestSeen uint32) (*WorkspaceKey, error) {
	if err := session.requireConfirmed(false); err != nil {
		return nil, err
	}
	defer session.Destroy()
	// Bound work on relay-controlled data before hashing or decrypting. A
	// workspace payload is exactly three length-prefixed, fixed-size fields.
	const payloadSize = 3*4 + KeyIDSize + 4 + KeySize
	if len(enc) != PairingPublicKeySize || len(ciphertext) != payloadSize+16 || len(tag) != KeySize || len(keyringBlob) == 0 || len(keyringBlob) > MaxKeyringSize {
		return nil, &AuthenticationError{Operation: "pairing transfer size"}
	}
	if !hmac.Equal(tag, session.transferMAC(enc, ciphertext, keyringBlob)) {
		return nil, &AuthenticationError{Operation: "pairing sender MAC"}
	}
	payload, err := OpenOnDevice(session.private, enc, ciphertext, session.info)
	if err != nil {
		return nil, err
	}
	defer clearBytes(payload)
	parts, err := splitLengthPrefixed(payload, 3)
	if err != nil {
		return nil, fmt.Errorf("%w: pairing payload: %v", ErrInvalidKey, err)
	}
	keyID, err := ParseKeyID(parts[0])
	if err != nil {
		return nil, err
	}
	if len(parts[1]) != 4 {
		return nil, fmt.Errorf("%w: pairing payload version is %d bytes", ErrInvalidKey, len(parts[1]))
	}
	version := binary.BigEndian.Uint32(parts[1])
	wk, err := workspaceKeyFromMaterial(keyID, version, parts[2])
	if err != nil {
		return nil, err
	}
	if _, err := Parse(keyringBlob, wk, highestSeen); err != nil {
		wk.Destroy()
		return nil, err
	}
	return wk, nil
}

// splitLengthPrefixed decodes exactly want length-prefixed parts produced by
// joinLengthPrefixed, rejecting truncation and trailing bytes.
func splitLengthPrefixed(data []byte, want int) ([][]byte, error) {
	parts := make([][]byte, 0, want)
	rest := data
	for len(parts) < want {
		if len(rest) < 4 {
			return nil, fmt.Errorf("truncated length prefix")
		}
		n := binary.BigEndian.Uint32(rest[:4])
		rest = rest[4:]
		if uint64(n) > uint64(len(rest)) {
			return nil, fmt.Errorf("part length %d exceeds remaining %d bytes", n, len(rest))
		}
		parts = append(parts, rest[:n])
		rest = rest[n:]
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("%d trailing bytes", len(rest))
	}
	return parts, nil
}
