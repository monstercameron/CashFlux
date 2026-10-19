// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fastDeriver is a deterministic stand-in for PBKDF2 so tests do not pay
// 600,000 iterations per case. One test exercises the real deriver.
func fastDeriver() PBKDF2Deriver {
	return PBKDF2Func(func(password string, salt []byte, iterations, keyLen int) ([]byte, error) {
		var out []byte
		for counter := byte(0); len(out) < keyLen; counter++ {
			h := sha256.New()
			fmt.Fprintf(h, "test-pbkdf2\x00%s\x00%d\x00%d", password, iterations, counter)
			h.Write(salt)
			out = h.Sum(out)
		}
		return out[:keyLen], nil
	})
}

func mustKey(t *testing.T) *WorkspaceKey {
	t.Helper()
	wk, err := NewWorkspaceKey()
	if err != nil {
		t.Fatalf("NewWorkspaceKey: %v", err)
	}
	t.Cleanup(wk.Destroy)
	return wk
}

// fullKeyring builds a keyring with one wrap of every kind for wk.
type fullFixture struct {
	wk           *WorkspaceKey
	kr           *Keyring
	passcode     string
	recoveryCode string
	credID       []byte
	prfSalt      []byte
	prf          []byte
	deviceID     string
	device       Wrapper
	deviceKey    []byte
}

func mustFullKeyring(t *testing.T) fullFixture {
	t.Helper()
	wk := mustKey(t)
	kr, err := NewKeyring(wk)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	f := fullFixture{
		wk:        wk,
		kr:        kr,
		passcode:  "correct horse battery staple",
		credID:    []byte("credential-1"),
		prfSalt:   bytes.Repeat([]byte{0xAB}, 32),
		prf:       bytes.Repeat([]byte{0xCD}, PRFOutputSize),
		deviceID:  "laptop",
		deviceKey: bytes.Repeat([]byte{0xEF}, KeySize),
	}
	f.device = AESGCMWrapper{Key: f.deviceKey}
	if err := kr.AddPasscodeWrap(wk, f.passcode, fastDeriver()); err != nil {
		t.Fatalf("AddPasscodeWrap: %v", err)
	}
	code, err := kr.AddRecoveryWrap(wk)
	if err != nil {
		t.Fatalf("AddRecoveryWrap: %v", err)
	}
	f.recoveryCode = code
	if err := kr.AddPasskeyWrap(wk, f.credID, f.prfSalt, f.prf); err != nil {
		t.Fatalf("AddPasskeyWrap: %v", err)
	}
	if err := kr.AddDeviceWrap(wk, f.deviceID, f.device); err != nil {
		t.Fatalf("AddDeviceWrap: %v", err)
	}
	return f
}

func sameKey(t *testing.T, want, got *WorkspaceKey) {
	t.Helper()
	if got == nil {
		t.Fatal("unwrapped key is nil")
	}
	defer got.Destroy()
	if got.KeyID() != want.KeyID() || got.Version() != want.Version() {
		t.Errorf("identity mismatch: got (%s, v%d), want (%s, v%d)", got.KeyID(), got.Version(), want.KeyID(), want.Version())
	}
	if !bytes.Equal(got.Material(), want.Material()) {
		t.Error("unwrapped material differs from the original WDK")
	}
}

func TestWrapUnwrapRoundTrips(t *testing.T) {
	f := mustFullKeyring(t)
	passkeyID := f.kr.Current().Wraps[0].identity // resolved per-case below instead
	_ = passkeyID

	passkeyEntryID := ""
	for _, w := range f.kr.Current().Wraps {
		if w.Kind == WrapPasskey {
			_, passkeyEntryID = w.identity()
		}
	}

	tests := []struct {
		name   string
		kind   WrapKind
		id     string
		secret UnwrapSecret
	}{
		{"passcode", WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()}},
		{"recovery by code", WrapRecovery, "", UnwrapSecret{RecoveryCode: f.recoveryCode}},
		{"recovery lowercased without hyphens", WrapRecovery, "", UnwrapSecret{RecoveryCode: lowerAndStrip(f.recoveryCode)}},
		{"passkey by entry id", WrapPasskey, passkeyEntryID, UnwrapSecret{PRFOutput: f.prf}},
		{"passkey by credential id", WrapPasskey, "no-such-id", UnwrapSecret{PRFOutput: f.prf, CredentialID: f.credID}},
		{"device", WrapDevice, f.deviceID, UnwrapSecret{Device: f.device}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.kr.Unwrap(tc.kind, tc.id, tc.secret)
			if err != nil {
				t.Fatalf("Unwrap: %v", err)
			}
			sameKey(t, f.wk, got)
		})
	}
}

