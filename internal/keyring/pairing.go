// SPDX-License-Identifier: MIT

package keyring

import (
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// Device pairing transfers workspace key material from an existing device to
// a new one without the server (which may relay the ciphertext) learning it.
// The new device generates an ephemeral X25519 key pair and shows its public
// key (for example as a QR code) plus a short confirmation code. The user
// checks the same code on both devices — that binds the public key they are
// looking at to the device in front of them, so a relay cannot substitute
// its own key unnoticed. The old device then seals the key material to that
// public key with HPKE (RFC 9180, base mode, DHKEM(X25519, HKDF-SHA-256),
// HKDF-SHA-256, AES-256-GCM).
//
// Pairing is unauthenticated by design (HPKE base mode): the confirmation
// code comparison by the user is the authentication. Do not skip it.

// PairingPublicKeySize is the size in bytes of an X25519 pairing public key.
const PairingPublicKeySize = 32

// PairingRequest is a new device's half of a pairing: its ephemeral key pair
// plus the human confirmation code derived from the public key. Keep the
// private key on the new device only and discard the request after pairing.
type PairingRequest struct {
	// PrivateKey is the new device's ephemeral X25519 private key. Never
	// send it anywhere.
	PrivateKey *ecdh.PrivateKey
	// PublicKey is the raw 32-byte X25519 public key to show the old device
	// (typically as a QR code).
	PublicKey []byte
	// ConfirmationCode is the 6-digit code both devices display so the
	// user can confirm they are looking at the same public key.
	ConfirmationCode string
}

// NewPairingRequest generates a fresh ephemeral pairing key pair.
func NewPairingRequest() (*PairingRequest, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("keyring: generate pairing key: %w", err)
	}
	pub := priv.PublicKey().Bytes()
	return &PairingRequest{
		PrivateKey:       priv,
		PublicKey:        cloneBytes(pub),
		ConfirmationCode: ConfirmationCodeForPublicKey(pub),
	}, nil
}

// ConfirmationCodeForPublicKey derives the 6-digit human confirmation code
// for a pairing public key: the first four bytes of its SHA-256 hash, taken
// modulo one million and zero-padded. Both devices derive it from the same
// public key bytes; equal codes mean both sides hold the same key (to the
// strength of a 6-digit code — a user-checked fingerprint, not a secret).
func ConfirmationCodeForPublicKey(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	n := binary.BigEndian.Uint32(sum[:4]) % 1_000_000
	return fmt.Sprintf("%06d", n)
}

// ParsePairingPublicKey validates raw public key bytes received from a new
// device (for example scanned from a QR code).
func ParsePairingPublicKey(raw []byte) (*ecdh.PublicKey, error) {
	pub, err := ecdh.X25519().NewPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: pairing public key: %v", ErrInvalidKey, err)
	}
	return pub, nil
}

// ParsePairingPrivateKey restores a pairing private key from its raw bytes
// (PrivateKey.Bytes), for a pairing that spans an app restart on the new
// device.
func ParsePairingPrivateKey(raw []byte) (*ecdh.PrivateKey, error) {
	priv, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: pairing private key: %v", ErrInvalidKey, err)
	}
	return priv, nil
}

// SealForDevice seals key material (typically a WDK's Material) to the new
// device's pairing public key. info binds the transfer to its context (use
// PairingInfo); it must match on the receiving side. It returns the HPKE
// encapsulated key and the ciphertext separately, ready to relay.
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
// returns an *AuthenticationError.
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
const pairingInfoDomain = "cfv2/pairing/v1"

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
// material) for transfer to a new device. The payload is a length-prefixed
// bundle of KeyID, version, and material; open it with
// OpenWorkspaceKeyOnDevice. info should come from PairingInfo.
func SealWorkspaceKeyForDevice(recipientPub []byte, wk *WorkspaceKey, info []byte) (enc, ciphertext []byte, err error) {
	if wk.Destroyed() {
		return nil, nil, fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	var versionBytes [4]byte
	binary.BigEndian.PutUint32(versionBytes[:], wk.version)
	payload := joinLengthPrefixed(wk.keyID[:], versionBytes[:], wk.material)
	defer clearBytes(payload)
	return SealForDevice(recipientPub, payload, info)
}

// OpenWorkspaceKeyOnDevice reverses SealWorkspaceKeyForDevice, returning the
// transferred workspace key ready to unwrap (or re-wrap for this device).
func OpenWorkspaceKeyOnDevice(priv *ecdh.PrivateKey, enc, ciphertext, info []byte) (*WorkspaceKey, error) {
	payload, err := OpenOnDevice(priv, enc, ciphertext, info)
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
	return workspaceKeyFromMaterial(keyID, version, parts[2])
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
