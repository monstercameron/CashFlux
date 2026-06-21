// Package cryptobox defines the on-disk envelope format for CashFlux's optional
// dataset encryption (backlog item C45). Encryption is derived from the user's
// passcode via PBKDF2 → AES-GCM; the key is never stored. The envelope is a
// thin binary prefix followed by JSON so callers can distinguish ciphertext from
// a legacy plaintext JSON dataset with a single bytes.HasPrefix check.
//
// Pure Go, no platform dependencies — the wasm/JS side performs the actual
// crypto.subtle calls; this package only marshals/parses the resulting envelope.
package cryptobox

import (
	"bytes"
	"encoding/json"
)

// EnvelopeMarker is the binary prefix written before the JSON envelope body.
// It is chosen to be invalid UTF-8 and to never appear at the start of a valid
// JSON object, so IsEnvelope is an O(4) prefix check with no ambiguity.
const EnvelopeMarker = "\x00cf1\x00"

// AlgAESGCM is the algorithm identifier stored in Envelope.Alg.
const AlgAESGCM = "AES-GCM-PBKDF2"

// PBKDF2Iterations is the iteration count used when deriving the AES key from
// the passcode. 600 000 matches OWASP's 2023 recommendation for PBKDF2-SHA-256.
const PBKDF2Iterations = 600_000

// CurrentVersion is the envelope format version. Increment if the structure
// changes in a breaking way.
const CurrentVersion = 1

// markerBytes is EnvelopeMarker as a byte slice, pre-computed for comparisons.
var markerBytes = []byte(EnvelopeMarker)

// Envelope is the persisted representation of an encrypted dataset.
//
//   - V      — envelope format version (CurrentVersion).
//   - Alg    — algorithm identifier (AlgAESGCM).
//   - Salt   — base64-encoded random salt used for PBKDF2 key derivation.
//   - IV     — base64-encoded random AES-GCM initialisation vector (12 bytes).
//   - Cipher — base64-encoded AES-GCM ciphertext (includes the 16-byte auth tag).
type Envelope struct {
	V      int    `json:"v"`
	Alg    string `json:"alg"`
	Salt   string `json:"salt"`
	IV     string `json:"iv"`
	Cipher string `json:"cipher"`
}

// Marshal serialises env to the on-disk form: EnvelopeMarker followed by the
// JSON encoding of the envelope fields. The result is always a valid byte slice
// that IsEnvelope will accept.
func Marshal(env Envelope) []byte {
	body, err := json.Marshal(env)
	if err != nil {
		// json.Marshal only fails for un-marshalable types (not the case here).
		return nil
	}
	out := make([]byte, len(markerBytes)+len(body))
	copy(out, markerBytes)
	copy(out[len(markerBytes):], body)
	return out
}

// Parse attempts to decode data (previously produced by Marshal) into an
// Envelope. Returns the envelope and true on success, or the zero value and
// false when data is not a valid envelope (wrong prefix, bad JSON, unknown
// version, or missing required fields).
func Parse(data []byte) (Envelope, bool) {
	if !bytes.HasPrefix(data, markerBytes) {
		return Envelope{}, false
	}
	body := data[len(markerBytes):]
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Envelope{}, false
	}
	if env.V != CurrentVersion || env.Alg != AlgAESGCM || env.Salt == "" || env.IV == "" || env.Cipher == "" {
		return Envelope{}, false
	}
	return env, true
}

// IsEnvelope reports whether data looks like a CashFlux encryption envelope
// (created by Marshal). Returns false for any data that starts with '{', which
// covers all legacy plaintext JSON datasets.
func IsEnvelope(data []byte) bool {
	return bytes.HasPrefix(data, markerBytes)
}
