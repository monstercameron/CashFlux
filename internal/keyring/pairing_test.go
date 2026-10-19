// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"testing"
)

func TestPairingRawMaterialAndTampering(t *testing.T) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	info := []byte("pairing test")
	material := bytes.Repeat([]byte{0x93}, KeySize)
	enc, ciphertext, err := SealForDevice(private.PublicKey().Bytes(), material, info)
	if err != nil {
		t.Fatalf("SealForDevice: %v", err)
	}
	got, err := OpenOnDevice(private, enc, ciphertext, info)
	if err != nil {
		t.Fatalf("OpenOnDevice: %v", err)
	}
	if !bytes.Equal(got, material) {
		t.Fatal("opened pairing material differs")
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := OpenOnDevice(private, enc, ciphertext, info); !errors.Is(err, ErrAuthenticationFailed) {
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
