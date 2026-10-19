// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func FuzzPairingReveal(f *testing.F) {
	f.Add(bytes.Repeat([]byte{1}, PairingPublicKeySize), bytes.Repeat([]byte{2}, PairingNonceSize), false)
	f.Add([]byte{1}, []byte{}, true)
	f.Add(make([]byte, PairingPublicKeySize), make([]byte, PairingNonceSize), false)
	f.Fuzz(func(t *testing.T, pub, nonce []byte, corrupt bool) {
		// Bound the test harness before building an adversarial commitment.
		if len(pub) > 64 || len(nonce) > 64 {
			return
		}
		commitment := pairingCommitment(pub, nonce)
		if corrupt {
			commitment[0] ^= 1
		}
		r := fixedResponder(t, commitment, PairingInfo(LocalScope("a"), "w"), 3)
		s, err := r.Finish(PairingReveal{PublicKey: pub, Nonce: nonce})
		if err == nil {
			defer s.Destroy()
			if corrupt || len(pub) != PairingPublicKeySize || len(nonce) != PairingNonceSize {
				t.Fatal("invalid reveal accepted")
			}
		}
		if r.private != nil {
			t.Fatal("reveal did not consume responder")
		}
	})
}

func FuzzAuthenticatedPairingPayload(f *testing.F) {
	wk, err := workspaceKeyFromMaterial(KeyID{1}, 1, bytes.Repeat([]byte{0x42}, KeySize))
	if err != nil {
		f.Fatal(err)
	}
	defer wk.Destroy()
	kr, err := NewKeyring(wk)
	if err != nil {
		f.Fatal(err)
	}
	if err := kr.AddPasscodeWrap(wk, "seed", fastDeriver()); err != nil {
		f.Fatal(err)
	}
	blob, err := kr.Marshal(wk)
	if err != nil {
		f.Fatal(err)
	}
	var version [4]byte
	binary.BigEndian.PutUint32(version[:], 1)
	validPayload := joinLengthPrefixed(wk.keyID[:], version[:], wk.material)
	f.Add(validPayload)
	f.Add([]byte{})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 4096 {
			return
		}
		sender, receiver := pairingExchange(t, PairingInfo(LocalScope("a"), "w"))
		confirmPair(t, sender, receiver)
		enc, ct, err := SealForDevice(sender.recipient, payload, sender.info)
		if err != nil {
			t.Fatal(err)
		}
		// Use a valid transcript MAC to reach payload parsing/key checks.
		tag := sender.transferMAC(enc, ct, blob)
		got, err := OpenWorkspaceKeyOnDevice(receiver, enc, ct, tag, blob, 1)
		if err == nil {
			sameKey(t, wk, got)
			if !bytes.Equal(payload, validPayload) {
				t.Fatal("accepted a different key payload")
			}
		} else if got != nil {
			t.Fatal("partial key returned on failure")
		}
		if receiver.macKey != nil {
			t.Fatal("transfer did not consume receiver")
		}
		if _, err := OpenWorkspaceKeyOnDevice(receiver, enc, ct, tag, blob, 1); !errors.Is(err, ErrAuthenticationFailed) {
			t.Fatalf("replay accepted: %v", err)
		}
	})
}
