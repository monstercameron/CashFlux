// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"testing"
)

func TestPairingRequestAndWorkspaceKeyRoundTrip(t *testing.T) {
	request, err := NewPairingRequest()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(request.PublicKey), PairingPublicKeySize; got != want {
		t.Fatalf("public key size = %d, want %d", got, want)
	}
	if got, want := request.ConfirmationCode, ConfirmationCodeForPublicKey(request.PublicKey); got != want {
		t.Fatalf("confirmation code = %q, want %q", got, want)
	}

	wk := mustKey(t)
	info := PairingInfo(Scope{ServerID: "server-a", AccountID: "account-a"}, "workspace-a")
	enc, ciphertext, err := SealWorkspaceKeyForDevice(request.PublicKey, wk, info)
	if err != nil {
		t.Fatalf("SealWorkspaceKeyForDevice: %v", err)
	}
	got, err := OpenWorkspaceKeyOnDevice(request.PrivateKey, enc, ciphertext, info)
	if err != nil {
		t.Fatalf("OpenWorkspaceKeyOnDevice: %v", err)
	}
	sameKey(t, wk, got)

	otherInfo := PairingInfo(Scope{ServerID: "server-b", AccountID: "account-a"}, "workspace-a")
	if _, err := OpenWorkspaceKeyOnDevice(request.PrivateKey, enc, ciphertext, otherInfo); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("open under different pairing scope error = %v, want authentication failure", err)
	}
}

func TestPairingRawMaterialAndTampering(t *testing.T) {
	request, err := NewPairingRequest()
	if err != nil {
		t.Fatal(err)
	}
	info := []byte("pairing test")
	material := bytes.Repeat([]byte{0x93}, KeySize)
	enc, ciphertext, err := SealForDevice(request.PublicKey, material, info)
	if err != nil {
		t.Fatalf("SealForDevice: %v", err)
	}
	got, err := OpenOnDevice(request.PrivateKey, enc, ciphertext, info)
	if err != nil {
		t.Fatalf("OpenOnDevice: %v", err)
	}
	if !bytes.Equal(got, material) {
		t.Fatal("opened pairing material differs")
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := OpenOnDevice(request.PrivateKey, enc, ciphertext, info); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("tampered ciphertext error = %v, want authentication failure", err)
	}
}

func TestPairingPrivateKeyEncoding(t *testing.T) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePairingPrivateKey(privateKey.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsed.PublicKey().Bytes(), privateKey.PublicKey().Bytes()) {
		t.Fatal("parsed pairing key has a different public key")
	}
	if _, err := ParsePairingPrivateKey([]byte("short")); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid private key error = %v, want ErrInvalidKey", err)
	}
}
