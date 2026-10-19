// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMetadataMACRejectsEveryMutation(t *testing.T) {
	f := mustFullKeyring(t)
	oldBlob, err := f.kr.Marshal(f.wk)
	if err != nil {
		t.Fatal(err)
	}
	next, err := f.kr.RotateRetainingWraps(func(kr *Keyring, wk *WorkspaceKey) error {
		return kr.AddPasscodeWrap(wk, f.passcode, fastDeriver())
	})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Destroy()
	blob, err := f.kr.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := Parse(blob, next, 2)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Keyring)
	}{
		{"remove old version", func(kr *Keyring) { kr.Keys = kr.Keys[1:] }},
		{"old key ID", func(kr *Keyring) { kr.Keys[0].KeyID[0] ^= 1 }},
		{"current key ID", func(kr *Keyring) { kr.Keys[1].KeyID[0] ^= 1 }},
		{"key check", func(kr *Keyring) { kr.Keys[0].KeyCheck[0] ^= 1 }},
		{"current key check", func(kr *Keyring) { kr.Keys[1].KeyCheck[0] ^= 1 }},
		{"wrap identity", func(kr *Keyring) { kr.Keys[1].Wraps[0].ID = "substituted" }},
		{"remove wrap", func(kr *Keyring) { kr.Keys[0].Wraps = kr.Keys[0].Wraps[1:] }},
		{"wrap salt", func(kr *Keyring) { kr.Keys[1].Wraps[0].Salt[0] ^= 1 }},
		{"wrap nonce", func(kr *Keyring) { kr.Keys[1].Wraps[0].Nonce[0] ^= 1 }},
		{"wrap ciphertext", func(kr *Keyring) { kr.Keys[1].Wraps[0].Wrapped[0] ^= 1 }},
		{"MAC", func(kr *Keyring) { kr.MetadataMAC[0] ^= 1 }},
		{"missing MAC", func(kr *Keyring) { kr.MetadataMAC = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kr := verified.Clone()
			tc.mutate(kr)
			// Preserve canonical order so rejection tests authentication, rather
			// than merely detecting a noncanonical JSON encoding.
			modified, err := json.Marshal(kr.canonical())
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(modified, next, 2)
			if !errors.Is(err, ErrAuthenticationFailed) || got != nil {
				t.Fatalf("mutation returned keyring=%v, error=%v", got, err)
			}
		})
	}
	// A server can replay an old valid MAC; only the persisted caller floor
	// distinguishes an old authentic keyring from an allowed first use.
	if got, err := Parse(oldBlob, f.wk, 2); !errors.Is(err, ErrRollback) || got != nil {
		t.Fatalf("signed snapshot rollback: keyring=%v error=%v", got, err)
	}
	if _, err := Parse(oldBlob, f.wk, 0); err != nil {
		t.Fatalf("first use rejected: %v", err)
	}
	if got, err := Parse(blob, next, 3); !errors.Is(err, ErrRollback) || got != nil {
		t.Fatalf("floor beyond current: keyring=%v error=%v", got, err)
	}
	// Lowering current while retaining higher keys is forbidden even if a
	// caller accidentally attempts to sign that state itself.
	kr := verified.Clone()
	kr.CurrentVersion = 1
	if _, err := kr.Marshal(f.wk); !errors.Is(err, ErrRollback) {
		t.Fatalf("lowered current: %v", err)
	}
}

func TestKeyringUnlockAuthenticatesBeforeReturning(t *testing.T) {
	f := mustFullKeyring(t)
	blob, err := f.kr.Marshal(f.wk)
	if err != nil {
		t.Fatal(err)
	}
	kr, wk, err := Unlock(blob, 1, WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()})
	if err != nil {
		t.Fatal(err)
	}
	sameKey(t, f.wk, wk)
	if !bytes.Equal(kr.MetadataMAC, mustParse(t, blob, f.wk, 1).MetadataMAC) {
		t.Fatal("unlock changed metadata")
	}
	bad := kr.Clone()
	// A different wrap's identity is still structurally valid and does not
	// interfere with opening the passcode entry, but its metadata MAC fails.
	for i := range bad.Keys[0].Wraps {
		if bad.Keys[0].Wraps[i].Kind == WrapDevice {
			bad.Keys[0].Wraps[i].ID = "attacker"
		}
	}
	tampered, err := json.Marshal(bad.canonical())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		data   []byte
		floor  uint32
		secret string
		want   error
	}{
		{"tampered unrelated wrap", tampered, 1, f.passcode, ErrAuthenticationFailed},
		{"wrong secret", blob, 1, "wrong", ErrAuthenticationFailed},
		{"rollback", blob, 2, f.passcode, ErrRollback},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotKR, gotWK, err := Unlock(tc.data, tc.floor, WrapPasscode, "", UnwrapSecret{Passcode: tc.secret, Deriver: fastDeriver()})
			if !errors.Is(err, tc.want) || gotKR != nil || gotWK != nil {
				t.Fatalf("partial result: kr=%v wk=%v err=%v", gotKR, gotWK, err)
			}
		})
	}
}

