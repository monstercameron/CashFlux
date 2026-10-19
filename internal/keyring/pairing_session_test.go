// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"crypto/ecdh"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"
)

func pairingExchange(t *testing.T, info []byte) (*PairingSession, *PairingSession) {
	t.Helper()
	request, err := NewPairingRequest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(request.Destroy)
	commitment := request.Commitment()
	if len(commitment) != sha256.Size {
		t.Fatal("missing commitment")
	}
	responder, err := NewPairingChallenge(commitment, info)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(responder.Destroy)
	reveal, receiver, err := request.Reveal(responder.Challenge(), info)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := responder.Finish(reveal)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sender.Destroy)
	t.Cleanup(receiver.Destroy)
	if sender.ConfirmationCode() != receiver.ConfirmationCode() || len(sender.ConfirmationCode()) != 8 {
		t.Fatal("honest pairing codes differ or are not eight digits")
	}
	if request.Commitment() != nil || responder.Challenge().PublicKey != nil {
		t.Fatal("consumed handshake state remains usable")
	}
	return sender, receiver
}

func confirmPair(t *testing.T, sender, receiver *PairingSession) {
	t.Helper()
	if err := sender.Confirm(receiver.ConfirmationCode()); err != nil {
		t.Fatal(err)
	}
	if err := receiver.Confirm(sender.ConfirmationCode()); err != nil {
		t.Fatal(err)
	}
}

func signedFixture(t *testing.T) (*WorkspaceKey, []byte) {
	t.Helper()
	f := mustFullKeyring(t)
	blob, err := f.kr.Marshal(f.wk)
	if err != nil {
		t.Fatal(err)
	}
	return f.wk, blob
}

func TestConfirmedWorkspacePairingRoundTrip(t *testing.T) {
	wk, blob := signedFixture(t)
	sender, receiver := pairingExchange(t, PairingInfo(LocalScope("account"), "workspace"))
	confirmPair(t, sender, receiver)
	enc, ciphertext, tag, err := SealWorkspaceKeyForDevice(sender, wk, blob, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenWorkspaceKeyOnDevice(receiver, enc, ciphertext, tag, blob, 1)
	if err != nil {
		t.Fatal(err)
	}
	sameKey(t, wk, got)
	if _, _, _, err := SealWorkspaceKeyForDevice(sender, wk, blob, 1); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("reused sender: %v", err)
	}
	if _, err := OpenWorkspaceKeyOnDevice(receiver, enc, ciphertext, tag, blob, 1); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("replayed transfer: %v", err)
	}
	if sender.private != nil || receiver.private != nil || sender.macKey != nil || receiver.macKey != nil {
		t.Fatal("consumed sessions retain secrets")
	}
}

func TestPairingRejectsUnconfirmedAndWrongRole(t *testing.T) {
	wk, blob := signedFixture(t)
	sender, receiver := pairingExchange(t, PairingInfo(LocalScope("account"), "workspace"))
	if _, _, _, err := SealWorkspaceKeyForDevice(sender, wk, blob, 0); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("unconfirmed seal: %v", err)
	}
	if _, err := OpenWorkspaceKeyOnDevice(receiver, nil, nil, nil, blob, 0); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("unconfirmed open: %v", err)
	}
	confirmPair(t, sender, receiver)
	if _, _, _, err := SealWorkspaceKeyForDevice(receiver, wk, blob, 0); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("receiver sealing: %v", err)
	}
	if _, err := OpenWorkspaceKeyOnDevice(sender, nil, nil, nil, blob, 0); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("sender opening: %v", err)
	}
	if err := sender.Confirm(receiver.ConfirmationCode()); err == nil {
		t.Fatal("confirmed twice")
	}
}

