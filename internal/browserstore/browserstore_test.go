//go:build js && wasm

package browserstore

import (
	"errors"
	"os"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

func reset() {
	mu.Lock()
	cache = map[string]string{}
	pending = map[string]int{}
	readErrors = map[string]error{}
	mu.Unlock()
	db = js.Value{}
	durable, ready, secureLocked = false, false, false
	secureInitErr = nil
	secureKeys = nil
	secureKeyStore = nil
	secureScope = keyring.LocalScope("")
	secureWorkspace = "legacy-browser-profile"
	idbName = "cashflux-kv"
	migratedKeys = nil
}

func fixture(t *testing.T) {
	t.Helper()
	reset()
	oldIDB, oldCrypto, oldLS := js.Global().Get("indexedDB"), js.Global().Get("crypto"), js.Global().Get("localStorage")
	oldBroadcast := js.Global().Get("BroadcastChannel")
	js.Global().Set("BroadcastChannel", js.Undefined())
	js.Global().Set("localStorage", js.Undefined())
	if !oldCrypto.Truthy() || !oldCrypto.Get("subtle").Truthy() {
		js.Global().Set("crypto", js.Global().Get("require").Invoke("node:crypto").Get("webcrypto"))
	}
	script, err := os.ReadFile("../localdb/testdata/fake_indexeddb.js")
	if err != nil {
		t.Fatal(err)
	}
	js.Global().Call("eval", string(script))
	t.Cleanup(func() {
		Barrier()
		if secureKeys != nil {
			secureKeys.Lock()
		}
		if secureKeyStore != nil {
			secureKeyStore.Close()
		}
		if db.Truthy() {
			db.Call("close")
		}
		reset()
		js.Global().Set("indexedDB", oldIDB)
		js.Global().Set("crypto", oldCrypto)
		js.Global().Set("localStorage", oldLS)
		js.Global().Set("BroadcastChannel", oldBroadcast)
	})
}

func initFixture(t *testing.T) {
	t.Helper()
	fixture(t)
	Init()
	if err := InitError(); err != nil {
		t.Fatal(err)
	}
	if !Durable() {
		t.Fatal("IndexedDB not opened")
	}
}

func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for IndexedDB")
		return nil
	}
}
func put(t *testing.T, key, value string) {
	t.Helper()
	done := make(chan error, 1)
	SetSecure(key, value, func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestCacheVolatileFallback(t *testing.T) {
	reset()
	for _, key := range []string{"cashflux:x", "cashflux:y", "cashflux:z"} {
		Set(key, "1")
	}
	if len(Keys()) != 3 || GetString("cashflux:x") != "1" {
		t.Fatal("memory cache failed")
	}
	Set("cashflux:x", "2")
	if GetString("cashflux:x") != "2" {
		t.Fatal("memory cache overwrite failed")
	}
	Remove("cashflux:x")
	if _, ok := Get("cashflux:x"); ok {
		t.Fatal("memory cache remove failed")
	}
}

func TestSetThenWithoutDBNeverAcknowledges(t *testing.T) {
	reset()
	acknowledged := false
	SetThen("cashflux:theme", "dark", func() { acknowledged = true })
	Barrier()
	if acknowledged {
		t.Fatal("unpersisted write acknowledged")
	}
	if _, have := Get("cashflux:theme"); have {
		t.Fatal("failed write changed cache")
	}
}

func TestSetDurableEncryptsAndPublishesOnlyAfterCommit(t *testing.T) {
	initFixture(t)
	const key = "cashflux:dataset"
	put(t, key, "old")
	done := make(chan error, 1)
	SetDurable(key, "Blue Bottle 1,850 Checking", func(err error) { done <- err })
	if got := GetString(key); got != "old" {
		t.Fatalf("changed before commit: %q", got)
	}
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	if GetString(key) != "Blue Bottle 1,850 Checking" {
		t.Fatal("committed value missing")
	}
	raw, have, err := (idbRecords{}).Read(key)
	if err != nil || !have || !atrest.IsSealed(raw) || strings.Contains(raw, "Blue Bottle") {
		t.Fatalf("unprotected durable record: %v %v", have, err)
	}
	got, have, err := GetSecure(key)
	if err != nil || !have || got != GetString(key) {
		t.Fatalf("readback: %q %v %v", got, have, err)
	}
}

func TestSetThenAbortDoesNotAcknowledgeOrChangeCache(t *testing.T) {
	initFixture(t)
	const key = "cashflux:dataset"
	put(t, key, "old")
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	acknowledged := false
	SetThen(key, "replacement", func() { acknowledged = true })
	Barrier()
	if acknowledged || GetString(key) != "old" {
		t.Fatal("aborted write acknowledged or exposed")
	}
	if got, _, err := GetSecure(key); err != nil || got != "old" {
		t.Fatalf("aborted write retained: %q %v", got, err)
	}
}

func TestRemoveDurableAbortPreservesData(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "old")
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	done := make(chan error, 1)
	RemoveDurable("cashflux:dataset", func(err error) { done <- err })
	if err := waitResult(t, done); err == nil || !strings.Contains(err.Error(), "QuotaExceededError") {
		t.Fatalf("abort error: %v", err)
	}
	if GetString("cashflux:dataset") != "old" {
		t.Fatal("aborted delete changed cache")
	}
	RemoveDurable("cashflux:dataset", func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	if _, have := Get("cashflux:dataset"); have {
		t.Fatal("committed delete not applied")
	}
}

func TestSecureRefusesTransientCredentials(t *testing.T) {
	initFixture(t)
	for _, key := range []string{"cashflux:auth:access-token", "cashflux:auth:refresh-token", "cashflux:openai-key", "cashflux:passcode", "cashflux:recovery-code"} {
		done := make(chan error, 1)
		SetSecure(key, "private", func(err error) { done <- err })
		if err := waitResult(t, done); !errors.Is(err, atrest.ErrPersistenceForbidden) {
			t.Fatalf("%s: %v", key, err)
		}
		Set(key, "memory only")
		if GetString(key) != "memory only" {
			t.Fatal("transient credential missing")
		}
		if _, have, err := (idbRecords{}).Read(key); err != nil || have {
			t.Fatalf("transient credential persisted: %v %v", have, err)
		}
	}
}

func TestSecureLockReadsReturnTypedError(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "private")
	if err := secureKeys.EnableAppLock("passcode"); err != nil {
		t.Fatal(err)
	}
	LockSecure()
	_, have, err := GetSecure("cashflux:dataset")
	var recordErr *RecordError
	if !have || !errors.Is(err, atrest.ErrLocked) || !errors.As(err, &recordErr) {
		t.Fatalf("locked read: %v %v", have, err)
	}
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("legacy read hid lock as empty data")
			}
		}()
		Get("cashflux:dataset")
	}()
	if err := UnlockSecure("wrong"); err == nil {
		t.Fatal("wrong passcode accepted")
	}
	if err := UnlockSecure("passcode"); err != nil {
		t.Fatal(err)
	}
	if GetString("cashflux:dataset") != "private" {
		t.Fatal("unlock failed")
	}
}

