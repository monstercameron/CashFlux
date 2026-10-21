// SPDX-License-Identifier: MIT

package assistantloop

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSessionCancelBeforeCallbackRegistration(t *testing.T) {
	s := NewSession("conversation-a")
	s.Cancel()
	if s.Active() {
		t.Fatal("canceled session is active")
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("Done remained open after cancellation")
	}
	var calls atomic.Int32
	s.SetCancel(func() { calls.Add(1) })
	if got := calls.Load(); got != 1 {
		t.Fatalf("late callback calls = %d, want 1", got)
	}
}

func TestSessionCancelIsIdempotentAndUsesCurrentCallback(t *testing.T) {
	s := NewSession("conversation-a")
	var oldCalls, currentCalls atomic.Int32
	s.SetCancel(func() { oldCalls.Add(1) })
	s.SetCancel(func() { currentCalls.Add(1) })
	s.Cancel()
	s.Cancel()
	if oldCalls.Load() != 0 || currentCalls.Load() != 1 {
		t.Fatalf("callback calls old/current = %d/%d, want 0/1", oldCalls.Load(), currentCalls.Load())
	}
}

func TestSessionLateCallbacksAndConversationIndependence(t *testing.T) {
	a, b := NewSession("conversation-a"), NewSession("conversation-b")
	var aCalls, bCalls atomic.Int32
	a.Cancel()
	a.SetCancel(func() { aCalls.Add(1) })
	b.SetCancel(func() { bCalls.Add(1) })
	b.Cancel()
	if a.ConversationID() != "conversation-a" || b.ConversationID() != "conversation-b" {
		t.Fatalf("conversation ids changed: %q / %q", a.ConversationID(), b.ConversationID())
	}
	if aCalls.Load() != 1 || bCalls.Load() != 1 {
		t.Fatalf("callback calls a/b = %d/%d, want 1/1", aCalls.Load(), bCalls.Load())
	}
}

func TestSessionConcurrentCancelInvokesCallbackOnce(t *testing.T) {
	s := NewSession("conversation-a")
	var calls atomic.Int32
	s.SetCancel(func() { calls.Add(1) })
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Cancel()
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("callback called %d times, want once", calls.Load())
	}
}

func TestApprovalRespondsExactlyOnce(t *testing.T) {
	a := NewApproval()
	if !a.TryRespond(true) {
		t.Fatal("first response was rejected")
	}
	if a.TryRespond(false) {
		t.Fatal("second response was accepted")
	}
	approved, responded := a.Wait(make(chan struct{}))
	if !responded || !approved {
		t.Fatalf("Wait = %t, %t; want approved response", approved, responded)
	}
}

func TestApprovalCancelWaitAndStaleResponse(t *testing.T) {
	a := NewApproval()
	done := make(chan struct{})
	close(done)
	approved, responded := a.Wait(done)
	if approved || responded {
		t.Fatalf("canceled Wait = %t, %t; want false, false", approved, responded)
	}
	if !a.TryRespond(true) || a.TryRespond(false) {
		t.Fatal("late stale response must be accepted once and reject repeats")
	}
	approved, responded = a.Wait(done)
	if approved || responded {
		t.Fatalf("stale approval after cancellation = %t, %t; want false, false", approved, responded)
	}
}

func TestApprovalWaitReturnsDecline(t *testing.T) {
	a := NewApproval()
	a.TryRespond(false)
	approved, responded := a.Wait(make(chan struct{}))
	if approved || !responded {
		t.Fatalf("declined Wait = %t, %t; want false, true", approved, responded)
	}
}