func TestPairingCommitmentRejectsSubstitutedReveal(t *testing.T) {
	for _, field := range []string{"public key", "nonce"} {
		t.Run(field, func(t *testing.T) {
			request, err := NewPairingRequest()
			if err != nil {
				t.Fatal(err)
			}
			info := PairingInfo(LocalScope("a"), "w")
			responder, err := NewPairingChallenge(request.Commitment(), info)
			if err != nil {
				t.Fatal(err)
			}
			reveal, receiver, err := request.Reveal(responder.Challenge(), info)
			if err != nil {
				t.Fatal(err)
			}
			defer receiver.Destroy()
			if field == "public key" {
				reveal.PublicKey[0] ^= 1
			} else {
				reveal.Nonce[0] ^= 1
			}
			if _, err := responder.Finish(reveal); !errors.Is(err, ErrAuthenticationFailed) {
				t.Fatalf("substituted reveal accepted: %v", err)
			}
			if _, err := responder.Finish(reveal); err == nil {
				t.Fatal("reused failed challenge")
			}
		})
	}
}

// Fixed secrets make the MITM and transcript tests deterministic, without
// probabilistic assertions about randomly generated short codes.
func fixedRequest(t *testing.T, seed byte) *PairingRequest {
	t.Helper()
	private, err := ecdh.X25519().NewPrivateKey(bytes.Repeat([]byte{seed}, 32))
	if err != nil {
		t.Fatal(err)
	}
	nonce := bytes.Repeat([]byte{seed + 1}, PairingNonceSize)
	r := &PairingRequest{private: private, nonce: nonce, commitment: pairingCommitment(private.PublicKey().Bytes(), nonce)}
	t.Cleanup(r.Destroy)
	return r
}

func fixedResponder(t *testing.T, commitment, info []byte, seed byte) *PairingResponder {
	t.Helper()
	private, err := ecdh.X25519().NewPrivateKey(bytes.Repeat([]byte{seed}, 32))
	if err != nil {
		t.Fatal(err)
	}
	r := &PairingResponder{private: private, nonce: bytes.Repeat([]byte{seed + 1}, PairingNonceSize), commitment: cloneBytes(commitment), info: cloneBytes(info)}
	t.Cleanup(r.Destroy)
	return r
}

func TestPairingMITMSubstitutionCannotConfirm(t *testing.T) {
	info := PairingInfo(LocalScope("account"), "workspace")
	newDevice := fixedRequest(t, 1)
	attackerRequest := fixedRequest(t, 11)
	oldDevice := fixedResponder(t, attackerRequest.Commitment(), info, 21)
	attackerResponder := fixedResponder(t, newDevice.Commitment(), info, 31)
	// The attacker must fix both commitment and challenge before seeing B's
	// reveal, even though it can establish a valid ECDH secret on each leg.
	newReveal, newSession, err := newDevice.Reveal(attackerResponder.Challenge(), info)
	if err != nil {
		t.Fatal(err)
	}
	defer newSession.Destroy()
	attackerReveal, attackerReceiver, err := attackerRequest.Reveal(oldDevice.Challenge(), info)
	if err != nil {
		t.Fatal(err)
	}
	defer attackerReceiver.Destroy()
	oldSession, err := oldDevice.Finish(attackerReveal)
	if err != nil {
		t.Fatal(err)
	}
	defer oldSession.Destroy()
	attackerSender, err := attackerResponder.Finish(newReveal)
	if err != nil {
		t.Fatal(err)
	}
	defer attackerSender.Destroy()
	if oldSession.ConfirmationCode() == newSession.ConfirmationCode() {
		t.Fatal("MITM produced equal codes on honest devices")
	}
	if err := oldSession.Confirm(newSession.ConfirmationCode()); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("MITM confirmation: %v", err)
	}
	if oldSession.macKey != nil {
		t.Fatal("mismatched session still has a MAC key")
	}
}

