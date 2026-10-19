//go:build js && wasm

package browserstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

const boot2EmptyKey = "cashflux:dataset:record:boot2:empty"

func TestBOOT2EmptySnapshotUsesBoundedKeyChecksAndExactCAS(t *testing.T) {
	initFixture(t)
	boot2FixtureRanges(t)
	values := map[string]string{}
	for i := range 64 {
		values[fmt.Sprintf("%s:%d", boot2EmptyKey, i)] = ""
	}
	if err := jrnlBatch(t, values, nil); err != nil {
		t.Fatal(err)
	}
	before := js.Global().Get("__fakeIDB").Call("transactionCount").Int()
	snapshot, err := ReadSecureSnapshot(nil, []string{boot2EmptyKey})
	transactions := js.Global().Get("__fakeIDB").Call("transactionCount").Int() - before
	if err != nil || len(snapshot.Values) != len(values) || len(snapshot.Errors) != 0 {
		t.Fatalf("empty snapshot: values=%d errors=%v err=%v", len(snapshot.Values), snapshot.Errors, err)
	}
	// One coherent record read and two durable key-state checks. No metadata
	// transaction per tombstone, and no unguarded cache-only snapshot.
	if transactions != 3 {
		t.Fatalf("snapshot transactions=%d, want 3", transactions)
	}
	var conditions []string
	for key := range values {
		if value, present := snapshot.Values[key]; !present || value != "" {
			t.Fatalf("lost tombstone %q", key)
		}
		conditions = append(conditions, key)
	}
	changed := conditions[len(conditions)/2]
	encoded, have := snapshot.EncodedRecord(changed)
	if !have {
		t.Fatal("snapshot lost original ciphertext")
	}
	replacement, err := storage().Encode(changed, "concurrent household edit")
	if err != nil {
		t.Fatal(err)
	}
	if err := (idbRecords{}).Replace(changed, replacement, &encoded); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	SetSecureSnapshotBatch(map[string]string{"cashflux:dataset": "stale repair"}, snapshot, conditions, func(err error) { done <- err })
	if err := waitResult(t, done); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("cached empty bypassed strict CAS: %v", err)
	}
	if _, have, err := GetSecure("cashflux:dataset"); err != nil || have {
		t.Fatalf("stale repair was published: have=%v err=%v", have, err)
	}
	current, err := ReadSecureSnapshot([]string{changed}, nil)
	if err != nil || current.Values[changed] != "concurrent household edit" {
		t.Fatalf("changed ciphertext treated as empty: %+v %v", current, err)
	}
}

func TestBOOT2EmptyProofAuthenticatesChangedBytesAndRecordKey(t *testing.T) {
	for _, test := range []struct {
		name       string
		replace    func(*testing.T, string) string
		want       string
		unreadable bool
	}{
		{name: "valid nonempty replacement", want: "new edit", replace: func(t *testing.T, _ string) string {
			raw, err := storage().Encode(boot2EmptyKey, "new edit")
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}},
		{name: "damaged tag", unreadable: true, replace: func(t *testing.T, raw string) string {
			var envelope atrest.Envelope
			if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
				t.Fatal(err)
			}
			envelope.Ciphertext[0] ^= 1
			changed, err := envelope.Encode()
			if err != nil {
				t.Fatal(err)
			}
			return string(changed)
		}},
		{name: "other record AAD", unreadable: true, replace: func(t *testing.T, _ string) string {
			raw, err := storage().Encode(boot2EmptyKey+":other", "")
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}},
		{name: "legacy plaintext", unreadable: true, replace: func(*testing.T, string) string { return "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			initFixture(t)
			put(t, boot2EmptyKey, "")
			original, have, err := (idbRecords{}).Read(boot2EmptyKey)
			if err != nil || !have {
				t.Fatal(err)
			}
			changed := test.replace(t, original)
			if err := (idbRecords{}).Replace(boot2EmptyKey, changed, &original); err != nil {
				t.Fatal(err)
			}
			snapshot, err := ReadSecureSnapshot([]string{boot2EmptyKey}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if test.unreadable {
				if snapshot.Errors[boot2EmptyKey] == nil {
					t.Fatal("cached authentication hid unreadable replacement")
				}
				if _, have := snapshot.Values[boot2EmptyKey]; have {
					t.Fatal("unreadable replacement published as empty")
				}
			} else if snapshot.Values[boot2EmptyKey] != test.want {
				t.Fatalf("replacement=%q", snapshot.Values[boot2EmptyKey])
			}
			if raw, have := snapshot.EncodedRecord(boot2EmptyKey); !have || raw != changed {
				t.Fatal("replacement ciphertext was lost")
			}
		})
	}
}

