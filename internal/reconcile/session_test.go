// SPDX-License-Identifier: MIT

package reconcile

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// fakeLedger is a minimal in-memory Ledger that records the calls made against
// it, so a rollback can be checked for what it did as well as what it left.
type fakeLedger struct {
	rows    map[string]domain.Transaction
	deleted []string
	failPut map[string]bool
	failDel map[string]bool
}

func newFakeLedger(rows ...domain.Transaction) *fakeLedger {
	l := &fakeLedger{rows: map[string]domain.Transaction{}, failPut: map[string]bool{}, failDel: map[string]bool{}}
	for _, r := range rows {
		l.rows[r.ID] = r
	}
	return l
}

func (l *fakeLedger) PutTransaction(t domain.Transaction) error {
	if l.failPut[t.ID] {
		return errors.New("put refused")
	}
	l.rows[t.ID] = t
	return nil
}

func (l *fakeLedger) DeleteTransaction(id string) error {
	if l.failDel[id] {
		return errors.New("delete refused")
	}
	delete(l.rows, id)
	l.deleted = append(l.deleted, id)
	return nil
}

func row(id string, cleared bool) domain.Transaction {
	t := domain.Transaction{
		ID: id, AccountID: "chk", Desc: id,
		Amount: money.New(-1000, "USD"),
		Date:   time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
	}
	if cleared {
		t.Cleared = true
		t.ClearedAt = time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	}
	return t
}

func TestSessionUntouchedIsANoOp(t *testing.T) {
	s := NewSession()
	if s.Touched() {
		t.Errorf("a fresh session reports changes")
	}
	if s.Count() != 0 {
		t.Errorf("Count = %d, want 0", s.Count())
	}
	l := newFakeLedger(row("a", false))
	if failed := s.Rollback(l); failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
	if len(l.deleted) != 0 || len(l.rows) != 1 {
		t.Errorf("an untouched rollback wrote to the ledger: %+v", l)
	}
}

func TestRollbackRestoresClearedState(t *testing.T) {
	original := row("a", false)
	l := newFakeLedger(original)
	s := NewSession()

	// The dialog ticks the row cleared.
	s.NoteBefore(original)
	ticked := original
	ticked.Cleared = true
	ticked.ClearedAt = time.Now()
	if err := l.PutTransaction(ticked); err != nil {
		t.Fatalf("put: %v", err)
	}
	if !l.rows["a"].Cleared {
		t.Fatalf("setup: the row was not ticked")
	}

	if failed := s.Rollback(l); failed != 0 {
		t.Fatalf("failed = %d, want 0", failed)
	}
	got := l.rows["a"]
	if got.Cleared {
		t.Errorf("the row is still cleared after Cancel")
	}
	if !got.ClearedAt.IsZero() {
		t.Errorf("ClearedAt = %v, want zero — the moment must go back too", got.ClearedAt)
	}
}