func TestPairingTranscriptBindsEveryField(t *testing.T) {
	for _, field := range []string{"context", "commitment", "pubA", "pubB", "nonceA", "nonceB"} {
		t.Run(field, func(t *testing.T) {
			r := fixedRequest(t, 1)
			info := PairingInfo(LocalScope("a"), "w")
			a := fixedResponder(t, r.Commitment(), info, 3)
			challenge := a.Challenge()
			reveal := PairingReveal{PublicKey: r.private.PublicKey().Bytes(), Nonce: cloneBytes(r.nonce)}
			base, err := newPairingSession(r.private, challenge, reveal, r.commitment, info, false)
			if err != nil {
				t.Fatal(err)
			}
			defer base.Destroy()
			commitment := cloneBytes(r.commitment)
			switch field {
			case "context":
				info = PairingInfo(LocalScope("other"), "w")
			case "commitment":
				commitment[0] ^= 1
			case "pubA":
				challenge.PublicKey = fixedRequest(t, 9).private.PublicKey().Bytes()
			case "pubB":
				reveal.PublicKey = fixedRequest(t, 9).private.PublicKey().Bytes()
			case "nonceA":
				challenge.Nonce[0] ^= 1
			case "nonceB":
				reveal.Nonce[0] ^= 1
			}
			changed, err := newPairingSession(r.private, challenge, reveal, commitment, info, false)
			if err != nil {
				t.Fatal(err)
			}
			defer changed.Destroy()
			if base.code == changed.code || bytes.Equal(base.macKey, changed.macKey) {
				t.Fatalf("transcript does not bind %s", field)
			}
		})
	}
}

func TestPairingRejectsForgedAndTamperedTransfers(t *testing.T) {
	for _, field := range []string{"enc", "ciphertext", "MAC", "keyring", "third-party sender", "public transcript MAC"} {
		t.Run(field, func(t *testing.T) {
			wk, blob := signedFixture(t)
			sender, receiver := pairingExchange(t, PairingInfo(LocalScope("account"), "workspace"))
			confirmPair(t, sender, receiver)
			enc, ct, tag, err := SealWorkspaceKeyForDevice(sender, wk, blob, 0)
			if err != nil {
				t.Fatal(err)
			}
			switch field {
			case "enc":
				enc[0] ^= 1
			case "ciphertext":
				ct[0] ^= 1
			case "MAC":
				tag[0] ^= 1
			case "keyring":
				blob = append(cloneBytes(blob), ' ')
			case "third-party sender", "public transcript MAC":
				// A correctly sized ciphertext reaches sender-MAC verification;
				// this must not pass just because HPKE accepts any public sender.
				payload := bytes.Repeat([]byte{0x23}, 3*4+KeyIDSize+4+KeySize)
				enc, ct, err = SealForDevice(receiver.recipient, payload, receiver.info)
				if err != nil {
					t.Fatal(err)
				}
				if field == "public transcript MAC" {
					fake := *receiver
					fake.macKey = cloneBytes(receiver.transcript)
					tag = fake.transferMAC(enc, ct, blob)
				}
			}
			got, err := OpenWorkspaceKeyOnDevice(receiver, enc, ct, tag, blob, 0)
			if !errors.Is(err, ErrAuthenticationFailed) || got != nil {
				t.Fatalf("tampered transfer returned key=%v error=%v", got, err)
			}
			if receiver.macKey != nil {
				t.Fatal("failed transfer not consumed")
			}
		})
	}
}

func TestPairingChecksReceivedKeyAgainstKeyring(t *testing.T) {
	for _, field := range []string{"key ID", "version", "material", "rollback", "malformed payload"} {
		t.Run(field, func(t *testing.T) {
			wk, blob := signedFixture(t)
			sender, receiver := pairingExchange(t, PairingInfo(LocalScope("account"), "workspace"))
			confirmPair(t, sender, receiver)
			id, version, material := wk.KeyID().Bytes(), wk.Version(), wk.Material()
			defer clearBytes(material)
			floor := uint32(0)
			switch field {
			case "key ID":
				id[0] ^= 1
			case "version":
				version++
			case "material":
				material[0] ^= 1
			case "rollback":
				floor = 2
			}
			var v [4]byte
			binary.BigEndian.PutUint32(v[:], version)
			payload := joinLengthPrefixed(id, v[:], material)
			defer clearBytes(payload)
			if field == "malformed payload" {
				payload = bytes.Repeat([]byte{0xff}, len(payload))
			}
			enc, ct, err := SealForDevice(sender.recipient, payload, sender.info)
			if err != nil {
				t.Fatal(err)
			}
			tag := sender.transferMAC(enc, ct, blob)
			got, err := OpenWorkspaceKeyOnDevice(receiver, enc, ct, tag, blob, floor)
			if err == nil || got != nil {
				t.Fatalf("wrong key accepted: key=%v, error=%v", got, err)
			}
		})
	}
}

