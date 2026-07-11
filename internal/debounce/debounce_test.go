// SPDX-License-Identifier: MIT

package debounce

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestCoalescesToOneTrailingCall(t *testing.T) {
	var n int32
	for i := 0; i < 5; i++ {
		Call("burst", 30*time.Millisecond, func() { atomic.AddInt32(&n, 1) })
	}
	if got := atomic.LoadInt32(&n); got != 0 {
		t.Fatalf("fired before the delay elapsed: %d", got)
	}
	time.Sleep(70 * time.Millisecond)
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Fatalf("want exactly one trailing call, got %d", got)
	}
}

func TestFlushCancelsPending(t *testing.T) {
	var fired int32
	Call("flush", 50*time.Millisecond, func() { atomic.AddInt32(&fired, 1) })
	if !Pending("flush") {
		t.Fatal("expected a pending call")
	}
	if !Flush("flush") {
		t.Fatal("Flush should report it cancelled a pending call")
	}
	time.Sleep(80 * time.Millisecond)
	if atomic.LoadInt32(&fired) != 0 {
		t.Fatalf("flushed call must not fire, got %d", fired)
	}
	if Flush("flush") {
		t.Fatal("second Flush should be a no-op")
	}
}

func TestZeroDelayRunsSynchronously(t *testing.T) {
	var n int32
	Call("zero", 0, func() { atomic.AddInt32(&n, 1) })
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Fatalf("delay<=0 should run fn now, got %d", got)
	}
	if Pending("zero") {
		t.Fatal("zero-delay call should leave nothing pending")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	var a, b int32
	Call("a", 20*time.Millisecond, func() { atomic.AddInt32(&a, 1) })
	Call("b", 20*time.Millisecond, func() { atomic.AddInt32(&b, 1) })
	time.Sleep(60 * time.Millisecond)
	if atomic.LoadInt32(&a) != 1 || atomic.LoadInt32(&b) != 1 {
		t.Fatalf("independent keys should each fire once: a=%d b=%d", a, b)
	}
}