func lowerAndStrip(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}

func TestWrongSecretsFailAndNeverMutate(t *testing.T) {
	f := mustFullKeyring(t)
	before, err := f.kr.Marshal()
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}

	otherCode, otherSecret, err := GenerateRecoveryCode()
	if err != nil {
		t.Fatalf("GenerateRecoveryCode: %v", err)
	}
	defer clearBytes(otherSecret)

	passkeyEntryID := ""
	for _, w := range f.kr.Current().Wraps {
		if w.Kind == WrapPasskey {
			_, passkeyEntryID = w.identity()
		}
	}

	tests := []struct {
		name   string
		kind   WrapKind
		id     string
		secret UnwrapSecret
	}{
		{"wrong passcode", WrapPasscode, "", UnwrapSecret{Passcode: "wrong", Deriver: fastDeriver()}},
		{"empty passcode", WrapPasscode, "", UnwrapSecret{Passcode: "", Deriver: fastDeriver()}},
		{"wrong recovery code (valid but different)", WrapRecovery, "", UnwrapSecret{RecoveryCode: otherCode}},
		{"wrong recovery raw secret", WrapRecovery, "", UnwrapSecret{RecoverySecret: bytes.Repeat([]byte{1}, RecoverySecretSize)}},
		{"malformed recovery code", WrapRecovery, "", UnwrapSecret{RecoveryCode: "NOT-A-CODE"}},
		{"wrong PRF output", WrapPasskey, passkeyEntryID, UnwrapSecret{PRFOutput: bytes.Repeat([]byte{7}, PRFOutputSize)}},
		{"short PRF output", WrapPasskey, passkeyEntryID, UnwrapSecret{PRFOutput: []byte{1, 2, 3}}},
		{"mismatched credential", WrapPasskey, passkeyEntryID, UnwrapSecret{PRFOutput: f.prf, CredentialID: []byte("someone-elses")}},
		{"wrong device key", WrapDevice, f.deviceID, UnwrapSecret{Device: AESGCMWrapper{Key: bytes.Repeat([]byte{9}, KeySize)}}},
		{"missing device wrapper", WrapDevice, f.deviceID, UnwrapSecret{}},
		{"unknown entry id", WrapDevice, "tablet", UnwrapSecret{Device: f.device}},
		{"unknown version", WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()}}, // version overridden below
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got *WorkspaceKey
			var err error
			if tc.name == "unknown version" {
				got, err = f.kr.UnwrapVersion(99, tc.kind, tc.id, tc.secret)
			} else {
				got, err = f.kr.Unwrap(tc.kind, tc.id, tc.secret)
			}
			if err == nil {
				got.Destroy()
				t.Fatal("Unwrap succeeded with a wrong secret")
			}
		})
	}

	after, err := f.kr.Marshal()
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("keyring changed after failed unwrap attempts")
	}

	// And the right secrets still work afterwards.
	got, err := f.kr.Unwrap(WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()})
	if err != nil {
		t.Fatalf("Unwrap with right passcode after failures: %v", err)
	}
	sameKey(t, f.wk, got)
}

func TestWrongPasscodeIsUnwrapError(t *testing.T) {
	f := mustFullKeyring(t)
	_, err := f.kr.Unwrap(WrapPasscode, "", UnwrapSecret{Passcode: "nope", Deriver: fastDeriver()})
	if !IsUnwrapError(err) {
		t.Errorf("wrong passcode error = %v, want an unwrap/authentication error", err)
	}
	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Errorf("wrong passcode error does not carry *AuthenticationError: %v", err)
	}
}

