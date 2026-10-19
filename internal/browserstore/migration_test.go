//go:build js && wasm

package browserstore

import (
	"errors"
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

func legacyStorage(t *testing.T, values map[string]string) js.Value {
	t.Helper()
	store := js.Global().Get("Function").New(`
const values = new Map();
return {get length(){return values.size},key(i){return Array.from(values.keys())[i] ?? null},
getItem(k){return values.has(k) ? values.get(k) : null},setItem(k,v){values.set(k,String(v))},removeItem(k){values.delete(k)}};
`).Invoke()
	for key, value := range values {
		store.Call("setItem", key, value)
	}
	js.Global().Set("localStorage", store)
	return store
}

func TestMigrationSealsEveryExistingSensitiveCatalogueEntry(t *testing.T) {
	initFixture(t)
	var want []string
	for _, entry := range atrest.Catalog() {
		if entry.Class != atrest.MustEncrypt {
			continue
		}
		key := "cashflux:" + strings.TrimSuffix(entry.Key, "*")
		if strings.HasSuffix(key, ":") {
			key += "test"
		}
		want = append(want, key)
		if err := (idbRecords{}).Replace(key, "Blue Bottle 1,850 private household", nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := initializeSecure(); err != nil {
		t.Fatal(err)
	}
	if len(MigratedKeys()) != len(want) {
		t.Fatalf("migrated %d want %d", len(MigratedKeys()), len(want))
	}
	for _, key := range want {
		raw, present, err := (idbRecords{}).Read(key)
		if err != nil || !present || !atrest.IsSealed(raw) || strings.Contains(raw, "Blue Bottle") {
			t.Fatalf("unsealed key %s: %v", key, err)
		}
		if got, _, err := GetSecure(key); err != nil || got != "Blue Bottle 1,850 private household" {
			t.Fatalf("read %s: %q %v", key, got, err)
		}
	}
	if err := initializeSecure(); err != nil {
		t.Fatal(err)
	}
	if len(MigratedKeys()) != 0 {
		t.Fatal("second migration rewrote sealed records")
	}
}

func TestLegacyLocalStorageMigrationWhenIDBAlreadyPopulated(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:theme", "dark")
	ls := legacyStorage(t, map[string]string{"cashflux:dataset": "Blue Bottle", "cashflux:tx-filter": "1,850", "cashflux:openai-key": "sk-legacy-private"})
	if err := initializeSecure(); err != nil {
		t.Fatal(err)
	}
	if ls.Get("length").Int() != 0 {
		t.Fatal("verified legacy sources not removed")
	}
	if GetString("cashflux:dataset") != "Blue Bottle" || GetString("cashflux:theme") != "dark" {
		t.Fatal("legacy import lost data")
	}
	if _, have, _ := (idbRecords{}).Read("cashflux:openai-key"); have {
		t.Fatal("raw AI key persisted")
	}
	if got, have, err := GetSecure("cashflux:cred:openai"); err != nil || !have || !strings.Contains(got, "sk-legacy-private") {
		t.Fatal("legacy credential not durably promoted", err)
	}
	if err := initializeSecure(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyCommitFailureRetainsSourceAndResumes(t *testing.T) {
	initFixture(t)
	ls := legacyStorage(t, map[string]string{"cashflux:dataset": "Blue Bottle"})
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	if err := initializeSecure(); err == nil {
		t.Fatal("failed migration acknowledged")
	}
	if ls.Call("getItem", "cashflux:dataset").String() != "Blue Bottle" {
		t.Fatal("source removed on failure")
	}
	if err := initializeSecure(); err != nil {
		t.Fatal(err)
	}
	if ls.Get("length").Int() != 0 {
		t.Fatal("resume left plaintext")
	}
}

func TestLegacyConflictPreservesBothCopies(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "authoritative data")
	ls := legacyStorage(t, map[string]string{"cashflux:dataset": "different legacy data"})
	if err := initializeSecure(); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("conflict: %v", err)
	}
	if ls.Get("length").Int() != 1 {
		t.Fatal("conflicting legacy source deleted")
	}
	if got, _, err := GetSecure("cashflux:dataset"); err != nil || got != "authoritative data" {
		t.Fatal("authoritative copy replaced", err)
	}
}

func TestMigrationRemovesPersistentAccessAndRefreshTokens(t *testing.T) {
	initFixture(t)
	for _, key := range []string{"cashflux:auth:access-token", "cashflux:auth:refresh-token", "cashflux:auth:csrf"} {
		if err := (idbRecords{}).Replace(key, "old-secret", nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := initializeSecure(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"cashflux:auth:access-token", "cashflux:auth:refresh-token", "cashflux:auth:csrf"} {
		if GetString(key) != "old-secret" {
			t.Fatal("live session lost during migration")
		}
		if _, have, err := (idbRecords{}).Read(key); err != nil || have {
			t.Fatalf("secret still persistent %s: %v", key, err)
		}
	}
}

func TestSecureTransactionAbortAfterSuccessfulPut(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "old")
	intercept := js.Global().Get("Function").New("db", `
const original = db.transaction;
let wrote = false, strict = false;
db.transaction = function(...args) {
 const tx = original.apply(this,args);
 if(args[1] !== 'readwrite') return tx;
 strict = args[2]?.durability === 'strict';
 const os = tx.objectStore.bind(tx);
 tx.objectStore = function(...names) {
  const store = os(...names), put = store.put.bind(store);
  store.put = function(...values) {const req = put(...values); req.onsuccess = function(){wrote=true;tx.abort()};return req};
  return store;
 };
 return tx;
};
return {get wrote(){return wrote},get strict(){return strict},restore(){db.transaction=original}};
`).Invoke(db)
	done := make(chan error, 1)
	SetSecure("cashflux:dataset", "Blue Bottle", func(err error) { done <- err })
	err := waitResult(t, done)
	intercept.Call("restore")
	if err == nil || !intercept.Get("wrote").Bool() || !intercept.Get("strict").Bool() {
		t.Fatalf("post-put abort not tested: %v", err)
	}
	if GetString("cashflux:dataset") != "old" {
		t.Fatal("aborted replacement published")
	}
	if got, _, err := GetSecure("cashflux:dataset"); err != nil || got != "old" {
		t.Fatal("aborted replacement persisted", err)
	}
}

func TestConfiguredScopesUseSeparatePhysicalDatabases(t *testing.T) {
	fixture(t)
	scopeA := keyring.Scope{ServerID: "https://server-a.test", AccountID: "account-a"}
	scopeB := keyring.Scope{ServerID: "https://server-b.test", AccountID: "account-b"}
	if err := ConfigureScope(scopeA, "same-workspace-id"); err != nil {
		t.Fatal(err)
	}
	Init()
	if err := InitError(); err != nil {
		t.Fatal(err)
	}
	put(t, "cashflux:dataset", "household A")
	nameA := idbName
	if err := ConfigureScope(scopeB, "same-workspace-id"); err == nil {
		t.Fatal("live scope change permitted")
	}
	Barrier()
	secureKeys.Lock()
	secureKeyStore.Close()
	db.Call("close")
	reset()
	legacyStorage(t, map[string]string{"cashflux:dataset": "unscoped legacy household"})
	if err := ConfigureScope(scopeB, "same-workspace-id"); err != nil {
		t.Fatal(err)
	}
	Init()
	if err := InitError(); err != nil {
		t.Fatal(err)
	}
	if nameA == idbName {
		t.Fatal("physical scope database collision")
	}
	if _, have, err := GetSecure("cashflux:dataset"); err != nil || have {
		t.Fatalf("foreign/legacy data entered scope B: %v %v", have, err)
	}
	put(t, "cashflux:dataset", "household B")
	Barrier()
	secureKeys.Lock()
	secureKeyStore.Close()
	db.Call("close")
	reset()
	if err := ConfigureScope(scopeA, "same-workspace-id"); err != nil {
		t.Fatal(err)
	}
	Init()
	if err := InitError(); err != nil {
		t.Fatal(err)
	}
	if got, _, err := GetSecure("cashflux:dataset"); err != nil || got != "household A" {
		t.Fatalf("scope A not isolated: %q %v", got, err)
	}
}

func TestMissingKeyMetadataNeverCreatesAReplacementOverCiphertext(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "private household")
	before, _, err := (idbRecords{}).Read("cashflux:dataset")
	if err != nil {
		t.Fatal(err)
	}
	secureKeys.Lock()
	secureKeyStore.Close()
	done := make(chan struct{})
	req := js.Global().Get("indexedDB").Call("deleteDatabase", keyDatabaseNameForTest())
	callback := js.FuncOf(func(js.Value, []js.Value) any { close(done); return nil })
	req.Set("onsuccess", callback)
	<-done
	callback.Release()
	if err := initializeSecure(); !errors.Is(err, atrest.ErrRecoveryRequired) {
		t.Fatalf("missing keys reset ciphertext: %v", err)
	}
	after, _, err := (idbRecords{}).Read("cashflux:dataset")
	if err != nil || after != before {
		t.Fatal("ciphertext was changed", err)
	}
	if dump := js.Global().Get("__fakeIDB").Call("dump", keyDatabaseNameForTest()).String(); strings.Contains(dump, "deviceKey") {
		t.Fatal("replacement key was generated")
	}
}
