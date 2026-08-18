// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import "testing"

// These cover the mechanism, not the browser. A headless wasm test has no
// IndexedDB and no second tab, so what is checked here is that the cache
// converges when another tab's write ARRIVES, and that every path degrades to
// today's behaviour when the browser offers nothing to deliver it with — which
// is the property that decides whether this is safe to ship.

func TestApplyingAnotherTabsWriteUpdatesTheCache(t *testing.T) {
	const key = "cashflux:test-crosstab"
	t.Cleanup(func() { Remove(key) })

	Set(key, "mine")
	if got := GetString(key); got != "mine" {
		t.Fatalf("GetString = %q", got)
	}

	// What the BroadcastChannel handler does when another tab reports a write.
	mu.Lock()
	cache[key] = "theirs"
	mu.Unlock()
	if got := GetString(key); got != "theirs" {
		t.Fatalf("GetString = %q, want the other tab's value", got)
	}

	// And a removal elsewhere must actually remove it here. A credential that
	// has been signed out in another tab must not survive in this one's cache,
	// which is how a dead session goes on being presented as live.
	mu.Lock()
	delete(cache, key)
	mu.Unlock()
	if _, ok := Get(key); ok {
		t.Fatal("a key removed in another tab is still cached here")
	}
}

func TestWatchersFireAndAreIsolatedFromEachOther(t *testing.T) {
	const key = "cashflux:test-watch"
	t.Cleanup(func() {
		watchMu.Lock()
		delete(watchers, key)
		watchMu.Unlock()
	})

	var second bool
	// A watcher that panics must not stop the ones after it: these run from a JS
	// event callback, where an escaping panic takes the page down.
	Watch(key, func(string, bool) { panic("watcher blew up") })
	Watch(key, func(v string, present bool) {
		if v != "value" || !present {
			t.Errorf("watcher got (%q, %v)", v, present)
		}
		second = true
	})

	notifyWatchers(key, "value", true)
	if !second {
		t.Fatal("a panicking watcher prevented the next one from running")
	}
}

func TestCrossTabDegradesWithoutBrowserSupport(t *testing.T) {
	// No BroadcastChannel and no IndexedDB in a headless wasm test, which is
	// exactly the unsupported-browser case: every entry point must be a no-op
	// rather than a panic, leaving the old per-tab behaviour intact.
	StartCrossTab()
	publish("cashflux:test-degrade", "v", true)
	publish("cashflux:test-degrade", "", false)
	Reload("cashflux:test-degrade")
	Reload()

	const key = "cashflux:test-degrade-set"
	t.Cleanup(func() { Remove(key) })
	Set(key, "still works")
	if got := GetString(key); got != "still works" {
		t.Fatalf("GetString = %q — a write must succeed with no cross-tab support", got)
	}
}