// A row ticked, unticked and ticked again must restore to how it STARTED, not to
// how it was one step ago. Recording only the first touch is what guarantees it.
func TestRollbackRestoresTheOriginalNotTheLastState(t *testing.T) {
	original := row("a", true) // starts cleared
	l := newFakeLedger(original)
	s := NewSession()

	cur := original
	for i := 0; i < 3; i++ {
		s.NoteBefore(cur)
		cur.Cleared = !cur.Cleared
		if err := l.PutTransaction(cur); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	if s.Count() != 1 {
		t.Errorf("Count = %d, want 1 — one row was touched, three times", s.Count())
	}
	if failed := s.Rollback(l); failed != 0 {
		t.Fatalf("failed = %d", failed)
	}
	if !l.rows["a"].Cleared {
		t.Errorf("the row came back unticked; it started ticked")
	}
}

// An adjustment the dialog posted did not exist before, so rolling back removes
// it rather than restoring some earlier version of it.
func TestRollbackDeletesRowsTheDialogCreated(t *testing.T) {
	l := newFakeLedger(row("a", false))
	s := NewSession()

	adj := row("adj", true)
	if err := l.PutTransaction(adj); err != nil {
		t.Fatalf("put: %v", err)
	}
	s.NoteCreated(adj.ID)

	if failed := s.Rollback(l); failed != 0 {
		t.Fatalf("failed = %d", failed)
	}
	if _, still := l.rows["adj"]; still {
		t.Errorf("the adjustment survived Cancel")
	}
	if _, gone := l.rows["a"]; !gone {
		t.Errorf("an untouched row was deleted")
	}
}

// Deletions run before restores, so the ledger is never briefly showing an
// adjustment against a balance that has already been put back.
func TestRollbackDeletesBeforeItRestores(t *testing.T) {
	original := row("a", false)
	l := newFakeLedger(original)
	s := NewSession()

	s.NoteBefore(original)
	ticked := original
	ticked.Cleared = true
	_ = l.PutTransaction(ticked)

	adj := row("adj", true)
	_ = l.PutTransaction(adj)
	s.NoteCreated(adj.ID)

	var order []string
	probe := &orderedLedger{inner: l, order: &order}
	if failed := s.Rollback(probe); failed != 0 {
		t.Fatalf("failed = %d", failed)
	}
	if len(order) == 0 || order[0] != "delete:adj" {
		t.Errorf("call order = %v, want the delete first", order)
	}
}

type orderedLedger struct {
	inner *fakeLedger
	order *[]string
}

func (o *orderedLedger) PutTransaction(t domain.Transaction) error {
	*o.order = append(*o.order, "put:"+t.ID)
	return o.inner.PutTransaction(t)
}

func (o *orderedLedger) DeleteTransaction(id string) error {
	*o.order = append(*o.order, "delete:"+id)
	return o.inner.DeleteTransaction(id)
}

// A rollback that stops at the first error leaves the remainder changed and says
// nothing. It must keep going and report how much it could not put back.
func TestRollbackContinuesPastAFailureAndCountsIt(t *testing.T) {
	a, b := row("a", false), row("b", false)
	l := newFakeLedger(a, b)
	l.failPut["a"] = true
	s := NewSession()

	for _, r := range []domain.Transaction{a, b} {
		s.NoteBefore(r)
		tick := r
		tick.Cleared = true
		l.rows[r.ID] = tick
	}
	failed := s.Rollback(l)
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
	if l.rows["b"].Cleared {
		t.Errorf("row b was left ticked — the rollback stopped at the first failure")
	}
}

// Finishing a reconciliation keeps its writes; the journal must not then be able
// to undo them.
func TestForgetDropsTheJournalWithoutRestoring(t *testing.T) {
	original := row("a", false)
	l := newFakeLedger(original)
	s := NewSession()

	s.NoteBefore(original)
	ticked := original
	ticked.Cleared = true
	_ = l.PutTransaction(ticked)

	s.Forget()
	if s.Touched() {
		t.Errorf("Forget left the session reporting changes")
	}
	if failed := s.Rollback(l); failed != 0 {
		t.Fatalf("failed = %d", failed)
	}
	if !l.rows["a"].Cleared {
		t.Errorf("a rollback after Forget still undid the write")
	}
}

// A second Cancel must not re-apply the first one's restore over newer work.
func TestRollbackIsNotRepeatable(t *testing.T) {
	original := row("a", false)
	l := newFakeLedger(original)
	s := NewSession()
	s.NoteBefore(original)
	ticked := original
	ticked.Cleared = true
	_ = l.PutTransaction(ticked)

	_ = s.Rollback(l)
	if s.Touched() {
		t.Errorf("the session still reports changes after a rollback")
	}
	// Someone else ticks the row afterwards; a stale rollback must not undo it.
	later := original
	later.Cleared = true
	_ = l.PutTransaction(later)
	if failed := s.Rollback(l); failed != 0 {
		t.Fatalf("failed = %d", failed)
	}
	if !l.rows["a"].Cleared {
		t.Errorf("a spent session rolled back work it never made")
	}
}

func TestNilSessionIsSafe(t *testing.T) {
	var s *Session
	s.NoteBefore(row("a", false))
	s.NoteCreated("x")
	s.Forget()
	if s.Touched() || s.Count() != 0 {
		t.Errorf("a nil session reported state")
	}
	if failed := s.Rollback(newFakeLedger()); failed != 0 {
		t.Errorf("failed = %d", failed)
	}
}
