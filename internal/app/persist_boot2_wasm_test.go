//go:build js && wasm

package app

import (
	"os"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/browserstore"
)

// boot2FixtureRanges installs only the missing browser API on the isolated Node
// shim. It does not change real browser data or production key/record services.
func boot2FixtureRanges(t *testing.T) {
	t.Helper()
	old := js.Global().Get("IDBKeyRange")
	script, err := os.ReadFile("../browserstore/secure_ranges_test.js")
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

func TestBOOT2FixtureRefreshesDurableGeneration(t *testing.T) {
	solp22Storage(t)
	previous := browserstore.GetString(datasetGenKey)
	t.Cleanup(func() { solp22Batch(t, map[string]string{datasetGenKey: previous}, nil) })
	solp22Batch(t, map[string]string{datasetGenKey: "boot2-durable-generation"}, nil)
	browserstore.SetMemory(datasetGenKey, "obsolete-test-generation")
	solp22Storage(t)
	if got := readDatasetGen(); got != "boot2-durable-generation" {
		t.Fatalf("fixture reused stale cached generation: %q", got)
	}
}
