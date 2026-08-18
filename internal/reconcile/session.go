// SPDX-License-Identifier: MIT

package reconcile

import "github.com/monstercameron/CashFlux/internal/domain"

// Ledger is the narrow slice of the store a rollback needs. An interface, so the
// journal stays pure logic testable without the app, and so a rollback holds only
// the two powers it actually uses.
type Ledger interface {
	PutTransaction(domain.Transaction) error
	DeleteTransaction(id string) error
}

// Session remembers everything a reconcile dialog has written, so cancelling it
// puts the ledger back exactly as it was found (C683).
//
// # Why a journal rather than staged edits
//
// Ticking a row cleared writes it immediately, and that is deliberate: the
// difference readout, the "mark all cleared" preview and the finish-path gate all
// recompute from live data, so a staged set would mean maintaining a second,
// shadow copy of the ledger for the dialog to read — and every figure shown would
// then derive from something the rest of the app cannot see.
//
// What was wrong was not that the writes happen, but that Cancel did not undo
// them. A dialog with a Cancel button makes a promise about what pressing it does;
// ticking twenty rows, realising the wrong statement was open, and pressing Cancel
// used to leave all twenty ticked, with no record of which they were.
type Session struct {
	// before holds each touched row exactly as it was when the modal first
	// touched it. Keyed by id, and written only on the FIRST touch — a row
	// ticked, unticked and ticked again must restore to its original state, not
	// to its state one step ago.
	before map[string]domain.Transaction
	// created holds rows the modal itself posted (reconciliation adjustments).
	// They did not exist before, so rolling back means deleting them.
	created []string
}

func NewSession() *Session {
	return &Session{before: map[string]domain.Transaction{}}
}

// NoteBefore records a row's pre-modal state the first time it is touched.
func (s *Session) NoteBefore(t domain.Transaction) {
	if s == nil || t.ID == "" {
		return
	}
	if _, seen := s.before[t.ID]; !seen {
		s.before[t.ID] = t
	}
}

// NoteCreated records a row the modal posted itself.
func (s *Session) NoteCreated(id string) {
	if s == nil || id == "" {
		return
	}
	s.created = append(s.created, id)
}

// Touched reports whether the modal has written anything, so Cancel can close
// silently when there is nothing to take back.
func (s *Session) Touched() bool {
	return s != nil && (len(s.before) > 0 || len(s.created) > 0)
}

// Count is how many rows Cancel would put back — used to say so before doing it.
func (s *Session) Count() int {
	if s == nil {
		return 0
	}
	return len(s.before) + len(s.created)
}

// Rollback restores every row the modal changed and deletes every row it created.
//
// Deletions run FIRST: a created adjustment can only have been posted after the
// clears that motivated it, and removing it before restoring the rest keeps the
// intermediate ledger from ever showing an adjustment against a balance that has
// already been put back.
//
// It reports the number of rows it could not restore rather than failing outright.
// A partial rollback is bad, but a rollback that stops at the first error and
// leaves the remainder changed is worse, and silence about it is worse still.
func (s *Session) Rollback(l Ledger) (failed int) {
	if s == nil || l == nil {
		return 0
	}

	// What could not be put back STAYS in the journal, so the caller can retry and
	// the before-image is not lost with the failure. Clearing it wholesale would
	// leave a row changed, no record of what it was, and only a count to say so.
	keptBefore := map[string]domain.Transaction{}
	var keptCreated []string
	for _, id := range s.created {
		if err := l.DeleteTransaction(id); err != nil {
			failed++
			keptCreated = append(keptCreated, id)
		}
	}
	for id, t := range s.before {
		if err := l.PutTransaction(t); err != nil {
			failed++
			keptBefore[id] = t
		}
	}
	s.before = keptBefore
	s.created = keptCreated
	return failed
}

// Forget drops the journal without restoring anything — what a COMPLETED
// reconciliation does, since its writes are the point rather than a draft.
func (s *Session) Forget() {
	if s == nil {
		return
	}
	s.before = map[string]domain.Transaction{}
	s.created = nil
}