func TestSecureQueuePreservesInvocationOrder(t *testing.T) {
	initFixture(t)
	for _, value := range []string{"first", "second", "third"} {
		Set("cashflux:dataset", value)
	}
	Barrier()
	got, have, err := GetSecure("cashflux:dataset")
	if err != nil || !have || got != "third" {
		t.Fatalf("queue reordered writes: %q %v %v", got, have, err)
	}
}

func TestCompatibilityReadModifyWriteRetainsAllPendingEdits(t *testing.T) {
	initFixture(t)
	const key = "cashflux:sync-queue"
	for _, value := range []string{"first;", "second;", "third;"} {
		Set(key, GetString(key)+value)
	}
	Barrier()
	if got, _, err := GetSecure(key); err != nil || got != "first;second;third;" {
		t.Fatalf("pending edits dropped: %q %v", got, err)
	}
}

func TestSecureBatchGenerationIsAtomicAndRefusesStaleWriter(t *testing.T) {
	initFixture(t)
	const dataset = "cashflux:dataset"
	const generation = "cashflux:dataset:gen"
	result := make(chan error, 1)
	SetSecureBatch(map[string]string{dataset: "first", generation: "one"}, map[string]string{generation: ""}, func(err error) { result <- err })
	if err := waitResult(t, result); err != nil {
		t.Fatal(err)
	}
	SetSecureBatch(map[string]string{dataset: "stale", generation: "two"}, map[string]string{generation: "old"}, func(err error) { result <- err })
	if err := waitResult(t, result); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("stale overwrite: %v", err)
	}
	if GetString(dataset) != "first" || GetString(generation) != "one" {
		t.Fatal("stale batch changed committed state")
	}
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	SetSecureBatch(map[string]string{dataset: "aborted", generation: "two"}, map[string]string{generation: "one"}, func(err error) { result <- err })
	if err := waitResult(t, result); err == nil {
		t.Fatal("aborted batch acknowledged")
	}
	for key, want := range map[string]string{dataset: "first", generation: "one"} {
		if got, _, err := GetSecure(key); err != nil || got != want {
			t.Fatalf("batch not atomic: %s %q %v", key, got, err)
		}
	}
}