func TestRewrapPasscodeKeepsWDKBytes(t *testing.T) {
	f := mustFullKeyring(t)
	beforeMaterial := f.wk.Material()
	dataKeyBefore, err := f.wk.DataKey()
	if err != nil {
		t.Fatalf("DataKey: %v", err)
	}
	aad := AAD(LocalScope("acct"), "ws-1", "change-1", f.wk.Version())
	sealed, err := Seal(dataKeyBefore, aad, []byte("ledger contents"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if err := f.kr.RewrapPasscode(f.wk, "a brand new passcode", fastDeriver()); err != nil {
		t.Fatalf("RewrapPasscode: %v", err)
	}
	if !bytes.Equal(beforeMaterial, f.wk.Material()) {
		t.Fatal("WDK material changed during rewrap")
	}
	if _, err := f.kr.Unwrap(WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()}); err == nil {
		t.Fatal("old passcode still unwraps after rewrap")
	}
	got, err := f.kr.Unwrap(WrapPasscode, "", UnwrapSecret{Passcode: "a brand new passcode", Deriver: fastDeriver()})
	if err != nil {
		t.Fatalf("Unwrap with new passcode: %v", err)
	}
	sameKey(t, f.wk, got)

	// Ciphertext sealed before the change still opens: nothing re-encrypted.
	dataKeyAfter, err := f.wk.DataKey()
	if err != nil {
		t.Fatalf("DataKey after: %v", err)
	}
	if !bytes.Equal(dataKeyBefore, dataKeyAfter) {
		t.Fatal("data subkey changed during rewrap")
	}
	plain, err := Open(dataKeyAfter, aad, sealed)
	if err != nil {
		t.Fatalf("Open after rewrap: %v", err)
	}
	if string(plain) != "ledger contents" {
		t.Errorf("plaintext = %q", plain)
	}
}

func TestRotateKeepsOldVersionsOpenable(t *testing.T) {
	f := mustFullKeyring(t)

	oldDataKey, err := f.wk.DataKey()
	if err != nil {
		t.Fatalf("DataKey: %v", err)
	}
	oldAAD := AAD(LocalScope("acct"), "ws-1", "change-old", f.wk.Version())
	sealedOld, err := Seal(oldDataKey, oldAAD, []byte("sealed under v1"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	var newCode string
	next, err := f.kr.RotateRetainingWraps(func(kr *Keyring, wk *WorkspaceKey) error {
		if err := kr.AddPasscodeWrap(wk, f.passcode, fastDeriver()); err != nil {
			return err
		}
		code, err := kr.AddRecoveryWrap(wk)
		newCode = code
		return err
	})
	if err != nil {
		t.Fatalf("RotateRetainingWraps: %v", err)
	}
	defer next.Destroy()

	if next.Version() != 2 || f.kr.CurrentVersion != 2 {
		t.Fatalf("after rotate: next v%d, current v%d, want 2/2", next.Version(), f.kr.CurrentVersion)
	}
	if bytes.Equal(next.Material(), f.wk.Material()) {
		t.Fatal("rotated key reuses the old material")
	}
	if err := f.kr.Validate(); err != nil {
		t.Fatalf("Validate after rotate: %v", err)
	}

	// Old version still unwraps and still opens its ciphertext.
	old, err := f.kr.UnwrapVersion(1, WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()})
	if err != nil {
		t.Fatalf("UnwrapVersion(1): %v", err)
	}
	sameKey(t, f.wk, old)

	unwrappedOld, err := f.kr.UnwrapVersion(1, WrapRecovery, "", UnwrapSecret{RecoveryCode: f.recoveryCode})
	if err != nil {
		t.Fatalf("UnwrapVersion(1, recovery): %v", err)
	}
	dk, err := unwrappedOld.DataKey()
	if err != nil {
		t.Fatalf("DataKey old: %v", err)
	}
	unwrappedOld.Destroy()
	if plain, err := Open(dk, oldAAD, sealedOld); err != nil || string(plain) != "sealed under v1" {
		t.Errorf("Open old ciphertext = %q, %v", plain, err)
	}

	// New version unwraps with its own (rotated) recovery code, not the old.
	gotNew, err := f.kr.Unwrap(WrapRecovery, "", UnwrapSecret{RecoveryCode: newCode})
	if err != nil {
		t.Fatalf("Unwrap new recovery: %v", err)
	}
	sameKey(t, next, gotNew)
	if _, err := f.kr.Unwrap(WrapRecovery, "", UnwrapSecret{RecoveryCode: f.recoveryCode}); err == nil {
		t.Fatal("old recovery code unwraps the rotated key")
	}

	// The new key cannot open v1 ciphertext (different subkey + AAD version).
	newDataKey, err := next.DataKey()
	if err != nil {
		t.Fatalf("DataKey new: %v", err)
	}
	if _, err := Open(newDataKey, oldAAD, sealedOld); err == nil {
		t.Fatal("rotated key opened v1 ciphertext")
	}
}

func TestRotateRetainingWrapsRollsBackOnFailure(t *testing.T) {
	f := mustFullKeyring(t)
	boom := errors.New("boom")
	if _, err := f.kr.RotateRetainingWraps(func(*Keyring, *WorkspaceKey) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped boom", err)
	}
	if f.kr.CurrentVersion != 1 || len(f.kr.Keys) != 1 {
		t.Fatalf("rollback failed: current v%d, %d versions", f.kr.CurrentVersion, len(f.kr.Keys))
	}
	if _, err := f.kr.RotateRetainingWraps(func(*Keyring, *WorkspaceKey) error { return nil }); err == nil {
		t.Fatal("rotate with no wraps installed succeeded")
	}
	if f.kr.CurrentVersion != 1 || len(f.kr.Keys) != 1 {
		t.Fatalf("rollback after empty wraps failed: current v%d, %d versions", f.kr.CurrentVersion, len(f.kr.Keys))
	}
}

func TestAddAndRemoveWrap(t *testing.T) {
	wk := mustKey(t)
	kr, err := NewKeyring(wk)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	if err := kr.AddPasscodeWrap(wk, "pw-one", fastDeriver()); err != nil {
		t.Fatalf("AddPasscodeWrap: %v", err)
	}
	// Duplicate kind+id is rejected; Rewrap is the way to replace it.
	if err := kr.AddPasscodeWrap(wk, "pw-two", fastDeriver()); !errors.Is(err, ErrDuplicateWrap) {
		t.Fatalf("duplicate passcode wrap err = %v, want ErrDuplicateWrap", err)
	}
	// The last wrap of the current version cannot be removed without force.
	if err := kr.RemoveWrap(wk.Version(), WrapPasscode, "", false); err == nil {
		t.Fatal("removed the last wrap without force")
	}
	if _, err := kr.AddRecoveryWrap(wk); err != nil {
		t.Fatalf("AddRecoveryWrap: %v", err)
	}
	if err := kr.RemoveWrap(wk.Version(), WrapPasscode, "", false); err != nil {
		t.Fatalf("RemoveWrap passcode: %v", err)
	}
	if _, err := kr.Unwrap(WrapPasscode, "", UnwrapSecret{Passcode: "pw-one", Deriver: fastDeriver()}); !errors.Is(err, ErrWrapNotFound) {
		t.Fatalf("unwrap removed entry err = %v, want ErrWrapNotFound", err)
	}
	if err := kr.RemoveWrap(wk.Version(), WrapPasscode, "", true); !errors.Is(err, ErrWrapNotFound) {
		t.Fatalf("remove missing entry err = %v, want ErrWrapNotFound", err)
	}
	if err := kr.RemoveWrap(77, WrapRecovery, "", true); !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("remove from missing version err = %v, want ErrVersionNotFound", err)
	}
}

func TestWrapRejectsEmptyPasscodeAndDestroyedKey(t *testing.T) {
	wk := mustKey(t)
	if _, err := WrapWithPasscode(wk, "", fastDeriver()); !errors.Is(err, ErrInvalidSecret) {
		t.Errorf("empty passcode err = %v, want ErrInvalidSecret", err)
	}
	dead, err := NewWorkspaceKey()
	if err != nil {
		t.Fatalf("NewWorkspaceKey: %v", err)
	}
	dead.Destroy()
	if _, err := WrapWithPasscode(dead, "pw", fastDeriver()); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("destroyed key err = %v, want ErrInvalidKey", err)
	}
	if _, err := dead.DeriveSubkey(SubkeyData); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("destroyed subkey err = %v, want ErrInvalidKey", err)
	}
	if dead.Material() != nil {
		t.Error("destroyed key still exposes material")
	}
}