func TestBOOT2EmptyProofRejectsLockRevocationAndScopeChanges(t *testing.T) {
	for _, timing := range []string{"before", "during"} {
		for _, boundary := range []string{"lock", "other tab rewrap", "local rewrap", "scope"} {
			t.Run(timing+"/"+boundary, func(t *testing.T) {
				initFixture(t)
				put(t, boot2EmptyKey, "")
				raw, _, err := (idbRecords{}).Read(boot2EmptyKey)
				if err != nil {
					t.Fatal(err)
				}
				verifier := newEmptyRecordVerifier()
				if timing == "during" {
					if plain, err := verifier.decode(boot2EmptyKey, raw); err != nil || plain != "" {
						t.Fatalf("first decode: %q %v", plain, err)
					}
				}
				switch boundary {
				case "lock":
					secureKeys.Lock()
				case "local rewrap":
					if err := secureKeys.EnableAppLock("boot2-passcode"); err != nil {
						t.Fatal(err)
					}
				case "other tab rewrap":
					other, err := atrest.OpenKeyService(secureKeyStore, secureScope, secureWorkspace, true)
					if err != nil {
						t.Fatal(err)
					}
					defer other.Lock()
					if err := other.EnableAppLock("boot2-other-passcode"); err != nil {
						t.Fatal(err)
					}
				case "scope":
					oldService, oldStore := secureKeys, secureKeyStore
					scope := keyring.LocalScope("boot2-other-scope")
					keys, err := atrest.NewBrowserKeyStore(scope, "boot2-other-workspace")
					if err != nil {
						t.Fatal(err)
					}
					service, err := atrest.OpenKeyService(keys, scope, "boot2-other-workspace", false)
					if err != nil {
						t.Fatal(err)
					}
					secureKeys, secureKeyStore = service, keys
					defer func() { service.Lock(); keys.Close(); secureKeys, secureKeyStore = oldService, oldStore }()
				}
				if timing == "before" {
					if _, err := verifier.decode(boot2EmptyKey, raw); err == nil {
						t.Fatal("obsolete proof accepted at start of authentication")
					}
				} else if err := verifier.finish(); err == nil {
					t.Fatal("obsolete proof published after key boundary")
				}
			})
		}
	}
}

// boot2FixtureRanges installs only the missing browser API on the isolated Node
// shim. It does not change real browser data or production key/record services.
func boot2FixtureRanges(t *testing.T) {
	t.Helper()
	old := js.Global().Get("IDBKeyRange")
	script, err := os.ReadFile("secure_ranges_test.js")
	if err != nil {
		t.Fatal(err)
	}
	result := js.Global().Call("eval", string(script))
	done := make(chan string, 1)
	success := js.FuncOf(func(js.Value, []js.Value) any { done <- ""; return nil })
	failure := js.FuncOf(func(_ js.Value, args []js.Value) any { done <- args[0].String(); return nil })
	result.Call("then", success, failure)
	if err := <-done; err != "" {
		t.Fatal(err)
	}
	success.Release()
	failure.Release()
	t.Cleanup(func() { js.Global().Set("IDBKeyRange", old) })
}

func TestBOOT2EmptyProofsReloadUnderAppLock(t *testing.T) {
	initFixture(t)
	values := map[string]string{}
	for i := range 8 {
		values[fmt.Sprintf("%s:%d", boot2EmptyKey, i)] = ""
	}
	if err := jrnlBatch(t, values, nil); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	EnableAppLockSecure("boot2-lock-passcode", func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	locked := make(chan struct{}, 1)
	LockSecureThen(func() { locked <- struct{}{} })
	<-locked
	if _, err := ReadSecureSnapshot(nil, []string{boot2EmptyKey}); !errors.Is(err, atrest.ErrLocked) {
		t.Fatalf("locked proofs granted authority: %v", err)
	}
	UnlockSecureThen("boot2-lock-passcode", func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	before := js.Global().Get("__fakeIDB").Call("transactionCount").Int()
	snapshot, err := ReadSecureSnapshot(nil, []string{boot2EmptyKey})
	transactions := js.Global().Get("__fakeIDB").Call("transactionCount").Int() - before
	if err != nil || len(snapshot.Values) != 8 || len(snapshot.Errors) != 0 || transactions != 3 {
		t.Fatalf("unlocked snapshot: values=%d errors=%v transactions=%d err=%v", len(snapshot.Values), snapshot.Errors, transactions, err)
	}
}
