// SPDX-License-Identifier: MIT

package cryptobox_test

import (
	"encoding/base64"
	"testing"

	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

// dummyEnvelope returns a well-formed Envelope for use in tests.
func dummyEnvelope() cryptobox.Envelope {
	return cryptobox.Envelope{
		V:      cryptobox.CurrentVersion,
		Alg:    cryptobox.AlgAESGCM,
		Salt:   base64.StdEncoding.EncodeToString([]byte("saltsaltsaltsalt")), // 16 bytes
		IV:     base64.StdEncoding.EncodeToString([]byte("iviviviviviv")),     // 12 bytes
		Cipher: base64.StdEncoding.EncodeToString([]byte("cipherciphercipher")),
	}
}

// TestMarshalParseRoundTrip checks that Marshal → Parse is lossless for a
// variety of envelope field values.
func TestMarshalParseRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   cryptobox.Envelope
	}{
		{
			name: "typical envelope",
			in:   dummyEnvelope(),
		},
		{
			name: "minimal valid envelope",
			in: cryptobox.Envelope{
				V:      cryptobox.CurrentVersion,
				Alg:    cryptobox.AlgAESGCM,
				Salt:   base64.StdEncoding.EncodeToString([]byte("s")),
				IV:     base64.StdEncoding.EncodeToString([]byte("i")),
				Cipher: base64.StdEncoding.EncodeToString([]byte("c")),
			},
		},
		{
			name: "large cipher field",
			in: cryptobox.Envelope{
				V:      cryptobox.CurrentVersion,
				Alg:    cryptobox.AlgAESGCM,
				Salt:   base64.StdEncoding.EncodeToString(make([]byte, 16)),
				IV:     base64.StdEncoding.EncodeToString(make([]byte, 12)),
				Cipher: base64.StdEncoding.EncodeToString(make([]byte, 4096)),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			marshalled := cryptobox.Marshal(tc.in)
			if len(marshalled) == 0 {
				t.Fatal("Marshal returned empty bytes")
			}
			got, ok := cryptobox.Parse(marshalled)
			if !ok {
				t.Fatal("Parse returned false for output of Marshal")
			}
			if got.V != tc.in.V {
				t.Errorf("V: got %d, want %d", got.V, tc.in.V)
			}
			if got.Alg != tc.in.Alg {
				t.Errorf("Alg: got %q, want %q", got.Alg, tc.in.Alg)
			}
			if got.Salt != tc.in.Salt {
				t.Errorf("Salt: got %q, want %q", got.Salt, tc.in.Salt)
			}
			if got.IV != tc.in.IV {
				t.Errorf("IV: got %q, want %q", got.IV, tc.in.IV)
			}
			if got.Cipher != tc.in.Cipher {
				t.Errorf("Cipher: got %q, want %q", got.Cipher, tc.in.Cipher)
			}
		})
	}
}

// TestParseRejectsInvalidInputs checks that Parse returns false for a variety
// of malformed or incomplete inputs.
func TestParseRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	validMarshalled := cryptobox.Marshal(dummyEnvelope())
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: []byte{}},
		{name: "plaintext JSON", data: []byte(`{"accounts":[]}`)},
		{name: "wrong prefix", data: append([]byte("\x01cf1\x00"), validMarshalled[5:]...)},
		{name: "marker only no JSON", data: []byte(cryptobox.EnvelopeMarker)},
		{name: "marker plus invalid JSON", data: append([]byte(cryptobox.EnvelopeMarker), []byte("not json")...)},
		{name: "wrong version", data: cryptobox.Marshal(cryptobox.Envelope{
			V: 99, Alg: cryptobox.AlgAESGCM,
			Salt: "a", IV: "b", Cipher: "c",
		})},
		{name: "wrong alg", data: cryptobox.Marshal(cryptobox.Envelope{
			V: cryptobox.CurrentVersion, Alg: "UNKNOWN",
			Salt: "a", IV: "b", Cipher: "c",
		})},
		{name: "missing salt", data: cryptobox.Marshal(cryptobox.Envelope{
			V: cryptobox.CurrentVersion, Alg: cryptobox.AlgAESGCM,
			IV: "b", Cipher: "c",
		})},
		{name: "missing iv", data: cryptobox.Marshal(cryptobox.Envelope{
			V: cryptobox.CurrentVersion, Alg: cryptobox.AlgAESGCM,
			Salt: "a", Cipher: "c",
		})},
		{name: "missing cipher", data: cryptobox.Marshal(cryptobox.Envelope{
			V: cryptobox.CurrentVersion, Alg: cryptobox.AlgAESGCM,
			Salt: "a", IV: "b",
		})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, ok := cryptobox.Parse(tc.data); ok {
				t.Errorf("Parse(%q) returned ok=true, want false", tc.data)
			}
		})
	}
}

// TestIsEnvelope checks the binary predicate across true and false cases.
func TestIsEnvelope(t *testing.T) {
	t.Parallel()
	validMarshalled := cryptobox.Marshal(dummyEnvelope())
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "valid envelope", data: validMarshalled, want: true},
		{name: "marker only", data: []byte(cryptobox.EnvelopeMarker), want: true},
		{name: "plaintext JSON object", data: []byte(`{"v":1}`), want: false},
		{name: "empty dataset", data: []byte(`{}`), want: false},
		{name: "empty bytes", data: []byte{}, want: false},
		{name: "null byte not marker", data: []byte("\x00"), want: false},
		{name: "partial marker", data: []byte("\x00cf"), want: false},
		{name: "legacy exported JSON", data: []byte(`{"accounts":[],"transactions":[]}`), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := cryptobox.IsEnvelope(tc.data); got != tc.want {
				t.Errorf("IsEnvelope(%q) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

// TestConstants verifies the exported constants have expected values, so any
// accidental change is caught by the test suite.
func TestConstants(t *testing.T) {
	t.Parallel()
	if cryptobox.AlgAESGCM != "AES-GCM-PBKDF2" {
		t.Errorf("AlgAESGCM = %q, want %q", cryptobox.AlgAESGCM, "AES-GCM-PBKDF2")
	}
	if cryptobox.PBKDF2Iterations != 600_000 {
		t.Errorf("PBKDF2Iterations = %d, want 600000", cryptobox.PBKDF2Iterations)
	}
	if cryptobox.CurrentVersion != 1 {
		t.Errorf("CurrentVersion = %d, want 1", cryptobox.CurrentVersion)
	}
	if cryptobox.EnvelopeMarker == "" {
		t.Error("EnvelopeMarker must not be empty")
	}
	if cryptobox.EnvelopeMarker[0] == '{' {
		t.Error("EnvelopeMarker must not start with '{' (would collide with plaintext JSON)")
	}
}

// TestMarshalStartsWithMarker verifies Marshal output always begins with the
// envelope marker (IsEnvelope and Parse both depend on this invariant).
func TestMarshalStartsWithMarker(t *testing.T) {
	t.Parallel()
	out := cryptobox.Marshal(dummyEnvelope())
	marker := []byte(cryptobox.EnvelopeMarker)
	if len(out) < len(marker) {
		t.Fatalf("Marshal output too short: %d bytes", len(out))
	}
	for i, b := range marker {
		if out[i] != b {
			t.Fatalf("Marshal output byte %d: got 0x%02x, want 0x%02x", i, out[i], b)
		}
	}
}
