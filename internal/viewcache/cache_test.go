// SPDX-License-Identifier: MIT

package viewcache

import (
	"sync"
	"testing"
)

func TestCacheRetainsRecentlyUsedView(t *testing.T) {
	cache := New[string, int](2)
	calls := map[string]int{}
	get := func(key string) int {
		return cache.GetOrCompute(key, func() int { calls[key]++; return calls[key] })
	}
	get("household")
	get("member")
	get("household") // household remains useful while exploring another view.
	get("category")
	if got := get("household"); got != 1 {
		t.Fatalf("recent household view recomputed: %d", got)
	}
	if got := get("member"); got != 2 {
		t.Fatalf("least-recent view did not expire: %d", got)
	}
	if cache.Len() != 2 {
		t.Fatalf("cache size = %d, want 2", cache.Len())
	}
}

func TestCacheKeysCoverDistinctInputs(t *testing.T) {
	type key struct {
		app, revision, scope, day int
	}
	var cache Cache[key, int]
	calls := 0
	for _, k := range []key{{1, 1, 0, 1}, {2, 1, 0, 1}, {1, 2, 0, 1}, {1, 1, 1, 1}, {1, 1, 0, 2}} {
		want := calls + 1
		for range 2 {
			if got := cache.GetOrCompute(k, func() int { calls++; return calls }); got != want {
				t.Fatalf("key %+v returned %d, want %d", k, got, want)
			}
		}
	}
	if calls != 5 {
		t.Fatalf("computed %d views, want 5", calls)
	}
}

func TestCacheZeroValuesClearAndBound(t *testing.T) {
	var cache Cache[int, []int]
	for i := range 100 {
		cache.GetOrCompute(i, func() []int { return nil })
		if cache.Len() > defaultCapacity {
			t.Fatal("zero-value cache exceeded default capacity")
		}
	}
	cache.GetOrCompute(99, func() []int { t.Fatal("nil result was not cached"); return nil })
	cache.Clear()
	if cache.Len() != 0 {
		t.Fatal("clear retained values")
	}
	cache.GetOrCompute(99, func() []int { return []int{99} })
}

func TestCacheConcurrentMissesReuseStoredResult(t *testing.T) {
	cache := New[string, int](2)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if got := cache.GetOrCompute("same", func() int { return 42 }); got != 42 {
				t.Errorf("concurrent result = %d", got)
			}
		})
	}
	wg.Wait()
	if cache.Len() != 1 {
		t.Fatalf("concurrent cache size = %d", cache.Len())
	}
}

func BenchmarkCacheWarmPage(b *testing.B) {
	cache := New[int, int](8)
	cache.GetOrCompute(1, func() int { return 42 })
	b.ReportAllocs()
	for b.Loop() {
		cache.GetOrCompute(1, func() int { b.Fatal("warm page recomputed"); return 0 })
	}
}
