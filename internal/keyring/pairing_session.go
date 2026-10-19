// SPDX-License-Identifier: MIT

package keyring

import (
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// PairingNonceSize is the entropy of each independent pairing nonce, in bytes.
const PairingNonceSize = 32

// PairingRequest holds the new device's private commitment state. It is
// single-use, must stay on that device, and must not be shared between goroutines.
// Send Commitment before receiving a challenge. Aborted sessions must be
// restarted with a new request, never with a reused commitment.
type PairingRequest struct {
	private    *ecdh.PrivateKey
	nonce      []byte
	commitment []byte
}

// PairingChallenge is the old device's response to a frozen commitment.
type PairingChallenge struct {
	// PublicKey is the old device's ephemeral X25519 public key (pubA).
	PublicKey []byte
	// Nonce is nonceA, revealed only after accepting the new commitment.
	Nonce []byte
}

// PairingReveal opens the new device's commitment, after the challenge is fixed.
type PairingReveal struct {
	// PublicKey is the new device's ephemeral X25519 public key (pubB).
	PublicKey []byte
	// Nonce is nonceB, the random secret used in the commitment.
	Nonce []byte
}

// PairingResponder holds the old device's private, single-use challenge state.
// It must not be shared between goroutines.
type PairingResponder struct {
	private                 *ecdh.PrivateKey
	nonce, commitment, info []byte
}

// PairingSession is a completed commitment exchange awaiting a human SAS
// comparison. Only Confirm enables a single transfer attempt. The MAC key is
// HKDF of the ephemeral ECDH secret bound to the transcript, never of the public
// SAS alone. Sessions are process-local, single-use and not goroutine-safe.
type PairingSession struct {
	private                             *ecdh.PrivateKey
	recipient, info, transcript, macKey []byte
	code                                string
	sender, confirmed                   bool
}

// NewPairingRequest generates pubB and nonceB and commits to both before nonceA
// is known. It deliberately produces no confirmation code yet.
func NewPairingRequest() (*PairingRequest, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("keyring: generate pairing key: %w", err)
	}
	nonce, err := randomBytes(PairingNonceSize)
	if err != nil {
		return nil, err
	}
	return &PairingRequest{private: private, nonce: nonce, commitment: pairingCommitment(private.PublicKey().Bytes(), nonce)}, nil
}

func pairingCommitment(pub, nonce []byte) []byte {
	sum := sha256.Sum256(joinLengthPrefixed([]byte("cfv2/pairing/commit/v2"), pub, nonce))
	return cloneBytes(sum[:])
}

// Commitment returns the 32-byte commitment to send first, without pubB/nonceB.
func (r *PairingRequest) Commitment() []byte {
	if r == nil || r.private == nil {
		return nil
	}
	return cloneBytes(r.commitment)
}

// Destroy discards the new device's pending pairing secrets. Go's ecdh private
// key cannot be zeroed; dropping its reference is best effort, as with HPKE.
func (r *PairingRequest) Destroy() {
	if r != nil {
		clearBytes(r.nonce)
		r.private, r.nonce, r.commitment = nil, nil, nil
	}
}

// NewPairingChallenge freezes a received commitment before generating pubA and
// revealing nonceA. info must be PairingInfo for the intended scope/workspace.
func NewPairingChallenge(commitment, info []byte) (*PairingResponder, error) {
	if len(commitment) != sha256.Size || len(info) == 0 {
		return nil, fmt.Errorf("%w: pairing needs a commitment and context", ErrInvalidKey)
	}
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("keyring: generate pairing responder: %w", err)
	}
	nonce, err := randomBytes(PairingNonceSize)
	if err != nil {
		return nil, err
	}
	return &PairingResponder{private: private, nonce: nonce, commitment: cloneBytes(commitment), info: cloneBytes(info)}, nil
}

// Challenge returns copies of pubA and nonceA. Repeated calls return the same
// challenge; they cannot replace it after seeing a reveal.
func (r *PairingResponder) Challenge() PairingChallenge {
	if r == nil || r.private == nil {
		return PairingChallenge{}
	}
	return PairingChallenge{PublicKey: r.private.PublicKey().Bytes(), Nonce: cloneBytes(r.nonce)}
}

// Destroy discards the old device's pending pairing secrets.
func (r *PairingResponder) Destroy() {
	if r != nil {
		clearBytes(r.nonce)
		r.private, r.nonce, r.commitment, r.info = nil, nil, nil, nil
	}
}

