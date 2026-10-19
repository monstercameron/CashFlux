//go:build js && wasm

package browserstore

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

// Empty journal tombstones are immutable authentication results, not absence.
// Remember only exact envelope bytes produced or opened by this key service.
// No nonempty financial value or key material is retained in this cache. Proofs
// may be prepared while unlock is loading the cache: they grant no read authority
// until the snapshot's live lock and key-service checks have both succeeded.
type emptyRecordProof struct {
	encoded  string
	service  *atrest.KeyService
	revision uint64
}

var emptyRecordProofs = map[string]emptyRecordProof{}

const emptyRecordProofLimit = 4096

func rememberEmptyRecord(key, encoded string, service *atrest.KeyService, revision uint64) {
	if service == nil || !strings.HasPrefix(key, "cashflux:dataset:record:") || atrest.Classify(key) != atrest.MustEncrypt {
		return
	}
	state := service.State()
	if service != secureKeys || state == nil || state.Revision != revision {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if len(emptyRecordProofs) >= emptyRecordProofLimit {
		emptyRecordProofs = map[string]emptyRecordProof{}
	}
	emptyRecordProofs[key] = emptyRecordProof{encoded, service, revision}
}

func keyRevision(service *atrest.KeyService) uint64 {
	if service == nil {
		return 0
	}
	state := service.State()
	if state == nil {
		return 0
	}
	return state.Revision
}

func decodeSecureRecord(key, encoded string) (string, error) {
	service := secureKeys
	revision := keyRevision(service)
	plain, err := storage().Decode(key, encoded)
	if err == nil && plain == "" {
		rememberEmptyRecord(key, encoded, service, revision)
	}
	return plain, err
}

func encodeSecureRecord(key, plain string) (string, error) {
	service := secureKeys
	revision := keyRevision(service)
	encoded, err := storage().Encode(key, plain)
	if err == nil && plain == "" {
		rememberEmptyRecord(key, encoded, service, revision)
	}
	return encoded, err
}

// emptyRecordVerifier reuses proofs only inside one snapshot authentication.
// The first and last use authenticate an actual envelope through KeyService.Open:
// both read durable key metadata and reject revocation, a changed scope or a
// locked key. Every intervening proof names this same service/revision and exact
// record-key-bound ciphertext. Nothing is published until the final check passes.
// CAS still compares every ciphertext in the final strict repair transaction.
type emptyRecordVerifier struct {
	service      *atrest.KeyService
	revision     uint64
	key, encoded string
}

func newEmptyRecordVerifier() *emptyRecordVerifier {
	service := secureKeys
	return &emptyRecordVerifier{service: service, revision: keyRevision(service)}
}

func (v *emptyRecordVerifier) decode(key, encoded string) (string, error) {
	mu.Lock()
	proof, have := emptyRecordProofs[key]
	mu.Unlock()
	if !have || proof.encoded != encoded || proof.service != v.service || proof.revision != v.revision {
		return decodeSecureRecord(key, encoded)
	}
	if v.key == "" {
		if secureKeys != v.service || keyRevision(v.service) != v.revision {
			return "", atrest.ErrKeyStateChanged
		}
		plain, err := storage().Decode(key, encoded)
		if err != nil {
			return "", err
		}
		if plain != "" {
			return "", atrest.ErrRecordChanged
		}
		v.key, v.encoded = key, encoded
	}
	return "", nil
}

func (v *emptyRecordVerifier) finish() error {
	if v.key == "" {
		return nil
	}
	plain, err := storage().Decode(v.key, v.encoded)
	if err != nil {
		return err
	}
	if plain != "" || secureKeys != v.service || keyRevision(v.service) != v.revision {
		return atrest.ErrKeyStateChanged
	}
	return nil
}