func TestWrapEntryMovedToAnotherVersionFails(t *testing.T) {
	f := mustFullKeyring(t)
	// A second workspace key with the same version number but a different
	// KeyID: copying f's passcode entry into its keyring must not unwrap,
	// because the wrap AAD binds the original KeyID.
	other := mustKey(t)
	kr2, err := NewKeyring(other)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	var stolen WrapEntry
	for _, w := range f.kr.Current().Wraps {
		if w.Kind == WrapPasscode {
			stolen = w.Clone()
		}
	}
	kr2.Keys[0].Wraps = []WrapEntry{stolen}
	if _, err := kr2.Unwrap(WrapPasscode, "", UnwrapSecret{Passcode: f.passcode, Deriver: fastDeriver()}); err == nil {
		t.Fatal("wrap entry transplanted to another key id unwrapped")
	}
}

func TestMarshalParseRoundTrip(t *testing.T) {
	f := mustFullKeyring(t)
	blob, err := f.kr.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	blob2, err := f.kr.Marshal()
	if err != nil {
		t.Fatalf("Marshal again: %v", err)
	}
	if !bytes.Equal(blob, blob2) {
		t.Fatal("Marshal is not canonical (two calls differ)")
	}
	parsed, err := Parse(blob)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	reblob, err := parsed.Marshal()
	if err != nil {
		t.Fatalf("Marshal parsed: %v", err)
	}
	if !bytes.Equal(blob, reblob) {
		t.Fatal("Parse→Marshal does not reproduce the canonical blob")
	}
	got, err := parsed.Unwrap(WrapRecovery, "", UnwrapSecret{RecoveryCode: f.recoveryCode})
	if err != nil {
		t.Fatalf("Unwrap from parsed keyring: %v", err)
	}
	sameKey(t, f.wk, got)

	// JSON carries the documented field names.
	if !bytes.Contains(blob, []byte(`"formatVersion":1`)) || !bytes.Contains(blob, []byte(`"currentVersion":1`)) {
		t.Errorf("blob missing version fields: %s", blob)
	}
}