func TestInitWithoutIndexedDBReportsNonDurable(t *testing.T) {
	reset()
	old := js.Global().Get("indexedDB")
	js.Global().Set("indexedDB", js.Undefined())
	t.Cleanup(func() { js.Global().Set("indexedDB", old); reset() })
	Init()
	if Durable() {
		t.Fatal("fallback reported durability")
	}
	done := make(chan error, 1)
	SetDurable("cashflux:theme", "light", func(err error) { done <- err })
	if err := waitResult(t, done); err == nil {
		t.Fatal("fallback durable write acknowledged")
	}
}

func TestStrictDurabilityUnsupportedIsNotSilentlyDowngraded(t *testing.T) {
	initFixture(t)
	original := db
	db = js.Global().Get("Function").New("db", `
return {transaction(...args) {
 if(args[1] === 'readwrite') throw new TypeError('strict durability unsupported');
 return db.transaction(...args);
}};
`).Invoke(original)
	t.Cleanup(func() { db = original })
	done := make(chan error, 1)
	SetSecure("cashflux:dataset", "private", func(err error) { done <- err })
	if err := waitResult(t, done); err == nil || !strings.Contains(err.Error(), "TypeError") {
		t.Fatalf("strict transaction downgraded: %v", err)
	}
	if _, have := Get("cashflux:dataset"); have {
		t.Fatal("uncommitted write published")
	}
}

func TestRecoveryWrapperRestoresRecordsWithoutPersistingTheCode(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "private household")
	code, err := secureKeys.AddRecovery()
	if err != nil {
		t.Fatal(err)
	}
	state := SecureKeyState()
	if state == nil {
		t.Fatal("key metadata unavailable to recovery gate")
	}
	state.WorkspaceID = "mutated copy"
	if SecureKeyState().WorkspaceID == state.WorkspaceID {
		t.Fatal("caller mutated retained key metadata")
	}
	LockSecure()
	done := make(chan error, 1)
	UnlockRecoverySecure(code, func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	RepairDeviceSecure(func(err error) { done <- err })
	if err := waitResult(t, done); !errors.Is(err, atrest.ErrInvalidKeyState) {
		t.Fatalf("existing device should not be silently replaced: %v", err)
	}
	if got, _, err := GetSecure("cashflux:dataset"); err != nil || got != "private household" {
		t.Fatalf("recovery lost records: %q %v", got, err)
	}
	AssertNoPlaintextForTest(t, []string{code, "private household"})
}

func TestAppLockWrapperTransitionsPreserveCiphertext(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "private household")
	before, _, err := (idbRecords{}).Read("cashflux:dataset")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	EnableAppLockSecure("first", func(err error) { done <- err })
	if err := waitResult(t, done); err != nil || !SecureKeyState().AppLock {
		t.Fatalf("enable: %v", err)
	}
	ChangePasscodeSecure("second", func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	LockSecure()
	if err := UnlockSecure("first"); err == nil {
		t.Fatal("old passcode still accepted")
	}
	if err := UnlockSecure("second"); err != nil {
		t.Fatal(err)
	}
	DisableAppLockSecure(func(err error) { done <- err })
	if err := waitResult(t, done); err != nil || SecureKeyState().AppLock {
		t.Fatalf("disable: %v", err)
	}
	after, _, err := (idbRecords{}).Read("cashflux:dataset")
	if err != nil || after != before {
		t.Fatal("wrap changes rewrote records", err)
	}
}
