// SPDX-License-Identifier: MIT

// Package assistantloop contains concurrency-safe lifecycle primitives for one
// interactive assistant request session.
package assistantloop

import "sync"

// Session owns cancellation for one conversation's active assistant loop.
// ConversationID is fixed at construction so later UI selection changes cannot
// retarget callbacks or accounting for this session.
type Session struct {
	conversationID string
	done           chan struct{}
	mu             sync.Mutex
	cancel         func()
	active         bool
}

// NewSession creates an active session for conversationID.
func NewSession(conversationID string) *Session {
	return &Session{
		conversationID: conversationID,
		done:           make(chan struct{}),
		active:         true,
	}
}

// ConversationID returns the immutable conversation id captured at creation.
func (s *Session) ConversationID() string { return s.conversationID }

// Done returns the channel closed when the session is canceled.
func (s *Session) Done() <-chan struct{} { return s.done }

// Active reports whether the session has not been canceled.
func (s *Session) Active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// SetCancel installs the current provider-request cancellation callback. While
// active, a new callback replaces the previous request's callback. If the
// session was already canceled, a non-nil callback is invoked immediately.
func (s *Session) SetCancel(cancel func()) {
	s.mu.Lock()
	if s.active {
		s.cancel = cancel
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Cancel marks the session inactive, closes Done, and invokes the currently
// installed provider callback once. Repeated calls have no effect.
func (s *Session) Cancel() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	close(s.done)
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Approval is a one-shot response channel for an assistant action awaiting the
// user's decision.
type Approval struct {
	mu        sync.Mutex
	responded bool
	response  chan bool
}

// NewApproval creates an unresolved approval decision.
func NewApproval() *Approval { return &Approval{response: make(chan bool, 1)} }

// TryRespond records the first decision and returns true only for that decision.
// The buffered channel keeps the response nonblocking if cancellation wins first.
func (a *Approval) TryRespond(approved bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.responded {
		return false
	}
	a.responded = true
	a.response <- approved
	return true
}

// Wait returns the decision and true when a response wins, or false when done
// is closed first. A canceled wait never reports approval if both become ready
// together.
func (a *Approval) Wait(done <-chan struct{}) (approved bool, responded bool) {
	select {
	case <-done:
		return false, false
	default:
	}
	select {
	case <-done:
		return false, false
	case approved = <-a.response:
		select {
		case <-done:
			return false, false
		default:
			return approved, true
		}
	}
}