func TestParseRejectsMalformedBlobs(t *testing.T) {
	f := mustFullKeyring(t)
	good, err := f.kr.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// mutate decodes the good keyring, applies fn, and re-encodes with the
	// plain JSON encoder (bypassing Marshal's validation).
	mutate := func(fn func(*Keyring)) []byte {
		kr := f.kr.Clone()
		fn(kr)
		raw, err := json.Marshal(kr)
		if err != nil {
			t.Fatalf("json.Marshal mutated: %v", err)
		}
		return raw
	}

	tests := []struct {
		name string
		blob []byte
		want error
	}{
		{"empty", nil, ErrInvalidKeyring},
		{"not json", []byte("hello"), ErrInvalidKeyring},
		{"trailing data", append(cloneBytes(good), []byte(" {}")...), ErrInvalidKeyring},
		{"unknown field", bytes.Replace(good, []byte(`{"formatVersion"`), []byte(`{"surprise":true,"formatVersion"`), 1), ErrInvalidKeyring},
		{"duplicate field", bytes.Replace(good, []byte(`"formatVersion":1`), []byte(`"formatVersion":1,"formatVersion":1`), 1), ErrInvalidKeyring},
		{"noncanonical whitespace", append([]byte(" "), good...), ErrInvalidKeyring},
		{"unsupported format", mutate(func(kr *Keyring) { kr.FormatVersion = 99 }), ErrUnsupportedFormat},
		{"no keys", mutate(func(kr *Keyring) { kr.Keys = nil }), ErrInvalidKeyring},
		{"zero version", mutate(func(kr *Keyring) { kr.Keys[0].Version = 0 }), ErrInvalidKeyring},
		{"zero key id", mutate(func(kr *Keyring) { kr.Keys[0].KeyID = KeyID{} }), ErrInvalidKeyring},
		{"current missing", mutate(func(kr *Keyring) { kr.CurrentVersion = 42 }), ErrInvalidKeyring},
		{"current has no wraps", mutate(func(kr *Keyring) { kr.Keys[0].Wraps = nil }), ErrInvalidKeyring},
		{"duplicate version", mutate(func(kr *Keyring) { kr.Keys = append(kr.Keys, kr.Keys[0].Clone()) }), ErrInvalidKeyring},
		{"duplicate wrap", mutate(func(kr *Keyring) {
			kr.Keys[0].Wraps = append(kr.Keys[0].Wraps, kr.Keys[0].Wraps[0].Clone())
		}), ErrInvalidKeyring},
		{"unknown wrap kind", mutate(func(kr *Keyring) { kr.Keys[0].Wraps[0].Kind = "magic" }), ErrInvalidKeyring},
		{"wrong iterations", mutate(func(kr *Keyring) {
			for i := range kr.Keys[0].Wraps {
				if kr.Keys[0].Wraps[i].Kind == WrapPasscode {
					kr.Keys[0].Wraps[i].Iterations = 3
				}
			}
		}), ErrInvalidKeyring},
		{"short salt", mutate(func(kr *Keyring) {
			for i := range kr.Keys[0].Wraps {
				if kr.Keys[0].Wraps[i].Kind == WrapPasscode {
					kr.Keys[0].Wraps[i].Salt = []byte{1}
				}
			}
		}), ErrInvalidKeyring},
		{"truncated wrapped key", mutate(func(kr *Keyring) {
			for i := range kr.Keys[0].Wraps {
				if kr.Keys[0].Wraps[i].Kind == WrapRecovery {
					kr.Keys[0].Wraps[i].Wrapped = kr.Keys[0].Wraps[i].Wrapped[:10]
				}
			}
		}), ErrInvalidKeyring},
		{"oversized", make([]byte, MaxKeyringSize+1), ErrInvalidKeyring},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.blob); !errors.Is(err, tc.want) {
				t.Fatalf("Parse err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseBoundsJSONStructure(t *testing.T) {
	tooDeep := []byte(`{"unknown":` + strings.Repeat("[", MaxKeyringJSONDepth+1) + "0" + strings.Repeat("]", MaxKeyringJSONDepth+1) + `}`)
	tooManyKeys := []byte(`{"keys":[` + strings.TrimSuffix(strings.Repeat("{},", MaxKeyringVersions+1), ",") + `]}`)
	tooManyWraps := []byte(`{"keys":[{"wraps":[` + strings.TrimSuffix(strings.Repeat("{},", MaxWrapsPerVersion+1), ",") + `]}]}`)
	tooManyFields := []byte(`{"unknown":{` + strings.TrimSuffix(strings.Repeat(`"x":0,`, MaxFieldsPerObject), ",") + `,"extra":0}}`)

	tests := []struct {
		name string
		data []byte
	}{
		{"deep JSON", tooDeep},
		{"too many key versions", tooManyKeys},
		{"too many wraps", tooManyWraps},
		{"too many object fields", tooManyFields},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			started := time.Now()
			if _, err := Parse(tc.data); err == nil {
				t.Fatal("Parse accepted a blob beyond a structural limit")
			}
			if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
				t.Fatalf("Parse took %s for a bounded rejection", elapsed)
			}
		})
	}
}

