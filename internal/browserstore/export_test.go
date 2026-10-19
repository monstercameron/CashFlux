//go:build js && wasm

package browserstore

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

func SetupSecureForTest(t *testing.T)                  { initFixture(t) }
func PutSecureForTest(t *testing.T, key, value string) { put(t, key, value) }
func ReopenSecureForTest() error                       { return initializeSecure() }
func EnableSecureForTest(passcode string) error        { return secureKeys.EnableAppLock(passcode) }

func keyDatabaseNameForTest() string {
	raw, _ := json.Marshal([]string{secureScope.ServerID, secureScope.AccountID, secureWorkspace})
	return "cashflux-atrest-keys-v1:" + base64.RawURLEncoding.EncodeToString(raw)
}
func AssertNoPlaintextForTest(t *testing.T, sentinels []string) {
	t.Helper()
	keys, err := (idbRecords{}).Keys()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		raw, _, err := (idbRecords{}).Read(key)
		if err != nil {
			t.Fatal(err)
		}
		for _, sentinel := range sentinels {
			if sentinel != "" && strings.Contains(raw, sentinel) {
				t.Errorf("plaintext sentinel %q in IndexedDB record %q", sentinel, key)
			}
		}
		if atrest.Classify(key) == atrest.MustEncrypt && !atrest.IsSealed(raw) {
			t.Errorf("unsealed sensitive record %q", key)
		}
	}
	for _, name := range []string{"localStorage", "sessionStorage"} {
		store := js.Global().Get(name)
		if !store.Truthy() {
			continue
		}
		for i := 0; i < store.Get("length").Int(); i++ {
			key := store.Call("key", i).String()
			raw := store.Call("getItem", key).String()
			for _, sentinel := range sentinels {
				if sentinel != "" && strings.Contains(raw, sentinel) {
					t.Errorf("plaintext sentinel %q in %s %q", sentinel, name, key)
				}
			}
		}
	}
	// The key store contains structured-cloned CryptoKeys and wrapped metadata.
	// Inspect its JSON-safe dump as well as the application's string records.
	fake := js.Global().Get("__fakeIDB")
	if fake.Truthy() {
		dump := fake.Call("dump", keyDatabaseNameForTest()).String()
		for _, sentinel := range sentinels {
			if sentinel != "" && strings.Contains(dump, sentinel) {
				t.Errorf("plaintext sentinel %q in key database", sentinel)
			}
		}
	}
}