func mustParse(t *testing.T, blob []byte, wk *WorkspaceKey, floor uint32) *Keyring {
	t.Helper()
	kr, err := Parse(blob, wk, floor)
	if err != nil {
		t.Fatal(err)
	}
	return kr
}

func TestMetadataRejectsWrongKeyIdentityAndMaterial(t *testing.T) {
	wk, blob := signedFixture(t)
	for _, field := range []string{"unrelated key", "same identity wrong material", "wrong version", "nil", "destroyed"} {
		t.Run(field, func(t *testing.T) {
			var candidate *WorkspaceKey
			switch field {
			case "unrelated key":
				candidate = mustKey(t)
			case "same identity wrong material":
				candidate = &WorkspaceKey{keyID: wk.keyID, version: wk.version, material: bytes.Repeat([]byte{0x17}, KeySize)}
			case "wrong version":
				candidate = &WorkspaceKey{keyID: wk.keyID, version: wk.version + 1, material: wk.Material()}
			case "destroyed":
				candidate = mustKey(t)
				candidate.Destroy()
			}
			if candidate != nil {
				defer candidate.Destroy()
			}
			if got, err := Parse(blob, candidate, 0); err == nil || got != nil {
				t.Fatalf("wrong key accepted: kr=%v err=%v", got, err)
			}
			if _, err := mustParse(t, blob, wk, 0).Marshal(candidate); err == nil {
				t.Fatal("Marshal accepted wrong signer")
			}
		})
	}
}

func TestMetadataCanonicalRoundTripAndCopies(t *testing.T) {
	f := mustFullKeyring(t)
	before := f.kr.Clone()
	blob, err := f.kr.Marshal(f.wk)
	if err != nil {
		t.Fatal(err)
	}
	if f.kr.MetadataMAC != nil || !bytes.Equal(before.Keys[0].KeyCheck, f.kr.Keys[0].KeyCheck) {
		t.Fatal("Marshal mutated receiver")
	}
	kr := mustParse(t, blob, f.wk, 1)
	kr.Keys[0].Wraps[0], kr.Keys[0].Wraps[1] = kr.Keys[0].Wraps[1], kr.Keys[0].Wraps[0]
	other, err := kr.Marshal(f.wk)
	if err != nil || !bytes.Equal(blob, other) {
		t.Fatalf("canonical ordering changed: %v", err)
	}
	copy := kr.Clone()
	copy.MetadataMAC[0] ^= 1
	copy.Keys[0].KeyCheck[0] ^= 1
	if bytes.Equal(copy.MetadataMAC, kr.MetadataMAC) || bytes.Equal(copy.Keys[0].KeyCheck, kr.Keys[0].KeyCheck) {
		t.Fatal("clone aliases authentication bytes")
	}
	if _, err := Parse(bytes.Replace(blob, []byte(`"formatVersion":2`), []byte(`"formatVersion":1`), 1), f.wk, 0); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("legacy unsigned format accepted: %v", err)
	}
	kr = mustParse(t, blob, f.wk, 1)
	kr.Keys[0].Wraps[0].ID = strings.Repeat("x", MaxKeyringSize)
	if _, err := kr.Marshal(f.wk); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("oversized canonical output: %v", err)
	}
}

func TestRotatedKeyringUnlockKeepsOldKeyChecks(t *testing.T) {
	f := mustFullKeyring(t)
	next, err := f.kr.RotateRetainingWraps(func(kr *Keyring, wk *WorkspaceKey) error {
		return kr.AddPasscodeWrap(wk, f.passcode, fastDeriver())
	})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Destroy()
	blob, err := f.kr.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	kr, got, err := Unlock(blob, 2, WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()})
	if err != nil {
		t.Fatal(err)
	}
	sameKey(t, next, got)
	old, err := kr.UnwrapVersion(1, WrapRecovery, "", UnwrapSecret{RecoveryCode: f.recoveryCode})
	if err != nil {
		t.Fatal(err)
	}
	sameKey(t, f.wk, old)
	kr.Keys[0].KeyCheck[0] ^= 1
	if got, err := kr.UnwrapVersion(1, WrapRecovery, "", UnwrapSecret{RecoveryCode: f.recoveryCode}); !errors.Is(err, ErrAuthenticationFailed) || got != nil {
		t.Fatalf("old key check ignored: key=%v err=%v", got, err)
	}
}
