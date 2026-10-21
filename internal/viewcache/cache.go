// SPDX-License-Identifier: MIT

// Package viewcache shares expensive derived values across page components while
// bounding their retained memory. Callers own invalidation: keys must describe
// the data instance, revision, scope, time window, and other computation inputs.
package viewcache

import (
	"container/list"
	"sync"
)

const defaultCapacity = 8

// Cache is a bounded least-recently-used cache. Its zero value is ready to use
// with capacity eight. Values are shared, so callers must treat them as immutable.
// A Cache must not be copied after first use.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	values   map[K]*list.Element
	order    list.List
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

// New creates a cache with the requested capacity. Nonpositive capacity selects
// the same default as the zero value.
func New[K comparable, V any](capacity int) *Cache[K, V] {
	return &Cache[K, V]{capacity: capacity}
}

// GetOrCompute reuses a value when key is present, otherwise computes and stores
// it. Computation runs outside the lock so it may call other caches. Concurrent
// misses may compute more than once; the first stored result wins. Panics are
// propagated and never cached.
func (c *Cache[K, V]) GetOrCompute(key K, compute func() V) V {
	c.mu.Lock()
	if value, ok := c.lookup(key); ok {
		c.mu.Unlock()
		return value
	}
	c.mu.Unlock()

	value := compute()
	c.mu.Lock()
	defer c.mu.Unlock()
	if stored, ok := c.lookup(key); ok {
		return stored
	}
	if c.capacity <= 0 {
		c.capacity = defaultCapacity
	}
	if c.values == nil {
		c.values = make(map[K]*list.Element, c.capacity)
	}
	if len(c.values) == c.capacity {
		oldest := c.order.Back()
		delete(c.values, oldest.Value.(entry[K, V]).key)
		c.order.Remove(oldest)
	}
	c.values[key] = c.order.PushFront(entry[K, V]{key: key, value: value})
	return value
}

func (c *Cache[K, V]) lookup(key K) (V, bool) {
	if element, ok := c.values[key]; ok {
		c.order.MoveToFront(element)
		return element.Value.(entry[K, V]).value, true
	}
	var zero V
	return zero, false
}

// Get returns a cached value and marks it recently used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lookup(key)
}

// Set inserts or replaces a value and applies the configured LRU bound.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.lookup(key); ok {
		el := c.values[key]
		el.Value = entry[K, V]{key: key, value: value}
		return
	}
	if c.capacity <= 0 {
		c.capacity = defaultCapacity
	}
	if c.values == nil {
		c.values = make(map[K]*list.Element, c.capacity)
	}
	if len(c.values) == c.capacity {
		oldest := c.order.Back()
		delete(c.values, oldest.Value.(entry[K, V]).key)
		c.order.Remove(oldest)
	}
	c.values[key] = c.order.PushFront(entry[K, V]{key: key, value: value})
}

// Len reports the number of retained values.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.values)
}

// Clear releases all retained values without changing the configured capacity.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.values)
	c.order.Init()
}
