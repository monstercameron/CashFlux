// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func FuzzParseKeyring(f *testing.F) {
	// Stable material makes authenticated seeds valid in every fuzz worker.
	wk, err := workspaceKeyFromMaterial(KeyID{1}, 1, bytes.Repeat([]byte{0x41}, KeySize))
	if err != nil {
		f.Fatal(err)
	}
	defer wk.Destroy()
	kr, err := NewKeyring(wk)
	if err != nil {
		f.Fatal(err)
	}
	if err := kr.AddPasscodeWrap(wk, "fuzz seed", fastDeriver()); err != nil {
		f.Fatal(err)
	}
	blob, err := kr.Marshal(wk)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(blob)
	f.Add([]byte(`{"formatVersion":1,"formatVersion":1}`))
	f.Add([]byte("not a keyring"))
	f.Fuzz(func(t *testing.T, data []byte) {
		done := make(chan error, 1)
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		go func() {
			// Exercise the private bounded decoder too: authenticated mutation
			// normally stops at the MAC, but structural fuzz coverage still matters.
			_, _ = parseUntrusted(data, 1)
			parsed, err := Parse(data, wk, 1)
			if err != nil {
				done <- nil
				return
			}
			encoded, err := parsed.Marshal(wk)
			if err != nil {
				done <- err
				return
			}
			if !bytes.Equal(encoded, data) {
				done <- errNoncanonicalKeyring
				return
			}
			done <- nil
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("keyring fuzz body: %v", err)
			}
		case <-timer.C:
			t.Fatalf("Parse/Marshal did not return for input (%d bytes): %s", len(data), hex.EncodeToString(data))
		}
	})
}

var errNoncanonicalKeyring = errors.New("Parse accepted a noncanonical keyring blob")

func FuzzOpen(f *testing.F) {
	key := bytes.Repeat([]byte{0x42}, KeySize)
	aad := []byte("fuzz authenticated data")
	sealed, err := Seal(key, aad, []byte("fuzz plaintext"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(key, aad, sealed)
	f.Add(key, []byte("different aad"), sealed)
	f.Add(key, aad, []byte{})
	f.Fuzz(func(t *testing.T, key, aad, sealed []byte) {
		_, _ = Open(key, aad, sealed)
	})
}