func TestSubkeysAreDistinctAndStable(t *testing.T) {
	wk := mustKey(t)
	seen := map[string]string{}
	for _, info := range SubkeyInfos() {
		key, err := wk.DeriveSubkey(info)
		if err != nil {
			t.Fatalf("DeriveSubkey(%q): %v", info, err)
		}
		if len(key) != KeySize {
			t.Fatalf("subkey %q is %d bytes", info, len(key))
		}
		if other, dup := seen[string(key)]; dup {
			t.Errorf("subkeys %q and %q are identical", info, other)
		}
		seen[string(key)] = info
		again, err := wk.DeriveSubkey(info)
		if err != nil || !bytes.Equal(key, again) {
			t.Errorf("subkey %q is not deterministic", info)
		}
		if bytes.Equal(key, wk.Material()) {
			t.Errorf("subkey %q equals the WDK itself", info)
		}
	}
	// The named helpers agree with DeriveSubkey.
	pairs := []struct {
		info string
		fn   func() ([]byte, error)
	}{
		{SubkeyData, wk.DataKey},
		{SubkeyAttachments, wk.AttachmentsKey},
		{SubkeyAttachMAC, wk.AttachMACKey},
		{SubkeyVault, wk.VaultKey},
	}
	for _, p := range pairs {
		want, _ := wk.DeriveSubkey(p.info)
		got, err := p.fn()
		if err != nil || !bytes.Equal(want, got) {
			t.Errorf("helper for %q disagrees with DeriveSubkey", p.info)
		}
	}
	// A different WDK yields different subkeys.
	other := mustKey(t)
	a, _ := wk.DataKey()
	b, _ := other.DataKey()
	if bytes.Equal(a, b) {
		t.Error("different WDKs produced the same data subkey")
	}
}

// TestRealPBKDF2RoundTrip pays the real 600k-iteration cost exactly twice
// (wrap + unwrap) to prove the standard-library path end to end.
func TestRealPBKDF2RoundTrip(t *testing.T) {
	wk := mustKey(t)
	kr, err := NewKeyring(wk)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	if err := kr.AddPasscodeWrap(wk, "real passcode", nil); err != nil {
		t.Fatalf("AddPasscodeWrap: %v", err)
	}
	entry := kr.Current().findWrap(WrapPasscode, "")
	if entry.Iterations != PasscodeIterations || len(entry.Salt) != PasscodeSaltSize {
		t.Fatalf("stored entry has iterations=%d salt=%d bytes", entry.Iterations, len(entry.Salt))
	}
	got, err := kr.Unwrap(WrapPasscode, "", UnwrapSecret{Passcode: "real passcode"})
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	sameKey(t, wk, got)
}
