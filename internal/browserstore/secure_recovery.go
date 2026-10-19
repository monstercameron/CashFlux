//go:build js && wasm

package browserstore

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

const recoveryIndexKey = "cashflux:recovery-index:v1"

// RecoveryDescribe authenticates application scope and fingerprints a payload.
// It must not guess an owner for a legacy unbound copy.
type RecoveryDescribe func(key, plaintext string) RecoveryDescription

// RecoveryLive binds containment evidence to one authenticated live snapshot.
// Pruning compares its exact ciphertexts in the same transaction as deletion.
type RecoveryLive struct {
	Versions   map[string][]string
	Snapshot   SecureSnapshot
	Conditions []string
}

// RecoveryStats reports only committed removals, in encoded UTF-8 bytes.
type RecoveryStats struct{ Removed, BytesFreed int }

type recoveryIndex struct {
	Version  int             `json:"version"`
	Sequence uint64          `json:"sequence"`
	Entries  []RecoveryEntry `json:"entries"`
}

func recoveryHash(raw string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) }

func retainedRecoveryKey(key string) bool {
	return strings.HasPrefix(key, "cashflux:stale-recovery:") || (strings.HasPrefix(key, "cashflux:quarantine:") && !strings.HasSuffix(key, ":reason"))
}

// RetainRecovery durably adds a copy and prunes eligible older copies in the
// SAME strict transaction. Identical plaintext returns an existing key. The
// callback runs outside the serial writer and never precedes the commit.
func RetainRecovery(key, plaintext, reason string, describe RecoveryDescribe, live RecoveryLive, done func(string, error)) {
	enqueue(func() {
		actual, _, err := reconcileRecovery(key, plaintext, reason, describe, live)
		if done != nil {
			go done(actual, err)
		}
	})
}

// PruneRecovery migrates an oversized store after first paint. It reads one
// payload at a time, yields between values, and never changes dataset keys.
func PruneRecovery(describe RecoveryDescribe, live RecoveryLive, done func(RecoveryStats, error)) {
	// Scanning old blobs must not occupy the writer queue. The final strict
	// transaction compares the index, membership and every source ciphertext,
	// so ordinary durable writes may proceed while this worker yields.
	go func() {
		var total RecoveryStats
		for {
			_, stats, err := reconcileRecovery("", "", "", describe, live)
			total.Removed += stats.Removed
			total.BytesFreed += stats.BytesFreed
			if err != nil || stats.Removed == 0 {
				if done != nil {
					done(total, err)
				}
				return
			}
			// Keep strict transactions small even on a 330 MB legacy store.
			// Ordinary writers can run between maintenance batches.
			time.Sleep(time.Millisecond)
		}
	}()
}

func reconcileRecovery(key, plaintext, reason string, describe RecoveryDescribe, live RecoveryLive) (string, RecoveryStats, error) {
	for attempt := range 4 {
		actual, stats, err := reconcileRecoveryOnce(key, plaintext, reason, describe, live, attempt > 0)
		if !errors.Is(err, atrest.ErrRecordChanged) {
			return actual, stats, err
		}
		// A changed live dataset invalidates containment evidence. A retry
		// without that evidence can retain extra copies but cannot lose an edit.
		live = RecoveryLive{}
		time.Sleep(time.Millisecond)
	}
	return "", RecoveryStats{}, atrest.ErrRecordChanged
}

