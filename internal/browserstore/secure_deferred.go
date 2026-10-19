//go:build js && wasm

package browserstore

import (
	"errors"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

// deferredKeys contains names only; recovery values never enter the boot cache.
var deferredKeys = map[string]bool{}

func recoveryValueKey(key string) bool {
	return key == recoveryIndexKey || strings.HasPrefix(key, "cashflux:stale-recovery:") || strings.HasPrefix(key, "cashflux:quarantine:") || strings.HasPrefix(key, "cashflux:journal-recovery:") || key == "cashflux:dataset:checkpoint-backup"
}

// singleRecordMigration seals a legacy recovery value only when it is requested.
type singleRecordMigration struct {
	idbRecords
	key string
}

func (b singleRecordMigration) Keys() ([]string, error) { return []string{b.key}, nil }

func openDeferredRecord(key string) error {
	s := storage()
	s.Backend = singleRecordMigration{key: key}
	_, err := s.Migrate()
	return err
}

// reloadDeferredNames handles cross-tab invalidations without fetching opaque
// blobs. An explicit GetSecure remains the only on-demand value read.
func reloadDeferredNames(keys []string) []string {
	var active, deferred []string
	for _, key := range keys {
		if recoveryValueKey(key) {
			deferred = append(deferred, key)
		} else {
			active = append(active, key)
		}
	}
	if len(deferred) == 0 {
		return active
	}
	names, err := (idbRecords{}).Keys()
	if err != nil {
		return active
	}
	present := map[string]bool{}
	for _, key := range names {
		present[key] = true
	}
	mu.Lock()
	for _, key := range deferred {
		delete(cache, key)
		delete(readErrors, key)
		if present[key] {
			deferredKeys[key] = true
		} else {
			delete(deferredKeys, key)
		}
	}
	mu.Unlock()
	return active
}

// PruneEmptyJournalRecovery removes only authenticated, explicitly emptied
// recovery tombstones. Call after paint; a ciphertext CAS preserves a copy
// replaced by another tab while authentication yielded. Dataset keys are never
// considered, including the checkpoint backup.
func PruneEmptyJournalRecovery() (RecoveryStats, error) {
	var stats RecoveryStats
	keys, err := (idbRecords{}).Keys()
	if err != nil {
		return stats, err
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, "cashflux:journal-recovery:") {
			continue
		}
		time.Sleep(time.Millisecond)
		encoded, have, err := (idbRecords{}).Read(key)
		if err != nil {
			return stats, err
		}
		if !have {
			continue
		}
		plain, err := storage().Decode(key, encoded)
		if err != nil || plain != "" {
			continue
		}
		err = (idbRecords{}).Delete(key, &encoded)
		if errors.Is(err, atrest.ErrRecordChanged) {
			continue
		}
		if err != nil {
			return stats, err
		}
		mu.Lock()
		delete(cache, key)
		delete(deferredKeys, key)
		delete(readErrors, key)
		mu.Unlock()
		stats.Removed++
		stats.BytesFreed += len(encoded)
		publish(key)
	}
	return stats, nil
}