func TestPairingSenderRefusesInvalidKeyring(t *testing.T) {
	for _, field := range []string{"metadata", "wrong key", "rollback", "destroyed"} {
		t.Run(field, func(t *testing.T) {
			wk, blob := signedFixture(t)
			sender, receiver := pairingExchange(t, PairingInfo(LocalScope("a"), "w"))
			confirmPair(t, sender, receiver)
			floor := uint32(1)
			switch field {
			case "metadata":
				blob[len(blob)-3] ^= 1
			case "wrong key":
				wk = mustKey(t)
			case "rollback":
				floor = 2
			case "destroyed":
				wk.Destroy()
			}
			enc, ct, tag, err := SealWorkspaceKeyForDevice(sender, wk, blob, floor)
			if err == nil || enc != nil || ct != nil || tag != nil {
				t.Fatalf("invalid sender state returned data: %v", err)
			}
			if sender.macKey != nil {
				t.Fatal("failed sender attempt not consumed")
			}
		})
	}
}

func TestPairingContextsCannotBeSwapped(t *testing.T) {
	info := PairingInfo(LocalScope("a"), "w")
	for _, other := range [][]byte{
		PairingInfo(Scope{ServerID: "remote", AccountID: "a"}, "w"),
		PairingInfo(LocalScope("b"), "w"),
		PairingInfo(LocalScope("a"), "other"),
	} {
		request := fixedRequest(t, 1)
		responder := fixedResponder(t, request.Commitment(), info, 3)
		reveal, receiver, err := request.Reveal(responder.Challenge(), other)
		if err != nil {
			t.Fatal(err)
		}
		defer receiver.Destroy()
		sender, err := responder.Finish(reveal)
		if err != nil {
			t.Fatal(err)
		}
		defer sender.Destroy()
		if err := sender.Confirm(receiver.ConfirmationCode()); !errors.Is(err, ErrAuthenticationFailed) {
			t.Fatalf("cross-scope confirmation: %v", err)
		}
	}
}

func TestPairingMalformedHandshakeAndDefensiveCopies(t *testing.T) {
	info := PairingInfo(LocalScope("a"), "w")
	if _, err := NewPairingChallenge(nil, info); err == nil {
		t.Fatal("missing commitment accepted")
	}
	if _, err := NewPairingChallenge(make([]byte, 32), nil); err == nil {
		t.Fatal("missing context accepted")
	}
	request := fixedRequest(t, 1)
	commit := request.Commitment()
	commit[0] ^= 1
	if bytes.Equal(commit, request.Commitment()) {
		t.Fatal("Commitment aliases internal state")
	}
	responder := fixedResponder(t, request.Commitment(), info, 3)
	challenge := responder.Challenge()
	challenge.PublicKey[0] ^= 1
	challenge.Nonce[0] ^= 1
	if bytes.Equal(challenge.Nonce, responder.Challenge().Nonce) || bytes.Equal(challenge.PublicKey, responder.Challenge().PublicKey) {
		t.Fatal("Challenge aliases internal state")
	}
	for _, challenge := range []PairingChallenge{
		{PublicKey: []byte{1}, Nonce: make([]byte, 32)},
		{PublicKey: make([]byte, 32), Nonce: make([]byte, 32)}, // low-order X25519 point
		{PublicKey: responder.Challenge().PublicKey, Nonce: []byte{1}},
	} {
		r := fixedRequest(t, 1)
		if _, s, err := r.Reveal(challenge, info); err == nil || s != nil {
			t.Fatal("invalid challenge accepted")
		}
		if _, _, err := r.Reveal(responder.Challenge(), info); err == nil {
			t.Fatal("request reused after failure")
		}
	}
}

func TestPairingCommitmentGolden(t *testing.T) {
	// Locks the wire commitment domain and framing to an independently computed
	// SHA-256 value; no public-key-only confirmation API remains.
	got := pairingCommitment(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	const want = "c483ad2805dded179c16aa1119d04ef138accedb9543fe8c4f0cbebedb09b1d4"
	if hex.EncodeToString(got) != want {
		t.Fatalf("commitment = %x", got)
	}
}