func reconcileRecoveryOnce(key, plaintext, reason string, describe RecoveryDescribe, live RecoveryLive, refresh bool) (string, RecoveryStats, error) {
	var stats RecoveryStats
	if secureInitErr != nil {
		return "", stats, secureInitErr
	}
	if key != "" && !retainedRecoveryKey(key) {
		return "", stats, fmt.Errorf("invalid recovery key %q", key)
	}
	keys, err := (idbRecords{}).Keys()
	if err != nil {
		return "", stats, err
	}
	indexRaw, indexPresent, err := (idbRecords{}).Read(recoveryIndexKey)
	if err != nil {
		return "", stats, err
	}
	index := recoveryIndex{Version: 1}
	if indexPresent {
		plain, err := storage().Decode(recoveryIndexKey, indexRaw)
		if err != nil {
			return "", stats, err
		}
		if plain != "" {
			if err := json.Unmarshal([]byte(plain), &index); err != nil {
				return "", stats, fmt.Errorf("read recovery index: %w", err)
			}
			if index.Version != 1 {
				return "", stats, fmt.Errorf("unsupported recovery index version %d", index.Version)
			}
		}
	}
	indexed := map[string]RecoveryEntry{}
	for _, e := range index.Entries {
		indexed[e.Key] = e
	}
	present := map[string]bool{}
	var entries []RecoveryEntry
	// Legacy quarantine keys encode their append order; stale keys used UUIDs, so
	// their stable key order is only a retention tie-break, never a data winner.
	sort.Slice(keys, func(i, j int) bool { return recoveryLegacyLess(keys[i], keys[j]) })
	for _, k := range keys {
		if !retainedRecoveryKey(k) {
			continue
		}
		present[k] = true
		e, have := indexed[k]
		if refresh {
			have = false
		}
		if !have {
			encoded, exists, readErr := (idbRecords{}).Read(k)
			if readErr != nil {
				return "", stats, readErr
			}
			if !exists {
				return "", stats, atrest.ErrRecordChanged
			}
			if previous, known := indexed[k]; refresh && known && recoveryHash(encoded) == previous.EncodedHash {
				entries = append(entries, previous)
				time.Sleep(time.Millisecond)
				continue
			}
			if !atrest.IsSealed(encoded) {
				if err := openDeferredRecord(k); err != nil {
					return "", stats, err
				}
				encoded, exists, readErr = (idbRecords{}).Read(k)
				if readErr != nil || !exists {
					return "", stats, atrest.ErrRecordChanged
				}
			}
			raw, decodeErr := storage().Decode(k, encoded)
			desc := RecoveryDescription{Group: "unreadable:" + k, Hash: recoveryHash(encoded), Protect: true}
			if decodeErr == nil {
				desc = describe(k, raw)
			}

			index.Sequence++
			e = RecoveryEntry{RecoveryDescription: desc, Key: k, Order: index.Sequence, Bytes: len(encoded), EncodedHash: recoveryHash(encoded), Empty: decodeErr == nil && raw == ""}
			if e.Hash == "" {
				e.Hash = recoveryHash(raw)
			}
			reason, _, readErr := (idbRecords{}).Read(k + ":reason")
			if readErr != nil {
				return "", stats, readErr
			}
			e.ReasonBytes = len(reason)
			time.Sleep(time.Millisecond)
		}
		entries = append(entries, e)
	}
	actual := key
	newValues := map[string]string{}
	if key != "" {
		desc := describe(key, plaintext)
		if desc.Hash == "" {
			desc.Hash = recoveryHash(plaintext)
		}
		var newest uint64
		for _, e := range entries {
			if e.Group == desc.Group && e.Hash == desc.Hash && (actual == key || e.Order > newest) {
				actual = e.Key
				newest = e.Order
			}
		}
		if actual != key {
			// A repeated edit is the newest observation of that distinct
			// content. Reuse its ciphertext and refresh only its logical order.
			index.Sequence++
			for i := range entries {
				if entries[i].Key == actual {
					entries[i].Order = index.Sequence
					break
				}
			}
		}
		if actual == key {
			if present[key] {
				return "", stats, atrest.ErrRecordChanged
			}
			index.Sequence++
			encoded, err := storage().Encode(key, plaintext)
			if err != nil {
				return "", stats, err
			}
			entries = append(entries, RecoveryEntry{RecoveryDescription: desc, Key: key, Order: index.Sequence, Bytes: len(encoded), EncodedHash: recoveryHash(encoded)})
			newValues[key] = encoded
			if strings.HasPrefix(key, "cashflux:quarantine:") {
				encodedReason, err := storage().Encode(key+":reason", reason)
				if err != nil {
					return "", stats, err
				}
				newValues[key+":reason"] = encodedReason
				entries[len(entries)-1].ReasonBytes = len(encodedReason)
			}
		}
	}
	keep, remove := PlanRecoveryRetention(entries, live.Versions)
	verified := map[string]bool{}
	if key == "" {
		// Background cleanup commits at most three payload deletions at once.
		// Pending removals remain indexed and are reconsidered on the next pass.
		plannedKeep := keep
		if len(remove) > RecoveryLimit {
			keep = append(append([]RecoveryEntry(nil), keep...), remove[RecoveryLimit:]...)
			remove = remove[:RecoveryLimit]
		}
		for _, removed := range remove {
			verified[removed.Key] = true
			duplicate := ""
			for _, retained := range plannedKeep {
				if removed.Group == retained.Group && removed.Hash != "" && removed.Hash == retained.Hash {
					duplicate = retained.Key
					break
				}
			}
			if duplicate != "" {
				verified[duplicate] = true
				continue
			}
			// Containment can depend on several retained copies. Verify every
			// source in this scope, along with the exact live snapshot below.
			for _, retained := range plannedKeep {
				if removed.Group == retained.Group {
					verified[retained.Key] = true
				}
			}
		}
	} else {
		for _, entry := range entries {
			verified[entry.Key] = true
		}
	}
	// An add must retain the edit it promises to the caller, even if a protected
	// older copy caused the normal cap to be exceeded.
	for _, e := range remove {
		if e.Key == actual && key != "" {
			return "", stats, fmt.Errorf("new recovery copy would not be retained")
		}
	}
	index.Entries = keep
	plainIndex, err := json.Marshal(index)
	if err != nil {
		return "", stats, err
	}
	encodedIndex, err := storage().Encode(recoveryIndexKey, string(plainIndex))
	if err != nil {
		return "", stats, err
	}
	err = recordTransaction(true, func(tx js.Value, retain func(js.Func), fail func(error)) {
		os := tx.Call("objectStore", idbStoreName)
		abort := func() { fail(atrest.ErrRecordChanged); tx.Call("abort") }
		req := os.Call("get", recoveryIndexKey)
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			v := req.Get("result")
			have := !v.IsUndefined() && !v.IsNull()
			if have != indexPresent || (have && (v.Type() != js.TypeString || v.String() != indexRaw)) {
				abort()
				return nil
			}
			// Include legacy writers which do not know about the retention index.
			kr := os.Call("getAllKeys")
			kf := js.FuncOf(func(js.Value, []js.Value) any {
				found := map[string]bool{}
				ks := kr.Get("result")
				for i := 0; i < ks.Length(); i++ {
					k := ks.Index(i).String()
					if retainedRecoveryKey(k) {
						found[k] = true
					}
				}
				if len(found) != len(present) {
					abort()
					return nil
				}
				for k := range present {
					if !found[k] {
						abort()
						return nil
					}
				}
				for _, condition := range live.Conditions {
					k := condition
					get := os.Call("get", k)
					check := js.FuncOf(func(js.Value, []js.Value) any {
						v := get.Get("result")
						expected, have := live.Snapshot.EncodedRecord(k)
						present := !v.IsUndefined() && !v.IsNull()
						if present != have || (have && (v.Type() != js.TypeString || v.String() != expected)) {
							abort()
						}
						return nil
					})
					retain(check)
					get.Set("onsuccess", check)
				}
				// Verify deletion targets and every retained containment source,
				// not just the index. Old-tab replacements invalidate the plan.
				for _, e := range entries {
					if !verified[e.Key] {
						continue
					}
					if _, adding := newValues[e.Key]; adding {
						continue
					}
					entry := e
					get := os.Call("get", entry.Key)
					check := js.FuncOf(func(js.Value, []js.Value) any {
						v := get.Get("result")
						if v.Type() != js.TypeString || recoveryHash(v.String()) != entry.EncodedHash {
							abort()
						}
						return nil
					})
					retain(check)
					get.Set("onsuccess", check)
				}
				for _, e := range remove {
					os.Call("delete", e.Key)
					os.Call("delete", e.Key+":reason")
				}
				for k, v := range newValues {
					os.Call("put", v, k)
				}
				os.Call("put", encodedIndex, recoveryIndexKey)
				return nil
			})
			retain(kf)
			kr.Set("onsuccess", kf)
			return nil
		})
		retain(fn)
		req.Set("onsuccess", fn)
	})
	if err != nil {
		return "", stats, err
	}
	mu.Lock()
	for _, e := range remove {
		delete(cache, e.Key)
		delete(cache, e.Key+":reason")
		delete(deferredKeys, e.Key)
		delete(deferredKeys, e.Key+":reason")
		stats.Removed++
		stats.BytesFreed += e.Bytes + e.ReasonBytes
	}
	// Only index names are cached. Downloads explicitly authenticate durable bytes.
	for _, e := range keep {
		deferredKeys[e.Key] = true
	}
	deferredKeys[recoveryIndexKey] = true
	delete(cache, recoveryIndexKey)
	mu.Unlock()
	changed := []string{recoveryIndexKey}
	for _, e := range remove {
		changed = append(changed, e.Key, e.Key+":reason")
	}
	for k := range newValues {
		changed = append(changed, k)
	}
	publishBatch(changed)
	return actual, stats, nil
}

func recoveryLegacyLess(a, b string) bool {
	const prefix = "cashflux:quarantine:"
	if strings.HasPrefix(a, prefix) && strings.HasPrefix(b, prefix) {
		var at, as, bt, bs uint64
		fmt.Sscanf(strings.SplitN(strings.TrimPrefix(a, prefix), ":", 2)[0], "%d-%d", &at, &as)
		fmt.Sscanf(strings.SplitN(strings.TrimPrefix(b, prefix), ":", 2)[0], "%d-%d", &bt, &bs)
		if at != bt {
			return at < bt
		}
		if as != bs {
			return as < bs
		}
	}
	return a < b
}