// Reveal consumes the request, freezing the received challenge before revealing
// pubB/nonceB. Display the resulting session code and compare on both devices.
// No transfer is permitted until the user explicitly confirms the match.
func (r *PairingRequest) Reveal(challenge PairingChallenge, info []byte) (PairingReveal, *PairingSession, error) {
	if r == nil || r.private == nil {
		return PairingReveal{}, nil, fmt.Errorf("%w: pairing request is unavailable", ErrInvalidKey)
	}
	defer r.Destroy()
	reveal := PairingReveal{PublicKey: r.private.PublicKey().Bytes(), Nonce: cloneBytes(r.nonce)}
	session, err := newPairingSession(r.private, challenge, reveal, r.commitment, info, false)
	if err != nil {
		clearBytes(reveal.Nonce)
		return PairingReveal{}, nil, err
	}
	return reveal, session, nil
}

// Finish consumes the challenge and verifies the reveal against the commitment
// before deriving a code. Any changed pubB or nonceB aborts authentication.
func (r *PairingResponder) Finish(reveal PairingReveal) (*PairingSession, error) {
	if r == nil || r.private == nil {
		return nil, fmt.Errorf("%w: pairing challenge is unavailable", ErrInvalidKey)
	}
	defer r.Destroy()
	if len(reveal.PublicKey) != PairingPublicKeySize || len(reveal.Nonce) != PairingNonceSize {
		return nil, &AuthenticationError{Operation: "pairing reveal size"}
	}
	if !hmac.Equal(pairingCommitment(reveal.PublicKey, reveal.Nonce), r.commitment) {
		return nil, &AuthenticationError{Operation: "pairing commitment"}
	}
	return newPairingSession(r.private, r.Challenge(), reveal, r.commitment, r.info, true)
}

func newPairingSession(private *ecdh.PrivateKey, challenge PairingChallenge, reveal PairingReveal, commitment, info []byte, sender bool) (*PairingSession, error) {
	if len(challenge.Nonce) != PairingNonceSize || len(reveal.Nonce) != PairingNonceSize || len(info) == 0 {
		return nil, fmt.Errorf("%w: pairing nonce or context is invalid", ErrInvalidKey)
	}
	peer := challenge.PublicKey
	if sender {
		peer = reveal.PublicKey
	}
	pub, err := ParsePairingPublicKey(peer)
	if err != nil {
		return nil, err
	}
	shared, err := private.ECDH(pub)
	if err != nil {
		return nil, &AuthenticationError{Operation: "pairing key agreement", Err: err}
	}
	defer clearBytes(shared)
	transcript := sha256.Sum256(joinLengthPrefixed([]byte("cfv2/pairing/transcript/v2"), info, commitment, challenge.PublicKey, reveal.PublicKey, challenge.Nonce, reveal.Nonce))
	macKey, err := hkdf.Key(sha256.New, shared, transcript[:], "cfv2/pairing/transfer-mac/v2", KeySize)
	if err != nil {
		return nil, fmt.Errorf("keyring: derive pairing MAC key: %w", err)
	}
	code := fmt.Sprintf("%08d", binary.BigEndian.Uint64(transcript[:8])%100_000_000)
	return &PairingSession{private: private, recipient: cloneBytes(reveal.PublicKey), info: cloneBytes(info), transcript: cloneBytes(transcript[:]), macKey: macKey, code: code, sender: sender}, nil
}

// ConfirmationCode returns the eight-digit SAS to compare on both devices.
func (s *PairingSession) ConfirmationCode() string {
	if s == nil {
		return ""
	}
	return s.code
}

// Confirm records the user's explicit comparison with the OTHER device's code.
// Never automatically pass this session's own code. A mismatch destroys the
// session; start again with fresh keys/nonces. A confirmation is not reusable.
func (s *PairingSession) Confirm(peerCode string) error {
	if s == nil || len(s.macKey) != KeySize || s.confirmed {
		return fmt.Errorf("%w: pairing session is unavailable or already confirmed", ErrInvalidKey)
	}
	if !hmac.Equal([]byte(s.code), []byte(peerCode)) {
		s.Destroy()
		return &AuthenticationError{Operation: "pairing confirmation"}
	}
	s.confirmed = true
	return nil
}

// Destroy clears the session MAC key and discards its ephemeral private key.
func (s *PairingSession) Destroy() {
	if s != nil {
		clearBytes(s.macKey)
		s.macKey, s.private, s.info, s.transcript, s.recipient = nil, nil, nil, nil, nil
		s.code, s.confirmed = "", false
	}
}

func (s *PairingSession) requireConfirmed(sender bool) error {
	if s == nil || !s.confirmed || s.sender != sender || len(s.macKey) != KeySize {
		return &AuthenticationError{Operation: "pairing requires confirmed session and correct role"}
	}
	return nil
}

func (s *PairingSession) transferMAC(enc, ciphertext, keyringBlob []byte) []byte {
	digest := sha256.Sum256(keyringBlob)
	mac := hmac.New(sha256.New, s.macKey)
	mac.Write(joinLengthPrefixed([]byte("cfv2/pairing/payload/v2"), s.transcript, s.info, s.recipient, enc, ciphertext, digest[:]))
	return mac.Sum(nil)
}
